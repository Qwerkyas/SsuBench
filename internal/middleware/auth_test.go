package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qwerkyas/ssubench/internal/domain"
)

func TestAuth_MissingHeader_JSONError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rr := httptest.NewRecorder()

	h := Auth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("expected unauthorized error, got %q", body["error"])
	}
}

func TestRequireRole_Forbidden_JSONError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, string(domain.RoleExecutor)))
	rr := httptest.NewRecorder()

	h := RequireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}
	if body["error"] != "forbidden" {
		t.Fatalf("expected forbidden error, got %q", body["error"])
	}
}

func TestRequireRole_AllowsExpectedRole(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, string(domain.RoleAdmin)))
	rr := httptest.NewRecorder()

	h := RequireRole(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRequireRole_AdminBypass(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserRoleKey, string(domain.RoleAdmin)))
	rr := httptest.NewRecorder()

	h := RequireRole(domain.RoleCustomer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
