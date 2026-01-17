package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mvaleed/aegis/internal/domain"
)

// RoleRepository implements storage.RoleRepository using PostgreSQL.
type RoleRepository struct {
	pool *pgxpool.Pool
}

// NewRoleRepository creates a new role repository.
func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{pool: pool}
}

// Create stores a new role.
func (r *RoleRepository) Create(ctx context.Context, role *domain.Role) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)`,
		role.ID,
		role.Name,
		role.Description,
		role.CreatedAt,
		role.UpdatedAt,
	)

	return mapError(err)
}

// GetByID retrieves a role by ID with its permissions.
func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM roles WHERE id = $1`, id)

	role, err := r.scanRole(row)
	if err != nil {
		return nil, err
	}

	// Load permissions
	permissions, err := r.getRolePermissions(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	role.Permissions = permissions

	return role, nil
}

// GetByName retrieves a role by name with its permissions.
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM roles WHERE name = $1`, name)

	role, err := r.scanRole(row)
	if err != nil {
		return nil, err
	}

	// Load permissions
	permissions, err := r.getRolePermissions(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	role.Permissions = permissions

	return role, nil
}

// Update saves changes to an existing role.
func (r *RoleRepository) Update(ctx context.Context, role *domain.Role) error {
	db := getDB(ctx, r.pool)

	result, err := db.Exec(ctx, `
		UPDATE roles SET name = $2, description = $3, updated_at = $4
		WHERE id = $1`,
		role.ID,
		role.Name,
		role.Description,
		time.Now().UTC(),
	)
	if err != nil {
		return mapError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Delete removes a role.
func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := getDB(ctx, r.pool)

	// Check if any users have this role
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM user_roles WHERE role_id = $1`, id).Scan(&count)
	if err != nil {
		return mapError(err)
	}
	if count > 0 {
		return domain.ErrConflict
	}

	result, err := db.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// List retrieves all roles.
func (r *RoleRepository) List(ctx context.Context) ([]domain.Role, error) {
	db := getDB(ctx, r.pool)

	rows, err := db.Query(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM roles ORDER BY name`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		role, err := r.scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}

	// Load permissions for each role
	for i := range roles {
		perms, err := r.getRolePermissions(ctx, roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Permissions = perms
	}

	return roles, nil
}

// GetUserRoles retrieves all roles assigned to a user.
func (r *RoleRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	db := getDB(ctx, r.pool)

	rows, err := db.Query(ctx, `
		SELECT r.id, r.name, r.description, r.created_at, r.updated_at
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name`, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		role, err := r.scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}

	// Load permissions for each role
	for i := range roles {
		perms, err := r.getRolePermissions(ctx, roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Permissions = perms
	}

	return roles, nil
}

// AssignRole assigns a role to a user.
func (r *RoleRepository) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, role_id) DO NOTHING`,
		userID, roleID)

	return mapError(err)
}

// RemoveRole removes a role from a user.
func (r *RoleRepository) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1 AND role_id = $2`,
		userID, roleID)

	return mapError(err)
}

func (r *RoleRepository) getRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	db := getDB(ctx, r.pool)

	rows, err := db.Query(ctx, `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action`, roleID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt)
		if err != nil {
			return nil, mapError(err)
		}
		perms = append(perms, p)
	}

	return perms, nil
}

func (r *RoleRepository) scanRole(row scannable) (*domain.Role, error) {
	var role domain.Role

	err := row.Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &role, nil
}
