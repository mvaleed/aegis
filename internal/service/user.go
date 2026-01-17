// Package service contains the business logic layer.
// Services orchestrate operations across repositories, handle transactions,
// and publish events. They do not know about HTTP, gRPC, or transport details.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mvaleed/aegis/internal/auth"
	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/event"
	"github.com/mvaleed/aegis/internal/storage"
)

// UserService handles user-related business operations.
type UserService struct {
	users     storage.UserRepository
	roles     storage.RoleRepository
	publisher event.Publisher
}

func NewUserService(
	users storage.UserRepository,
	roles storage.RoleRepository,
	publisher event.Publisher,
) *UserService {
	return &UserService{
		users:     users,
		roles:     roles,
		publisher: publisher,
	}
}

type CreateUserInput struct {
	Email    string
	Password string
	Username string
	FullName string
	Type     domain.UserType
	Phone    string
	UserType domain.UserType
}

// CreateUser creates a new user account.
func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	if err := auth.ValidatePasswordStrength(input.Password); err != nil {
		return nil, domain.ValidationError{Field: "password", Message: err.Error()}
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user, err := domain.NewUser(input.Email, input.Username, input.FullName, input.Type)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = passwordHash

	if input.Phone != "" {
		if err := user.SetPhone(input.Phone); err != nil {
			return nil, err
		}
	}

	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			// Be specific about what exists
			if _, emailErr := s.users.GetByEmail(ctx, input.Email); emailErr == nil {
				return nil, domain.ValidationError{Field: "email", Message: "already taken"}
			}
			if _, userErr := s.users.GetByUsername(ctx, input.Username); userErr == nil {
				return nil, domain.ValidationError{Field: "username", Message: "already taken"}
			}
		}
		return nil, err
	}

	defaultRole, err := s.roles.GetByName(ctx, "user")
	if err == nil {
		_ = s.roles.AssignRole(ctx, user.ID, defaultRole.ID)
	}

	_ = s.publisher.Publish(ctx, domain.UserCreatedEvent(user))

	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	roles, err := s.roles.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	return user, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	roles, err := s.roles.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	return user, nil
}

type UpdateUserInput struct {
	FullName *string
	Phone    *string
	Username *string
}

func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.FullName != nil {
		user.FullName = *input.FullName
	}

	if input.Username != nil {
		user.Username = *input.Username
	}

	if input.Phone != nil {
		if err := user.SetPhone(*input.Phone); err != nil {
			return nil, err
		}
	}

	if err := user.Validate(); err != nil {
		return nil, err
	}

	user.UpdatedAt = time.Now().UTC()

	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}

	_ = s.publisher.Publish(ctx, domain.NewEvent(domain.EventUserUpdated, user.ID, nil))

	return user, nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := auth.CheckPassword(currentPassword, user.PasswordHash); err != nil {
		return domain.ErrInvalidCredential
	}

	if err := auth.ValidatePasswordStrength(newPassword); err != nil {
		return domain.ValidationError{Field: "new_password", Message: err.Error()}
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = newHash
	user.UpdatedAt = time.Now().UTC()

	if err := s.users.Update(ctx, user); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.NewEvent(domain.EventPasswordChanged, user.ID, nil))

	return nil
}

// ActivateUser activates a user account.
func (s *UserService) ActivateUser(ctx context.Context, id uuid.UUID) error {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := user.Activate(); err != nil {
		return err
	}

	if err := s.users.Update(ctx, user); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.UserActivatedEvent(user))

	return nil
}

// SuspendUser suspends a user account.
func (s *UserService) SuspendUser(ctx context.Context, id uuid.UUID, reason string) error {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := user.Suspend(); err != nil {
		return err
	}

	if err := s.users.Update(ctx, user); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.UserSuspendedEvent(user, reason))

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if err := s.users.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.UserDeletedEvent(id))

	return nil
}

func (s *UserService) ListUsers(ctx context.Context, filter storage.UserFilter) ([]domain.User, int64, error) {
	return s.users.List(ctx, filter)
}

func (s *UserService) VerifyEmail(ctx context.Context, userID uuid.UUID) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	user.VerifyEmail()

	if err := s.users.Update(ctx, user); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.NewEvent(domain.EventUserEmailVerified, user.ID, nil))

	return nil
}

func (s *UserService) VerifyPhone(ctx context.Context, userID uuid.UUID) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	user.VerifyPhone()

	if err := s.users.Update(ctx, user); err != nil {
		return err
	}

	_ = s.publisher.Publish(ctx, domain.NewEvent(domain.EventUserPhoneVerified, user.ID, nil))

	return nil
}
