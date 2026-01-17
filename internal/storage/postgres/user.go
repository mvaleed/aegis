package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/storage"
)

// UserRepository implements storage.UserRepository using PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new user repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create stores a new user.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		INSERT INTO users (
			id, email, password_hash, phone, username, full_name,
			user_type, status, email_verified, phone_verified,
			created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Phone,
		user.Username,
		user.FullName,
		string(user.Type),
		string(user.Status),
		user.EmailVerified,
		user.PhoneVerified,
		user.CreatedAt,
		user.UpdatedAt,
		user.Version,
	)

	return mapError(err)
}

// GetByID retrieves a user by their ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, email, password_hash, phone, username, full_name,
			   user_type, status, email_verified, phone_verified,
			   created_at, updated_at, deleted_at, version
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id)

	return r.scanUser(row)
}

// GetByEmail retrieves a user by their email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, email, password_hash, phone, username, full_name,
			   user_type, status, email_verified, phone_verified,
			   created_at, updated_at, deleted_at, version
		FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`, email)

	return r.scanUser(row)
}

// GetByUsername retrieves a user by their username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, email, password_hash, phone, username, full_name,
			   user_type, status, email_verified, phone_verified,
			   created_at, updated_at, deleted_at, version
		FROM users WHERE username = $1 AND deleted_at IS NULL`, username)

	return r.scanUser(row)
}

// Update saves changes to an existing user with optimistic locking.
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	db := getDB(ctx, r.pool)

	result, err := db.Exec(ctx, `
		UPDATE users SET
			email = $2,
			password_hash = $3,
			phone = $4,
			username = $5,
			full_name = $6,
			user_type = $7,
			status = $8,
			email_verified = $9,
			phone_verified = $10,
			updated_at = $11,
			version = version + 1
		WHERE id = $1 AND version = $12 AND deleted_at IS NULL`,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Phone,
		user.Username,
		user.FullName,
		string(user.Type),
		string(user.Status),
		user.EmailVerified,
		user.PhoneVerified,
		time.Now().UTC(),
		user.Version,
	)
	if err != nil {
		return mapError(err)
	}

	if result.RowsAffected() == 0 {
		// Could be not found or version mismatch - check which
		existing, err := r.GetByID(ctx, user.ID)
		if err != nil {
			return err // Likely ErrNotFound
		}
		if existing.Version != user.Version {
			return domain.ErrVersionMismatch
		}
		return domain.ErrNotFound
	}

	user.Version++ // Update local version
	return nil
}

// Delete performs a soft delete.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := getDB(ctx, r.pool)

	result, err := db.Exec(ctx, `
		UPDATE users SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// List retrieves users with filtering and pagination.
func (r *UserRepository) List(ctx context.Context, filter storage.UserFilter) ([]domain.User, int64, error) {
	db := getDB(ctx, r.pool)

	// Set defaults
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Build query dynamically based on filter
	// Using parameterized queries to prevent SQL injection
	args := []any{}
	argIndex := 1

	whereClause := ""
	if !filter.Deleted {
		whereClause = "deleted_at IS NULL"
	}

	if filter.Status != nil {
		if whereClause != "" {
			whereClause += " AND "
		}
		whereClause += "status = $" + string(rune('0'+argIndex))
		args = append(args, string(*filter.Status))
		argIndex++
	}

	if filter.Type != nil {
		if whereClause != "" {
			whereClause += " AND "
		}
		whereClause += "user_type = $" + string(rune('0'+argIndex))
		args = append(args, string(*filter.Type))
		argIndex++
	}

	if filter.Search != "" {
		if whereClause != "" {
			whereClause += " AND "
		}
		whereClause += "(LOWER(email) LIKE LOWER($" + string(rune('0'+argIndex)) + ") OR " +
			"LOWER(username) LIKE LOWER($" + string(rune('0'+argIndex)) + ") OR " +
			"LOWER(full_name) LIKE LOWER($" + string(rune('0'+argIndex)) + "))"
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}

	if whereClause == "" {
		whereClause = "1=1"
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM users WHERE " + whereClause
	var total int64
	err := db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, mapError(err)
	}

	// Get page
	listArgs := append(args, filter.Limit, filter.Offset)
	listQuery := `
		SELECT id, email, password_hash, phone, username, full_name,
			   user_type, status, email_verified, phone_verified,
			   created_at, updated_at, deleted_at, version
		FROM users WHERE ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + string(rune('0'+argIndex)) + ` OFFSET $` + string(rune('0'+argIndex+1))

	rows, err := db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, mapError(err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		user, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, mapError(err)
	}

	return users, total, nil
}

// scannable is satisfied by both pgx.Row and pgx.Rows
type scannable interface {
	Scan(dest ...any) error
}

func (r *UserRepository) scanUser(row scannable) (*domain.User, error) {
	var user domain.User
	var userType, status string

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Phone,
		&user.Username,
		&user.FullName,
		&userType,
		&status,
		&user.EmailVerified,
		&user.PhoneVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
		&user.Version,
	)
	if err != nil {
		return nil, mapError(err)
	}

	user.Type = domain.UserType(userType)
	user.Status = domain.UserStatus(status)

	return &user, nil
}

func (r *UserRepository) scanUserFromRows(rows scannable) (*domain.User, error) {
	return r.scanUser(rows)
}
