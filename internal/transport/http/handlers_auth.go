package http

import (
	"net/http"

	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
)

// Health check

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone,omitempty"`
}

type authResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	User         userResponse `json:"user"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	user, err := s.userService.CreateUser(r.Context(), service.CreateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Username: req.Username,
		FullName: req.FullName,
		Type:     domain.UserTypeCustomer,
		Phone:    req.Phone,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}

	// Auto-login after registration
	result, err := s.authService.Login(r.Context(), service.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// Registration succeeded but auto-login failed - just return user
		s.writeJSON(w, http.StatusCreated, map[string]any{
			"user": toUserResponse(user),
		})
		return
	}

	s.writeJSON(w, http.StatusCreated, authResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresInSeconds,
		User:         toUserResponse(result.User),
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	result, err := s.authService.Login(r.Context(), service.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresInSeconds,
		User:         toUserResponse(result.User),
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if req.RefreshToken == "" {
		s.writeError(w, domain.ValidationError{Field: "refresh_token", Message: "required"})
		return
	}

	result, err := s.authService.RefreshToken(r.Context(), service.RefreshTokenInput{
		RefreshToken: req.RefreshToken,
		IPAddress:    getClientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresInSeconds,
		User:         toUserResponse(result.User),
	})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if err := s.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		s.writeError(w, domain.ErrUnauthorized)
		return
	}

	if err := s.authService.LogoutAll(r.Context(), claims.UserID); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "logged out from all devices"})
}
