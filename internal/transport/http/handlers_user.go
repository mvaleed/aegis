package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/service"
	"github.com/mvaleed/aegis/internal/storage"
)

// User response types

type userResponse struct {
	ID            string   `json:"id"`
	Email         string   `json:"email"`
	Username      string   `json:"username"`
	FullName      string   `json:"full_name"`
	Phone         *string  `json:"phone,omitempty"`
	Type          string   `json:"type"`
	Status        string   `json:"status"`
	EmailVerified bool     `json:"email_verified"`
	PhoneVerified bool     `json:"phone_verified"`
	Roles         []string `json:"roles,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

func toUserResponse(u *domain.User) userResponse {
	resp := userResponse{
		ID:            u.ID.String(),
		Email:         u.Email,
		Username:      u.Username,
		FullName:      u.FullName,
		Phone:         u.Phone,
		Type:          string(u.Type),
		Status:        string(u.Status),
		EmailVerified: u.EmailVerified,
		PhoneVerified: u.PhoneVerified,
		CreatedAt:     u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     u.UpdatedAt.Format(time.RFC3339),
	}

	for _, r := range u.Roles {
		resp.Roles = append(resp.Roles, r.Name)
	}

	return resp
}

// User handlers

func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		s.writeError(w, domain.ErrUnauthorized)
		return
	}

	user, err := s.userService.GetUser(r.Context(), claims.UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toUserResponse(user))
}

type updateUserRequest struct {
	FullName *string `json:"full_name,omitempty"`
	Username *string `json:"username,omitempty"`
	Phone    *string `json:"phone,omitempty"`
}

func (s *Server) handleUpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		s.writeError(w, domain.ErrUnauthorized)
		return
	}

	var req updateUserRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	user, err := s.userService.UpdateUser(r.Context(), claims.UserID, service.UpdateUserInput{
		FullName: req.FullName,
		Username: req.Username,
		Phone:    req.Phone,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toUserResponse(user))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		s.writeError(w, domain.ErrUnauthorized)
		return
	}

	var req changePasswordRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if req.CurrentPassword == "" {
		s.writeError(w, domain.ValidationError{Field: "current_password", Message: "required"})
		return
	}
	if req.NewPassword == "" {
		s.writeError(w, domain.ValidationError{Field: "new_password", Message: "required"})
		return
	}

	if err := s.userService.ChangePassword(r.Context(), claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "password changed successfully"})
}

// Admin user handlers

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := storage.UserFilter{
		Search: query.Get("search"),
		Offset: 0,
		Limit:  20,
	}

	if offset, err := strconv.Atoi(query.Get("offset")); err == nil && offset >= 0 {
		filter.Offset = offset
	}
	if limit, err := strconv.Atoi(query.Get("limit")); err == nil && limit > 0 && limit <= 100 {
		filter.Limit = limit
	}

	if status := query.Get("status"); status != "" {
		st := domain.UserStatus(status)
		if st.Valid() {
			filter.Status = &st
		}
	}

	if userType := query.Get("type"); userType != "" {
		ut := domain.UserType(userType)
		if ut.Valid() {
			filter.Type = &ut
		}
	}

	users, total, err := s.userService.ListUsers(r.Context(), filter)
	if err != nil {
		s.writeError(w, err)
		return
	}

	userResponses := make([]userResponse, len(users))
	for i, u := range users {
		userResponses[i] = toUserResponse(&u)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"users":  userResponses,
		"total":  total,
		"offset": filter.Offset,
		"limit":  filter.Limit,
	})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	user, err := s.userService.GetUser(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	var req updateUserRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	user, err := s.userService.UpdateUser(r.Context(), id, service.UpdateUserInput{
		FullName: req.FullName,
		Username: req.Username,
		Phone:    req.Phone,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleActivateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	if err := s.userService.ActivateUser(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "user activated"})
}

type suspendRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleSuspendUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	var req suspendRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if err := s.userService.SuspendUser(r.Context(), id, req.Reason); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "user suspended"})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	if err := s.userService.DeleteUser(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusNoContent, nil)
}
