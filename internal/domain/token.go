package domain

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a stored refresh token for token rotation.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string // We store a hash, not the raw token
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time

	IPAddress string
	UserAgent string
}

func (t *RefreshToken) IsExpired() bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

func (t *RefreshToken) IsValid() bool {
	return !t.IsExpired() && !t.IsRevoked()
}

// Revoke marks the token as revoked.
func (t *RefreshToken) Revoke() {
	if t.RevokedAt == nil {
		now := time.Now().UTC()
		t.RevokedAt = &now
	}
}

// GenerateTokenString generates a cryptographically secure random token string.
// This is the raw token that will be sent to the client.
func GenerateTokenString() (string, error) {
	bytes := make([]byte, 32) // 256 bits of entropy
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// TokenPair represents an access token and refresh token pair.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // Seconds until access token expires
}
