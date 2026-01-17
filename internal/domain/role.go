package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Permission represents a single permission in the resource:action model.
type Permission struct {
	ID          uuid.UUID
	Resource    string // e.g., "users", "orders", "reports"
	Action      string // e.g., "read", "write", "delete", "admin"
	Description string
	CreatedAt   time.Time
}

// NewPermission creates a validated permission.
func NewPermission(resource, action, description string) (*Permission, error) {
	p := &Permission{
		ID:          uuid.New(),
		Resource:    strings.ToLower(strings.TrimSpace(resource)),
		Action:      strings.ToLower(strings.TrimSpace(action)),
		Description: strings.TrimSpace(description),
		CreatedAt:   time.Now().UTC(),
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate checks the permission fields.
func (p *Permission) Validate() error {
	var errs ValidationErrors

	if p.Resource == "" {
		errs = append(errs, ValidationError{Field: "resource", Message: "required"})
	} else if len(p.Resource) > 50 {
		errs = append(errs, ValidationError{Field: "resource", Message: "must be at most 50 characters"})
	}

	if p.Action == "" {
		errs = append(errs, ValidationError{Field: "action", Message: "required"})
	} else if len(p.Action) > 50 {
		errs = append(errs, ValidationError{Field: "action", Message: "must be at most 50 characters"})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// String returns the permission in resource:action format.
func (p *Permission) String() string {
	return p.Resource + ":" + p.Action
}

// Role represents a named collection of permissions.
type Role struct {
	ID          uuid.UUID
	Name        string
	Description string
	Permissions []Permission
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewRole creates a validated role.
func NewRole(name, description string) (*Role, error) {
	r := &Role{
		ID:          uuid.New(),
		Name:        strings.ToLower(strings.TrimSpace(name)),
		Description: strings.TrimSpace(description),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := r.Validate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Role) Validate() error {
	var errs ValidationErrors

	if r.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "required"})
	} else if len(r.Name) > 50 {
		errs = append(errs, ValidationError{Field: "name", Message: "must be at most 50 characters"})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (r *Role) HasPermission(resource, action string) bool {
	for _, p := range r.Permissions {
		if p.Resource == resource && p.Action == action {
			return true
		}
		// Support wildcard action
		if p.Resource == resource && p.Action == "*" {
			return true
		}
		// Support wildcard resource
		if p.Resource == "*" && p.Action == action {
			return true
		}
		// Super admin: *:*
		if p.Resource == "*" && p.Action == "*" {
			return true
		}
	}
	return false
}

// AddPermission adds a permission to the role if not already present.
func (r *Role) AddPermission(p Permission) {
	for _, existing := range r.Permissions {
		if existing.ID == p.ID {
			return // Already has this permission
		}
	}
	r.Permissions = append(r.Permissions, p)
	r.UpdatedAt = time.Now().UTC()
}

// RemovePermission removes a permission from the role.
func (r *Role) RemovePermission(permissionID uuid.UUID) {
	for i, p := range r.Permissions {
		if p.ID == permissionID {
			r.Permissions = append(r.Permissions[:i], r.Permissions[i+1:]...)
			r.UpdatedAt = time.Now().UTC()
			return
		}
	}
}

// PermissionStrings returns all permissions as resource:action strings.
func (r *Role) PermissionStrings() []string {
	result := make([]string, len(r.Permissions))
	for i, p := range r.Permissions {
		result[i] = p.String()
	}
	return result
}
