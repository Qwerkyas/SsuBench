package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Qwerkyas/ssubench/internal/domain"
	"github.com/Qwerkyas/ssubench/internal/middleware"
	"github.com/Qwerkyas/ssubench/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type userResponse struct {
	ID        uuid.UUID   `json:"id"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	Balance   int64       `json:"balance"`
	IsBlocked bool        `json:"is_blocked"`
	CreatedAt time.Time   `json:"created_at"`
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	userIDStr := middleware.GetUserID(r.Context())
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.user.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"role":       user.Role,
		"balance":    user.Balance,
		"is_blocked": user.IsBlocked,
		"created_at": user.CreatedAt,
	})
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := getPagination(r)

	users, err := h.user.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, userResponse{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			Balance:   user.Balance,
			IsBlocked: user.IsBlocked,
			CreatedAt: user.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) blockUser(w http.ResponseWriter, r *http.Request) {
	adminIDStr := middleware.GetUserID(r.Context())
	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err = h.user.Block(r.Context(), adminID, userID); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user blocked"})
}

func (h *Handler) unblockUser(w http.ResponseWriter, r *http.Request) {
	adminIDStr := middleware.GetUserID(r.Context())
	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err = h.user.Unblock(r.Context(), adminID, userID); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user unblocked"})
}

func (h *Handler) topUpBalance(w http.ResponseWriter, r *http.Request) {
	adminIDStr := middleware.GetUserID(r.Context())
	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var input service.TopUpBalanceInput
	if err = json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err = h.validate.Struct(input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err = h.user.TopUpBalance(r.Context(), adminID, userID, input.Amount); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "balance topped up"})
}
