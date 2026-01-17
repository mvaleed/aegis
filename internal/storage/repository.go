// Package storage defines the repository interfaces for data persistence.
//
// These interfaces allow the business logic to remain independent of the
// storage implementation. Today we use PostgreSQL, but we could swap to
// MongoDB, DynamoDB, or any other database without changing the service layer.
//
// Design Decision: We define these interfaces here because we have a concrete
// requirement for database swappability. If we only ever used one database,
// we wouldn't need this abstraction.
package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/mvaleed/aegis/internal/domain"
)

// UserRepository defines the operations for user persistence.
type UserRepository interface {
	// Create stores a new user. Returns ErrAlreadyExists if email/username is taken.
	Create(ctx context.Context, user *domain.User) error

	// GetByID retrieves a user by their ID. Returns ErrNotFound if not found.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)

	// GetByEmail retrieves a user by their email. Returns ErrNotFound if not found.
	GetByEmail(ctx context.Context, email string) (*domain.User, error)

	// GetByUsername retrieves a user by their username. Returns ErrNotFound if not found.
	GetByUsername(ctx context.Context, username string) (*domain.User, error)

	// Update saves changes to an existing user. Uses optimistic locking via version.
	// Returns ErrVersionMismatch if the version doesn't match.
	// Returns ErrNotFound if the user doesn't exist.
	Update(ctx context.Context, user *domain.User) error

	// Delete performs a soft delete. Returns ErrNotFound if the user doesn't exist.
	Delete(ctx context.Context, id uuid.UUID) error

	// List retrieves users with pagination and optional filtering.
	List(ctx context.Context, filter UserFilter) ([]domain.User, int64, error)
}

// UserFilter contains options for filtering and paginating user lists.
type UserFilter struct {
	Status  *domain.UserStatus
	Type    *domain.UserType
	Search  string // Searches email, username, full_name
	Offset  int
	Limit   int
	Deleted bool // If true, include soft-deleted users
}

// RoleRepository defines operations for role persistence.
type RoleRepository interface {
	// Create stores a new role. Returns ErrAlreadyExists if name is taken.
	Create(ctx context.Context, role *domain.Role) error

	// GetByID retrieves a role by ID with its permissions.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)

	// GetByName retrieves a role by name with its permissions.
	GetByName(ctx context.Context, name string) (*domain.Role, error)

	// Update saves changes to an existing role.
	Update(ctx context.Context, role *domain.Role) error

	// Delete removes a role. Returns ErrConflict if users are assigned to it.
	Delete(ctx context.Context, id uuid.UUID) error

	// List retrieves all roles.
	List(ctx context.Context) ([]domain.Role, error)

	// GetUserRoles retrieves all roles assigned to a user.
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)

	// AssignRole assigns a role to a user. Idempotent - no error if already assigned.
	AssignRole(ctx context.Context, userID, roleID uuid.UUID) error

	// RemoveRole removes a role from a user. Idempotent - no error if not assigned.
	RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error
}

// PermissionRepository defines operations for permission persistence.
type PermissionRepository interface {
	// Create stores a new permission. Returns ErrAlreadyExists if resource:action exists.
	Create(ctx context.Context, perm *domain.Permission) error

	// GetByID retrieves a permission by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error)

	// GetByResourceAction retrieves a permission by its resource and action.
	GetByResourceAction(ctx context.Context, resource, action string) (*domain.Permission, error)

	// List retrieves all permissions.
	List(ctx context.Context) ([]domain.Permission, error)

	// Delete removes a permission. Returns ErrConflict if roles use it.
	Delete(ctx context.Context, id uuid.UUID) error

	// AssignToRole assigns a permission to a role. Idempotent.
	AssignToRole(ctx context.Context, roleID, permissionID uuid.UUID) error

	// RemoveFromRole removes a permission from a role. Idempotent.
	RemoveFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error
}

// TokenRepository defines operations for refresh token persistence.
type TokenRepository interface {
	// Create stores a new refresh token.
	Create(ctx context.Context, token *domain.RefreshToken) error

	// GetByHash retrieves a token by its hash.
	GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)

	// Revoke marks a token as revoked.
	Revoke(ctx context.Context, id uuid.UUID) error

	// RevokeAllForUser revokes all tokens for a user.
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error

	// DeleteExpired removes expired tokens older than the given duration.
	DeleteExpired(ctx context.Context) (int64, error)
}

// Repositories bundles all repositories together.
// This makes it easy to pass around and inject dependencies.
type Repositories struct {
	Users       UserRepository
	Roles       RoleRepository
	Permissions PermissionRepository
	Tokens      TokenRepository
}

// Transactor provides transaction support for operations that need atomicity.
// Not all operations need transactions, so we keep this separate.
type Transactor interface {
	// WithTransaction executes fn within a database transaction.
	// If fn returns an error, the transaction is rolled back.
	// If fn succeeds, the transaction is committed.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
