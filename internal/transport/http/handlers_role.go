package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mvaleed/aegis/internal/domain"
)

// Role response types

type roleResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Permissions []permissionResponse `json:"permissions,omitempty"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
}

type permissionResponse struct {
	ID          string `json:"id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

func toRoleResponse(r *domain.Role) roleResponse {
	resp := roleResponse{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	for _, p := range r.Permissions {
		resp.Permissions = append(resp.Permissions, toPermissionResponse(&p))
	}

	return resp
}

func toPermissionResponse(p *domain.Permission) permissionResponse {
	return permissionResponse{
		ID:          p.ID.String(),
		Resource:    p.Resource,
		Action:      p.Action,
		Description: p.Description,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Role handlers

type createRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if req.Name == "" {
		s.writeError(w, domain.ValidationError{Field: "name", Message: "required"})
		return
	}

	role, err := s.rbacService.CreateRole(r.Context(), req.Name, req.Description)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusCreated, toRoleResponse(role))
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.rbacService.ListRoles(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}

	roleResponses := make([]roleResponse, len(roles))
	for i, role := range roles {
		roleResponses[i] = toRoleResponse(&role)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"roles": roleResponses,
		"total": len(roles),
	})
}

func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	role, err := s.rbacService.GetRole(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toRoleResponse(role))
}

type updateRoleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (s *Server) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	var req updateRoleRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	// Get current values if not provided
	name := ""
	description := ""
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		description = *req.Description
	}

	role, err := s.rbacService.UpdateRole(r.Context(), id, name, description)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toRoleResponse(role))
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	if err := s.rbacService.DeleteRole(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusNoContent, nil)
}

// Role-Permission management

type addPermissionRequest struct {
	PermissionID string `json:"permission_id"`
}

func (s *Server) handleAddPermissionToRole(w http.ResponseWriter, r *http.Request) {
	roleIDStr := chi.URLParam(r, "id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	var req addPermissionRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	permID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "permission_id", Message: "invalid UUID"})
		return
	}

	if err := s.rbacService.AddPermissionToRole(r.Context(), roleID, permID); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "permission added to role"})
}

func (s *Server) handleRemovePermissionFromRole(w http.ResponseWriter, r *http.Request) {
	roleIDStr := chi.URLParam(r, "id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	permIDStr := chi.URLParam(r, "permissionId")
	permID, err := uuid.Parse(permIDStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "permission_id", Message: "invalid UUID"})
		return
	}

	if err := s.rbacService.RemovePermissionFromRole(r.Context(), roleID, permID); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "permission removed from role"})
}

// User-Role management

type assignRoleRequest struct {
	RoleID string `json:"role_id"`
}

func (s *Server) handleAssignRoleToUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	var req assignRoleRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "role_id", Message: "invalid UUID"})
		return
	}

	if err := s.rbacService.AssignRole(r.Context(), userID, roleID); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "role assigned to user"})
}

func (s *Server) handleRemoveRoleFromUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "role_id", Message: "invalid UUID"})
		return
	}

	if err := s.rbacService.RemoveRole(r.Context(), userID, roleID); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "role removed from user"})
}

// Permission handlers

type createPermissionRequest struct {
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

func (s *Server) handleCreatePermission(w http.ResponseWriter, r *http.Request) {
	var req createPermissionRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if req.Resource == "" {
		s.writeError(w, domain.ValidationError{Field: "resource", Message: "required"})
		return
	}
	if req.Action == "" {
		s.writeError(w, domain.ValidationError{Field: "action", Message: "required"})
		return
	}

	perm, err := s.rbacService.CreatePermission(r.Context(), req.Resource, req.Action, req.Description)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusCreated, toPermissionResponse(perm))
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := s.rbacService.ListPermissions(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}

	permResponses := make([]permissionResponse, len(perms))
	for i, p := range perms {
		permResponses[i] = toPermissionResponse(&p)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"permissions": permResponses,
		"total":       len(perms),
	})
}

func (s *Server) handleGetPermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	perm, err := s.rbacService.GetPermission(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, toPermissionResponse(perm))
}

func (s *Server) handleDeletePermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.writeError(w, domain.ValidationError{Field: "id", Message: "invalid UUID"})
		return
	}

	if err := s.rbacService.DeletePermission(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusNoContent, nil)
}
