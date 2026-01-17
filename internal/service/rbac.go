package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/event"
	"github.com/mvaleed/aegis/internal/storage"
)

// RBACService handles role-based access control operations.
type RBACService struct {
	users       storage.UserRepository
	roles       storage.RoleRepository
	permissions storage.PermissionRepository
	publisher   event.Publisher
}

func NewRBACService(
	users storage.UserRepository,
	roles storage.RoleRepository,
	permissions storage.PermissionRepository,
	publisher event.Publisher,
) *RBACService {
	return &RBACService{
		users:       users,
		roles:       roles,
		permissions: permissions,
		publisher:   publisher,
	}
}

func (s *RBACService) CreateRole(ctx context.Context, name, description string) (*domain.Role, error) {
	role, err := domain.NewRole(name, description)
	if err != nil {
		return nil, err
	}

	if err := s.roles.Create(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *RBACService) GetRole(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return s.roles.GetByID(ctx, id)
}

func (s *RBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	return s.roles.GetByName(ctx, name)
}

func (s *RBACService) ListRoles(ctx context.Context) ([]domain.Role, error) {
	return s.roles.List(ctx)
}

func (s *RBACService) UpdateRole(ctx context.Context, id uuid.UUID, name, description string) (*domain.Role, error) {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	role.Name = name
	role.Description = description

	if err := role.Validate(); err != nil {
		return nil, err
	}

	if err := s.roles.Update(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *RBACService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return s.roles.Delete(ctx, id)
}

func (s *RBACService) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	// TODO: find better way to check user exists or not
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return err
	}

	// TODO: find better way to check role exists or not
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return err
	}

	if err := s.roles.AssignRole(ctx, userID, roleID); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.RoleAssignedEvent(userID, role.Name))

	return nil
}

// RemoveRole removes a role from a user
func (s *RBACService) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return err
	}

	if err := s.roles.RemoveRole(ctx, userID, roleID); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.RoleRemovedEvent(userID, role.Name))

	return nil
}

func (s *RBACService) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	return s.roles.GetUserRoles(ctx, userID)
}

func (s *RBACService) CreatePermission(ctx context.Context, resource, action, description string) (*domain.Permission, error) {
	perm, err := domain.NewPermission(resource, action, description)
	if err != nil {
		return nil, err
	}

	if err := s.permissions.Create(ctx, perm); err != nil {
		return nil, err
	}

	return perm, nil
}

func (s *RBACService) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.permissions.List(ctx)
}

func (s *RBACService) AddPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	if _, err := s.roles.GetByID(ctx, roleID); err != nil {
		return err
	}

	if _, err := s.permissions.GetByID(ctx, permissionID); err != nil {
		return err
	}

	return s.permissions.AssignToRole(ctx, roleID, permissionID)
}

func (s *RBACService) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return s.permissions.RemoveFromRole(ctx, roleID, permissionID)
}

func (s *RBACService) CheckPermission(ctx context.Context, userID uuid.UUID, resource, action string) (bool, error) {
	roles, err := s.roles.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role.HasPermission(resource, action) {
			return true, nil
		}
	}

	return false, nil
}

func (s *RBACService) GetPermission(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	return s.permissions.GetByID(ctx, id)
}

func (s *RBACService) DeletePermission(ctx context.Context, id uuid.UUID) error {
	return s.permissions.Delete(ctx, id)
}
