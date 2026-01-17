// Package http provides the HTTP transport layer for the user service.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mvaleed/aegis/internal/auth"
	"github.com/mvaleed/aegis/internal/config"
	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
)

// Server is the HTTP server for the user service.
type Server struct {
	httpServer  *http.Server
	router      *chi.Mux
	userService *service.UserService
	authService *service.AuthService
	rbacService *service.RBACService
	jwtManager  *auth.JWTManager
	logger      *slog.Logger
}

// NewServer creates a new HTTP server.
func NewServer(
	cfg *config.Config,
	userService *service.UserService,
	authService *service.AuthService,
	rbacService *service.RBACService,
	jwtManager *auth.JWTManager,
	logger *slog.Logger,
) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		userService: userService,
		authService: authService,
		rbacService: rbacService,
		jwtManager:  jwtManager,
		logger:      logger,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// ListenAndServe starts the HTTP server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(s.loggingMiddleware)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))
}

func (s *Server) setupRoutes() {
	// Health check
	s.router.Get("/health", s.handleHealth)

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		// Public routes (no auth required)
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/refresh", s.handleRefreshToken)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// Auth
			r.Post("/auth/logout", s.handleLogout)
			r.Post("/auth/logout-all", s.handleLogoutAll)

			// Users
			r.Get("/users/me", s.handleGetCurrentUser)
			r.Put("/users/me", s.handleUpdateCurrentUser)
			r.Put("/users/me/password", s.handleChangePassword)

			// Admin routes
			r.Route("/users", func(r chi.Router) {
				r.Use(s.requirePermission("users", "read"))
				r.Get("/", s.handleListUsers)
				r.Get("/{id}", s.handleGetUser)

				r.Group(func(r chi.Router) {
					r.Use(s.requirePermission("users", "write"))
					r.Put("/{id}", s.handleUpdateUser)
					r.Post("/{id}/activate", s.handleActivateUser)
					r.Post("/{id}/suspend", s.handleSuspendUser)
				})

				r.Group(func(r chi.Router) {
					r.Use(s.requirePermission("users", "delete"))
					r.Delete("/{id}", s.handleDeleteUser)
				})
			})

			// User role assignment (admin only)
			r.Group(func(r chi.Router) {
				r.Use(s.requirePermission("roles", "assign"))
				r.Post("/users/{id}/roles", s.handleAssignRoleToUser)
				r.Delete("/users/{id}/roles/{roleId}", s.handleRemoveRoleFromUser)
			})

			// Roles (admin only)
			r.Route("/roles", func(r chi.Router) {
				r.Use(s.requirePermission("roles", "read"))
				r.Get("/", s.handleListRoles)
				r.Get("/{id}", s.handleGetRole)

				r.Group(func(r chi.Router) {
					r.Use(s.requirePermission("roles", "write"))
					r.Post("/", s.handleCreateRole)
					r.Put("/{id}", s.handleUpdateRole)
					r.Post("/{id}/permissions", s.handleAddPermissionToRole)
					r.Delete("/{id}/permissions/{permissionId}", s.handleRemovePermissionFromRole)
				})

				r.Group(func(r chi.Router) {
					r.Use(s.requirePermission("roles", "delete"))
					r.Delete("/{id}", s.handleDeleteRole)
				})
			})

			// Permissions (admin only)
			r.Route("/permissions", func(r chi.Router) {
				r.Use(s.requirePermission("permissions", "read"))
				r.Get("/", s.handleListPermissions)
				r.Get("/{id}", s.handleGetPermission)

				r.Group(func(r chi.Router) {
					r.Use(s.requirePermission("permissions", "write"))
					r.Post("/", s.handleCreatePermission)
				})

				r.Group(func(r chi.Router) {
					r.Use(s.requirePermission("permissions", "delete"))
					r.Delete("/{id}", s.handleDeletePermission)
				})
			})
		})
	})
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Response helpers

type errorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	var status int
	var resp errorResponse

	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		resp = errorResponse{Error: "resource not found", Code: "NOT_FOUND"}

	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
		resp = errorResponse{Error: "resource already exists", Code: "ALREADY_EXISTS"}

	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
		resp = errorResponse{Error: err.Error(), Code: "INVALID_INPUT"}
		if ve, ok := err.(domain.ValidationErrors); ok {
			resp.Details = make(map[string]string)
			for _, e := range ve {
				resp.Details[e.Field] = e.Message
			}
		} else if ve, ok := err.(domain.ValidationError); ok {
			resp.Details = map[string]string{ve.Field: ve.Message}
		}

	case errors.Is(err, domain.ErrInvalidCredential):
		status = http.StatusUnauthorized
		resp = errorResponse{Error: "invalid credentials", Code: "INVALID_CREDENTIALS"}

	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusUnauthorized
		resp = errorResponse{Error: "unauthorized", Code: "UNAUTHORIZED"}

	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
		resp = errorResponse{Error: "forbidden", Code: "FORBIDDEN"}

	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		resp = errorResponse{Error: "conflict", Code: "CONFLICT"}

	case errors.Is(err, domain.ErrVersionMismatch):
		status = http.StatusConflict
		resp = errorResponse{Error: "resource was modified by another request", Code: "VERSION_MISMATCH"}

	default:
		s.logger.Error("unhandled error", slog.String("error", err.Error()))
		status = http.StatusInternalServerError
		resp = errorResponse{Error: "internal server error", Code: "INTERNAL_ERROR"}
	}

	s.writeJSON(w, status, resp)
}

func (s *Server) readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return domain.ValidationError{Field: "body", Message: "invalid JSON"}
	}
	return nil
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, r)

		s.logger.Info("http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.status),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Context helpers

type contextKey string

const (
	userClaimsKey contextKey = "user_claims"
)

func setUserClaims(ctx context.Context, claims *userClaims) context.Context {
	return context.WithValue(ctx, userClaimsKey, claims)
}

func getUserClaims(ctx context.Context) *userClaims {
	if claims, ok := ctx.Value(userClaimsKey).(*userClaims); ok {
		return claims
	}
	return nil
}
