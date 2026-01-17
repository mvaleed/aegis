package http

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// userClaims holds the authenticated user's information from the JWT.
type userClaims struct {
	UserID      uuid.UUID
	Email       string
	Username    string
	UserType    string
	Permissions []string
}

// hasPermission checks if the user has a specific permission.
func (c *userClaims) hasPermission(resource, action string) bool {
	target := resource + ":" + action
	wildcard := resource + ":*"
	superAdmin := "*:*"
	actionWildcard := "*:" + action

	for _, p := range c.Permissions {
		if p == target || p == wildcard || p == superAdmin || p == actionWildcard {
			return true
		}
	}
	return false
}

// authMiddleware validates JWT tokens and sets user claims in context.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "missing authorization header",
				Code:  "UNAUTHORIZED",
			})
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			s.writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "invalid authorization header format",
				Code:  "UNAUTHORIZED",
			})
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := s.authService.ValidateToken(r.Context(), tokenString)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error: "invalid or expired token",
				Code:  "UNAUTHORIZED",
			})
			return
		}

		// Set claims in context
		userClaims := &userClaims{
			UserID:      claims.UserID,
			Email:       claims.Email,
			Username:    claims.Username,
			UserType:    claims.UserType,
			Permissions: claims.Permissions,
		}

		ctx := setUserClaims(r.Context(), userClaims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requirePermission returns middleware that checks for a specific permission.
func (s *Server) requirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := getUserClaims(r.Context())
			if claims == nil {
				s.writeJSON(w, http.StatusUnauthorized, errorResponse{
					Error: "unauthorized",
					Code:  "UNAUTHORIZED",
				})
				return
			}

			if !claims.hasPermission(resource, action) {
				s.writeJSON(w, http.StatusForbidden, errorResponse{
					Error: "you don't have permission to perform this action",
					Code:  "FORBIDDEN",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For first (set by proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Get the first IP (client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Fall back to X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}
