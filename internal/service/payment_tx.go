package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Qwerkyas/ssubench/internal/domain"
	"github.com/Qwerkyas/ssubench/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)
type txTaskService struct {
	*taskService
	db *pgxpool.Pool
}

func NewTxTaskService(
	db *pgxpool.Pool,
	taskRepo repo.TaskRepo,
	userRepo repo.UserRepo,
	bidRepo repo.BidRepo,
	paymentRepo repo.PaymentRepo,
) TaskService {
	base := &taskService{
		taskRepo:    taskRepo,
		userRepo:    userRepo,
		bidRepo:     bidRepo,
		paymentRepo: paymentRepo,
	}
	return &txTaskService{taskService: base, db: db}
}

func (s *txTaskService) Confirm(ctx context.Context, customerID, taskID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("txTaskService.Confirm begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		taskCustomerID uuid.UUID
		taskExecutorID *uuid.UUID
		taskReward     int64
		taskStatus     domain.TaskStatus
	)
	err = tx.QueryRow(ctx, `
		SELECT customer_id, executor_id, reward, status
		FROM tasks
		WHERE id = $1
		FOR UPDATE
	`, taskID).Scan(&taskCustomerID, &taskExecutorID, &taskReward, &taskStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTaskNotFound
		}
		return fmt.Errorf("txTaskService.Confirm lock task: %w", err)
	}

	if taskCustomerID != customerID {
		return domain.ErrForbidden
	}

	if taskStatus != domain.TaskStatusCompleted {
		return domain.ErrTaskInvalidStatus
	}

	if taskExecutorID == nil {
		return domain.ErrForbidden
	}

	var paymentExists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE task_id = $1)`, taskID).Scan(&paymentExists)
	if err != nil {
		return fmt.Errorf("txTaskService.Confirm check payment exists: %w", err)
	}
	if paymentExists {
		return domain.ErrTaskAlreadyPaid
	}

	debitResult, err := tx.Exec(ctx,
		`UPDATE users SET balance = balance - $1, updated_at = NOW() WHERE id = $2 AND balance >= $1`,
		taskReward, customerID,
	)
	if err != nil {
		return fmt.Errorf("txTaskService.Confirm debit: %w", err)
	}
	if debitResult.RowsAffected() != 1 {
		return domain.ErrInsufficientBalance
	}

	creditResult, err := tx.Exec(ctx,
		`UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2`,
		taskReward, *taskExecutorID,
	)
	if err != nil {
		return fmt.Errorf("txTaskService.Confirm credit: %w", err)
	}
	if creditResult.RowsAffected() != 1 {
		return domain.ErrForbidden
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO payments (id, task_id, from_user_id, to_user_id, amount, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), taskID, customerID, *taskExecutorID, taskReward, time.Now(),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrTaskAlreadyPaid
		}
		return fmt.Errorf("txTaskService.Confirm insert payment: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("txTaskService.Confirm commit: %w", err)
	}

	return nil
}
