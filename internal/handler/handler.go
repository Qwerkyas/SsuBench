package handler

import (
	"net/http"
	"strconv"

	"log/slog"

	"github.com/Qwerkyas/ssubench/internal/domain"
	"github.com/Qwerkyas/ssubench/internal/middleware"
	"github.com/Qwerkyas/ssubench/internal/service"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	auth     service.AuthService
	user     service.UserService
	task     service.TaskService
	bid      service.BidService
	payment  service.PaymentService
	validate *validator.Validate
	log      *slog.Logger
	secret   string
}

func New(
	auth service.AuthService,
	user service.UserService,
	task service.TaskService,
	bid service.BidService,
	payment service.PaymentService,
	log *slog.Logger,
	secret string,
) *Handler {
	return &Handler{
		auth:     auth,
		user:     user,
		task:     task,
		bid:      bid,
		payment:  payment,
		validate: validator.New(),
		log:      log,
		secret:   secret,
	}
}

func (h *Handler) InitRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(h.log))
	r.Use(middleware.Recover(h.log))
	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(h.secret))
		r.Get("/users/me", h.getMe)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(domain.RoleAdmin))
			r.Get("/users", h.listUsers)
			r.Patch("/users/{id}/block", h.blockUser)
			r.Patch("/users/{id}/unblock", h.unblockUser)
			r.Patch("/users/{id}/balance/top-up", h.topUpBalance)
		})
		r.Get("/tasks", h.listTasks)
		r.Get("/tasks/{id}", h.getTask)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(domain.RoleCustomer))
			r.Post("/tasks", h.createTask)
			r.Patch("/tasks/{id}/publish", h.publishTask)
			r.Patch("/tasks/{id}/cancel", h.cancelTask)
			r.Patch("/tasks/{id}/confirm", h.confirmTask)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(domain.RoleExecutor))
			r.Patch("/tasks/{id}/complete", h.completeTask)
		})
		r.Get("/tasks/{id}/bids", h.listBids)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(domain.RoleExecutor))
			r.Post("/tasks/{id}/bids", h.createBid)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(domain.RoleCustomer))
			r.Patch("/tasks/{id}/bids/{bid_id}/accept", h.acceptBid)
		})
		r.Get("/payments", h.listPayments)
	})

	return r
}

func getPagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	return limit, offset
}
