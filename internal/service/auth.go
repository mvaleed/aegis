package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mvaleed/aegis/internal/auth"
	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/event"
	"github.com/mvaleed/aegis/internal/storage"
)

// AuthService handles authentication operations.
type AuthService struct {
	users     storage.UserRepository
	roles     storage.RoleRepository
	tokens    storage.TokenRepository
	jwt       *auth.JWTManager
	publisher event.Publisher
}

func NewAuthService(
	users storage.UserRepository,
	roles storage.RoleRepository,
	tokens storage.TokenRepository,
	jwt *auth.JWTManager,
	publisher event.Publisher,
) *AuthService {
	return &AuthService{
		users:     users,
		roles:     roles,
		tokens:    tokens,
		jwt:       jwt,
		publisher: publisher,
	}
}

// LoginInput contains the credentials for login.
type LoginInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

// LoginResult contains the tokens and user info after successful login.
type LoginResult struct {
	AccessToken      string
	RefreshToken     string
	ExpiresInSeconds int64
	User             *domain.User
}

// Login authenticates a user and returns tokens.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	user, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredential
	}

	if err = auth.CheckPassword(input.Password, user.PasswordHash); err != nil {
		return nil, domain.ErrInvalidCredential
	}

	if !user.IsActive() {
		return nil, domain.ErrUnauthorized
	}

	roles, err := s.roles.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	tokens, err := s.generateTokens(ctx, user, input.IPAddress, input.UserAgent)
	if err != nil {
		return nil, err
	}

	if err = s.publisher.Publish(ctx, domain.UserLoggedInEvent(user.ID, input.IPAddress, input.UserAgent)); err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:      tokens.AccessToken,
		RefreshToken:     tokens.RefreshToken,
		ExpiresInSeconds: int64(s.jwt.AccessTokenTTL().Seconds()),
		User:             user,
	}, nil
}

// RefreshTokenInput contains the refresh token and metadata.
type RefreshTokenInput struct {
	RefreshToken string
	IPAddress    string
	UserAgent    string
}

func (s *AuthService) RefreshToken(ctx context.Context, input RefreshTokenInput) (*LoginResult, error) {
	// Hash the incoming token to look up stored record
	tokenHash := auth.HashToken(input.RefreshToken)

	storedToken, err := s.tokens.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil, domain.ErrInvalidCredential
	}

	if !storedToken.IsValid() {
		// Token reuse detection: if a revoked token is used, revoke all tokens for this user
		if storedToken.IsRevoked() {
			// Potential token theft - revoke all tokens for this user
			_ = s.tokens.RevokeAllForUser(ctx, storedToken.UserID)
		}
		return nil, domain.ErrInvalidCredential
	}

	user, err := s.users.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, domain.ErrInvalidCredential
	}

	if !user.IsActive() {
		_ = s.tokens.Revoke(ctx, storedToken.ID)
		return nil, domain.ErrUnauthorized
	}

	roles, err := s.roles.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	_ = s.tokens.Revoke(ctx, storedToken.ID)

	tokens, err := s.generateTokens(ctx, user, input.IPAddress, input.UserAgent)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:      tokens.AccessToken,
		RefreshToken:     tokens.RefreshToken,
		ExpiresInSeconds: int64(s.jwt.AccessTokenTTL().Seconds()),
		User:             user,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := auth.HashToken(refreshToken)

	storedToken, err := s.tokens.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil
	}

	if err := s.tokens.Revoke(ctx, storedToken.ID); err != nil {
		return err
	}

	if err := s.publisher.Publish(ctx, domain.NewEvent(domain.EventUserLoggedOut, storedToken.UserID, nil)); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.tokens.RevokeAllForUser(ctx, userID)
}

// ValidateToken validates an access token and returns the claims.
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*auth.Claims, error) {
	return s.jwt.ValidateAccessToken(token)
}

func (s *AuthService) generateTokens(ctx context.Context, user *domain.User, ipAddress, userAgent string) (*domain.TokenPair, error) {
	// Build permission strings for JWT
	permissions := make([]string, 0)
	for _, perm := range user.AllPermissions() {
		permissions = append(permissions, perm.String())
	}

	payload := auth.TokenPayload{
		UserID:      user.ID,
		Email:       user.Email,
		Username:    user.Username,
		UserType:    string(user.Type),
		Permissions: permissions,
	}

	accessToken, _, err := s.jwt.GenerateAccessToken(payload)
	if err != nil {
		return nil, err
	}

	refreshTokenString, err := domain.GenerateTokenString()
	if err != nil {
		return nil, err
	}

	refreshToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: auth.HashToken(refreshTokenString),
		ExpiresAt: time.Now().UTC().Add(s.jwt.RefreshTokenTTL()),
		CreatedAt: time.Now().UTC(),
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	if err := s.tokens.Create(ctx, refreshToken); err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(s.jwt.AccessTokenTTL().Seconds()),
	}, nil
}

// CleanupExpiredTokens removes old expired tokens from the database.
func (s *AuthService) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	return s.tokens.DeleteExpired(ctx)
}
