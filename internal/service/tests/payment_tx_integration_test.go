package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Qwerkyas/ssubench/internal/domain"
	"github.com/Qwerkyas/ssubench/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func requireIntegrationDB() bool {
	return os.Getenv("REQUIRE_INTEGRATION_DB") == "1" || os.Getenv("CI") != ""
}

func failOrSkipIntegration(t *testing.T, format string, args ...any) {
	t.Helper()
	if requireIntegrationDB() {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

func testDBDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_DSN"); dsn != "" {
		return dsn
	}
	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	pass := envOrDefault("DB_PASSWORD", "postgres")
	name := envOrDefault("DB_NAME", "ssubench")
	ssl := envOrDefault("DB_SSL_MODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func openIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, testDBDSN())
	if err != nil {
		failOrSkipIntegration(t, "integration test requires a reachable Postgres instance: %v", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		failOrSkipIntegration(t, "integration test requires a reachable Postgres instance: %v", err)
	}

	var usersExists bool
	err = pool.QueryRow(ctx, `SELECT to_regclass('public.users') IS NOT NULL`).Scan(&usersExists)
	if err != nil || !usersExists {
		pool.Close()
		failOrSkipIntegration(t, "integration test requires applied migrations before running go test ./...")
	}

	return pool
}

func seedUsersAndTask(t *testing.T, ctx context.Context, pool *pgxpool.Pool, customerBalance, reward int64) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	customerID := uuid.New()
	executorID := uuid.New()
	taskID := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, balance, is_blocked, created_at, updated_at)
		VALUES ($1, $2, $3, 'customer', $4, false, NOW(), NOW())
	`, customerID, "itest_customer_"+customerID.String()+"@example.com", "hash", customerBalance)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, balance, is_blocked, created_at, updated_at)
		VALUES ($1, $2, $3, 'executor', $4, false, NOW(), NOW())
	`, executorID, "itest_executor_"+executorID.String()+"@example.com", "hash", int64(0))
	if err != nil {
		t.Fatalf("insert executor: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO tasks (id, customer_id, executor_id, title, description, reward, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Integration Task', 'Integration test task description', $4, 'completed', NOW(), NOW())
	`, taskID, customerID, executorID, reward)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM payments WHERE task_id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM bids WHERE task_id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1 OR id = $2`, customerID, executorID)
	})

	return customerID, executorID, taskID
}

func userBalance(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) int64 {
	t.Helper()
	var balance int64
	if err := pool.QueryRow(ctx, `SELECT balance FROM users WHERE id = $1`, id).Scan(&balance); err != nil {
		t.Fatalf("select user balance: %v", err)
	}
	return balance
}

func paymentCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, taskID uuid.UUID) int {
	t.Helper()
	var cnt int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE task_id = $1`, taskID).Scan(&cnt); err != nil {
		t.Fatalf("select payment count: %v", err)
	}
	return cnt
}

func TestTxConfirm_TransfersFundsAndBlocksDoublePayment(t *testing.T) {
	pool := openIntegrationDB(t)
	defer pool.Close()

	ctx := context.Background()
	customerID, executorID, taskID := seedUsersAndTask(t, ctx, pool, 1000, 250)
	svc := service.NewTxTaskService(pool, nil, nil, nil, nil)

	err := svc.Confirm(ctx, customerID, taskID)
	if err != nil {
		t.Fatalf("first confirm failed: %v", err)
	}

	if got := userBalance(t, ctx, pool, customerID); got != 750 {
		t.Fatalf("customer balance expected 750, got %d", got)
	}
	if got := userBalance(t, ctx, pool, executorID); got != 250 {
		t.Fatalf("executor balance expected 250, got %d", got)
	}
	if got := paymentCount(t, ctx, pool, taskID); got != 1 {
		t.Fatalf("expected one payment row, got %d", got)
	}

	err = svc.Confirm(ctx, customerID, taskID)
	if !errors.Is(err, domain.ErrTaskAlreadyPaid) {
		t.Fatalf("expected ErrTaskAlreadyPaid, got %v", err)
	}

	if got := userBalance(t, ctx, pool, customerID); got != 750 {
		t.Fatalf("customer balance changed after second confirm: %d", got)
	}
	if got := userBalance(t, ctx, pool, executorID); got != 250 {
		t.Fatalf("executor balance changed after second confirm: %d", got)
	}
}

func TestTxConfirm_InsufficientBalance_IsAtomic(t *testing.T) {
	pool := openIntegrationDB(t)
	defer pool.Close()

	ctx := context.Background()
	customerID, executorID, taskID := seedUsersAndTask(t, ctx, pool, 100, 250)
	svc := service.NewTxTaskService(pool, nil, nil, nil, nil)

	err := svc.Confirm(ctx, customerID, taskID)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}

	if got := userBalance(t, ctx, pool, customerID); got != 100 {
		t.Fatalf("customer balance expected 100, got %d", got)
	}
	if got := userBalance(t, ctx, pool, executorID); got != 0 {
		t.Fatalf("executor balance expected 0, got %d", got)
	}
	if got := paymentCount(t, ctx, pool, taskID); got != 0 {
		t.Fatalf("expected no payment rows, got %d", got)
	}
}
