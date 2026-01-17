package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mvaleed/aegis/internal/domain"
)

// PermissionRepository implements storage.PermissionRepository using PostgreSQL.
type PermissionRepository struct {
	pool *pgxpool.Pool
}

// NewPermissionRepository creates a new permission repository.
func NewPermissionRepository(pool *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{pool: pool}
}

// Create stores a new permission.
func (r *PermissionRepository) Create(ctx context.Context, perm *domain.Permission) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		INSERT INTO permissions (id, resource, action, description, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		perm.ID,
		perm.Resource,
		perm.Action,
		perm.Description,
		perm.CreatedAt,
	)

	return mapError(err)
}

// GetByID retrieves a permission by ID.
func (r *PermissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, resource, action, description, created_at
		FROM permissions WHERE id = $1`, id)

	return r.scanPermission(row)
}

// GetByResourceAction retrieves a permission by its resource and action.
func (r *PermissionRepository) GetByResourceAction(ctx context.Context, resource, action string) (*domain.Permission, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, resource, action, description, created_at
		FROM permissions WHERE resource = $1 AND action = $2`, resource, action)

	return r.scanPermission(row)
}

// List retrieves all permissions.
func (r *PermissionRepository) List(ctx context.Context) ([]domain.Permission, error) {
	db := getDB(ctx, r.pool)

	rows, err := db.Query(ctx, `
		SELECT id, resource, action, description, created_at
		FROM permissions ORDER BY resource, action`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		perm, err := r.scanPermission(rows)
		if err != nil {
			return nil, err
		}
		perms = append(perms, *perm)
	}

	return perms, nil
}

// Delete removes a permission.
func (r *PermissionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := getDB(ctx, r.pool)

	// Check if any roles have this permission
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM role_permissions WHERE permission_id = $1`, id).Scan(&count)
	if err != nil {
		return mapError(err)
	}
	if count > 0 {
		return domain.ErrConflict
	}

	result, err := db.Exec(ctx, `DELETE FROM permissions WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// AssignToRole assigns a permission to a role.
func (r *PermissionRepository) AssignToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2)
		ON CONFLICT (role_id, permission_id) DO NOTHING`,
		roleID, permissionID)

	return mapError(err)
}

// RemoveFromRole removes a permission from a role.
func (r *PermissionRepository) RemoveFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		DELETE FROM role_permissions
		WHERE role_id = $1 AND permission_id = $2`,
		roleID, permissionID)

	return mapError(err)
}

func (r *PermissionRepository) scanPermission(row scannable) (*domain.Permission, error) {
	var perm domain.Permission

	err := row.Scan(
		&perm.ID,
		&perm.Resource,
		&perm.Action,
		&perm.Description,
		&perm.CreatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &perm, nil
}
