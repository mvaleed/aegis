package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Claims represents the JWT claims for access tokens.
type Claims struct {
	jwt.RegisteredClaims
	UserID      uuid.UUID `json:"uid"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	UserType    string    `json:"user_type"`
	Permissions []string  `json:"permissions,omitempty"`
}

// JWTConfig holds configuration for JWT token generation.
type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
	Audience        []string
}

// DefaultJWTConfig returns sensible defaults for JWT configuration.
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour, // 7 days
		Issuer:          "user-service",
		Audience:        []string{"user-service"},
	}
}

type JWTManager struct {
	config JWTConfig
}

func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{config: config}
}

// TokenPayload contains the information needed to generate tokens.
type TokenPayload struct {
	UserID      uuid.UUID
	Email       string
	Username    string
	UserType    string
	Permissions []string
}

func (m *JWTManager) GenerateAccessToken(payload TokenPayload) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.config.AccessTokenTTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   payload.UserID.String(),
			Issuer:    m.config.Issuer,
			Audience:  m.config.Audience,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID:      payload.UserID,
		Email:       payload.Email,
		Username:    payload.Username,
		UserType:    payload.UserType,
		Permissions: payload.Permissions,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.config.SecretKey))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(m.config.SecretKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (m *JWTManager) RefreshTokenTTL() time.Duration {
	return m.config.RefreshTokenTTL
}

func (m *JWTManager) AccessTokenTTL() time.Duration {
	return m.config.AccessTokenTTL
}
