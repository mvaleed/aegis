package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mvaleed/aegis/internal/domain"
)

// TokenRepository implements storage.TokenRepository using PostgreSQL.
type TokenRepository struct {
	pool *pgxpool.Pool
}

// NewTokenRepository creates a new token repository.
func NewTokenRepository(pool *pgxpool.Pool) *TokenRepository {
	return &TokenRepository{pool: pool}
}

// Create stores a new refresh token.
func (r *TokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		INSERT INTO refresh_tokens (
			id, user_id, token_hash, expires_at, created_at, ip_address, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
		token.IPAddress,
		token.UserAgent,
	)

	return mapError(err)
}

// GetByHash retrieves a token by its hash.
func (r *TokenRepository) GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	db := getDB(ctx, r.pool)

	row := db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at,
			   revoked_at, ip_address, user_agent, replaced_by_id
		FROM refresh_tokens WHERE token_hash = $1`, hash)

	return r.scanToken(row)
}

// Revoke marks a token as revoked.
func (r *TokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	db := getDB(ctx, r.pool)

	result, err := db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return mapError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// RevokeAllForUser revokes all tokens for a user.
func (r *TokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	db := getDB(ctx, r.pool)

	_, err := db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID)

	return mapError(err)
}

// DeleteExpired removes expired tokens.
func (r *TokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	db := getDB(ctx, r.pool)

	result, err := db.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < NOW() - INTERVAL '7 days'`)
	if err != nil {
		return 0, mapError(err)
	}

	return result.RowsAffected(), nil
}

func (r *TokenRepository) scanToken(row scannable) (*domain.RefreshToken, error) {
	var token domain.RefreshToken

	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.RevokedAt,
		&token.IPAddress,
		&token.UserAgent,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &token, nil
}
