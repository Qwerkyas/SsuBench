package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Qwerkyas/ssubench/internal/domain"
	"github.com/Qwerkyas/ssubench/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type stubAuthService struct{}

func (s *stubAuthService) Register(ctx context.Context, input service.RegisterInput) (*domain.User, error) {
	return nil, nil
}
func (s *stubAuthService) Login(ctx context.Context, input service.LoginInput) (string, error) {
	return "", nil
}

type stubUserService struct{}

func (s *stubUserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return &domain.User{ID: id, Email: "u@example.com", Role: domain.RoleCustomer}, nil
}
func (s *stubUserService) List(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	return []*domain.User{}, nil
}
func (s *stubUserService) Block(ctx context.Context, adminID, userID uuid.UUID) error   { return nil }
func (s *stubUserService) Unblock(ctx context.Context, adminID, userID uuid.UUID) error { return nil }
func (s *stubUserService) TopUpBalance(ctx context.Context, adminID, userID uuid.UUID, amount int64) error {
	return nil
}

type stubTaskService struct {
	confirmErr error
}

func (s *stubTaskService) Create(ctx context.Context, input service.CreateTaskInput) (*domain.Task, error) {
	return &domain.Task{ID: uuid.New(), CustomerID: input.CustomerID, Title: input.Title, Description: input.Description, Reward: input.Reward, Status: domain.TaskStatusDraft}, nil
}
func (s *stubTaskService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	return &domain.Task{ID: id, CustomerID: uuid.New(), Status: domain.TaskStatusDraft}, nil
}
func (s *stubTaskService) List(ctx context.Context, limit, offset int) ([]*domain.Task, error) {
	return []*domain.Task{}, nil
}
func (s *stubTaskService) Publish(ctx context.Context, customerID, taskID uuid.UUID) error {
	return nil
}
func (s *stubTaskService) Cancel(ctx context.Context, customerID, taskID uuid.UUID) error { return nil }
func (s *stubTaskService) MarkCompleted(ctx context.Context, executorID, taskID uuid.UUID) error {
	return nil
}
func (s *stubTaskService) Confirm(ctx context.Context, customerID, taskID uuid.UUID) error {
	return s.confirmErr
}

type stubBidService struct{}

func (s *stubBidService) Create(ctx context.Context, input service.CreateBidInput) (*domain.Bid, error) {
	return &domain.Bid{ID: uuid.New(), TaskID: input.TaskID, ExecutorID: input.ExecutorID, Status: domain.BidStatusPending}, nil
}
func (s *stubBidService) ListByTask(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*domain.Bid, error) {
	return []*domain.Bid{}, nil
}
func (s *stubBidService) Accept(ctx context.Context, customerID, taskID, bidID uuid.UUID) error {
	return nil
}

type stubPaymentService struct{}

func (s *stubPaymentService) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Payment, error) {
	return []*domain.Payment{}, nil
}

func newTestRouter(t *testing.T, userSvc service.UserService, taskSvc service.TaskService) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if userSvc == nil {
		userSvc = &stubUserService{}
	}
	h := New(
		&stubAuthService{},
		userSvc,
		taskSvc,
		&stubBidService{},
		&stubPaymentService{},
		log,
		"test-secret",
	)
	return h.InitRoutes()
}

func makeJWT(t *testing.T, role domain.Role) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": uuid.New().String(),
		"role":    string(role),
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func assertJSONError(t *testing.T, rr *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if body["error"] != expected {
		t.Fatalf("expected error %q, got %v", expected, body["error"])
	}
}

func TestRBAC_UsersEndpoint_AdminOnly(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	reqForbidden := httptest.NewRequest(http.MethodGet, "/users", nil)
	reqForbidden.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleCustomer))
	rrForbidden := httptest.NewRecorder()
	router.ServeHTTP(rrForbidden, reqForbidden)
	if rrForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rrForbidden.Code)
	}
	assertJSONError(t, rrForbidden, "forbidden")

	reqAllowed := httptest.NewRequest(http.MethodGet, "/users", nil)
	reqAllowed.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleAdmin))
	rrAllowed := httptest.NewRecorder()
	router.ServeHTTP(rrAllowed, reqAllowed)
	if rrAllowed.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrAllowed.Code)
	}
}

func TestRBAC_CreateTask_CustomerOnly(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	body := []byte(`{"title":"Task","description":"Long description for task","reward":100}`)
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleExecutor))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	assertJSONError(t, rr, "forbidden")
}

func TestRBAC_CreateTask_AllowsAdminBypass(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	body := []byte(`{"title":"Task","description":"Long description for task","reward":100}`)
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleAdmin))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestRBAC_CompleteTask_ExecutorOnly(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	req := httptest.NewRequest(http.MethodPatch, "/tasks/"+uuid.New().String()+"/complete", nil)
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleCustomer))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	assertJSONError(t, rr, "forbidden")
}

func TestProtectedRoute_WithoutToken_ReturnsUnauthorizedJSON(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	assertJSONError(t, rr, "unauthorized")
}

func TestConfirmTask_AlreadyPaid_ReturnsConflict(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{confirmErr: domain.ErrTaskAlreadyPaid})

	req := httptest.NewRequest(http.MethodPatch, "/tasks/"+uuid.New().String()+"/confirm", nil)
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleCustomer))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
	assertJSONError(t, rr, domain.ErrTaskAlreadyPaid.Error())
}

func TestConfirmTask_InsufficientBalance_ReturnsPaymentRequired(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{confirmErr: domain.ErrInsufficientBalance})

	req := httptest.NewRequest(http.MethodPatch, "/tasks/"+uuid.New().String()+"/confirm", nil)
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleCustomer))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rr.Code)
	}
	assertJSONError(t, rr, domain.ErrInsufficientBalance.Error())
}

func TestConfirmTask_NotOwner_ReturnsForbidden(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{confirmErr: domain.ErrForbidden})

	req := httptest.NewRequest(http.MethodPatch, "/tasks/"+uuid.New().String()+"/confirm", nil)
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleCustomer))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	assertJSONError(t, rr, domain.ErrForbidden.Error())
}

type stubUserListService struct {
	stubUserService
	users []*domain.User
}

func (s *stubUserListService) List(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	return s.users, nil
}

func TestUsersList_DoesNotExposePasswordHash(t *testing.T) {
	userSvc := &stubUserListService{
		users: []*domain.User{
			{
				ID:           uuid.New(),
				Email:        "admin@example.com",
				Role:         domain.RoleAdmin,
				Balance:      100,
				IsBlocked:    false,
				PasswordHash: "$2a$12$secret",
				CreatedAt:    time.Now(),
			},
		},
	}
	router := newTestRouter(t, userSvc, &stubTaskService{})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleAdmin))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}

	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("expected one user in response, got %v", body["data"])
	}

	user, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("expected user object, got %T", data[0])
	}
	if _, exists := user["password_hash"]; exists {
		t.Fatalf("password_hash must not be exposed in /users response")
	}
}

func TestRBAC_TopUpBalance_AdminOnly(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	req := httptest.NewRequest(http.MethodPatch, "/users/"+uuid.New().String()+"/balance/top-up", bytes.NewReader([]byte(`{"amount":500}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleCustomer))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	assertJSONError(t, rr, "forbidden")
}

func TestTopUpBalance_AdminSuccess(t *testing.T) {
	router := newTestRouter(t, nil, &stubTaskService{})

	req := httptest.NewRequest(http.MethodPatch, "/users/"+uuid.New().String()+"/balance/top-up", bytes.NewReader([]byte(`{"amount":500}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeJWT(t, domain.RoleAdmin))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
