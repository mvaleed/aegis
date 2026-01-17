package domain

import (
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// UserType represents the type/category of a user.
type UserType string

const (
	UserTypeAdmin    UserType = "admin"
	UserTypeCustomer UserType = "customer"
	UserTypePartner  UserType = "partner"
)

// Valid returns true if the UserType is recognized.
func (t UserType) Valid() bool {
	switch t {
	case UserTypeAdmin, UserTypeCustomer, UserTypePartner:
		return true
	}
	return false
}

// UserStatus represents the current state of a user account.
type UserStatus string

const (
	UserStatusPending   UserStatus = "pending"
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
)

// Valid returns true if the UserStatus is recognized.
func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusPending, UserStatusActive, UserStatusInactive, UserStatusSuspended:
		return true
	}
	return false
}

// CanTransitionTo validates allowed status transitions.
// This encapsulates the business rules for user status state machine.
func (s UserStatus) CanTransitionTo(target UserStatus) bool {
	allowed := map[UserStatus][]UserStatus{
		UserStatusPending:   {UserStatusActive, UserStatusInactive},
		UserStatusActive:    {UserStatusInactive, UserStatusSuspended},
		UserStatusInactive:  {UserStatusActive, UserStatusSuspended},
		UserStatusSuspended: {UserStatusActive, UserStatusInactive},
	}
	return slices.Contains(allowed[s], target)
}

// User is the core domain entity representing a user account.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string // Never expose this externally; set via SetPassword
	Phone        *string
	Username     string
	FullName     string
	Type         UserType
	Status       UserStatus
	UserType     UserType

	EmailVerified bool
	PhoneVerified bool

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	// Version for optimistic locking
	Version int

	// Roles assigned to this user (loaded separately)
	Roles []Role
}

func NewUser(email, username, fullName string, userType UserType) (*User, error) {
	u := &User{
		ID:        uuid.New(),
		Email:     strings.ToLower(strings.TrimSpace(email)),
		Username:  strings.TrimSpace(username),
		FullName:  strings.TrimSpace(fullName),
		Type:      userType,
		Status:    UserStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Version:   1,
	}

	if err := u.Validate(); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *User) Validate() error {
	var errs ValidationErrors

	// Email validation
	if u.Email == "" {
		errs = append(errs, ValidationError{Field: "email", Message: "required"})
	} else if _, err := mail.ParseAddress(u.Email); err != nil {
		errs = append(errs, ValidationError{Field: "email", Message: "invalid format"})
	}

	// Username validation
	if u.Username == "" {
		errs = append(errs, ValidationError{Field: "username", Message: "required"})
	} else if len(u.Username) < 3 || len(u.Username) > 50 {
		errs = append(errs, ValidationError{Field: "username", Message: "must be 3-50 characters"})
	} else if !isValidUsername(u.Username) {
		errs = append(errs, ValidationError{Field: "username", Message: "can only contain letters, numbers, underscores, and hyphens"})
	}

	// Full name validation
	if u.FullName == "" {
		errs = append(errs, ValidationError{Field: "full_name", Message: "required"})
	} else if len(u.FullName) > 200 {
		errs = append(errs, ValidationError{Field: "full_name", Message: "must be at most 200 characters"})
	}

	// Type validation
	if !u.Type.Valid() {
		errs = append(errs, ValidationError{Field: "type", Message: "invalid user type"})
	}

	// Status validation
	if !u.Status.Valid() {
		errs = append(errs, ValidationError{Field: "status", Message: "invalid status"})
	}

	// Phone validation (if provided)
	if u.Phone != nil && *u.Phone != "" {
		if !isValidPhone(*u.Phone) {
			errs = append(errs, ValidationError{Field: "phone", Message: "invalid phone format"})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (u *User) SetPhone(phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		u.Phone = nil
		u.PhoneVerified = false
		return nil
	}
	if !isValidPhone(phone) {
		return ValidationError{Field: "phone", Message: "invalid phone format"}
	}
	u.Phone = &phone
	u.PhoneVerified = false
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) ChangeStatus(newStatus UserStatus) error {
	if !newStatus.Valid() {
		return ValidationError{Field: "status", Message: "invalid status"}
	}
	if !u.Status.CanTransitionTo(newStatus) {
		return ValidationError{
			Field:   "status",
			Message: "cannot transition from " + string(u.Status) + " to " + string(newStatus),
		}
	}
	u.Status = newStatus
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) Activate() error {
	if u.Status == UserStatusActive {
		return nil // Already active, idempotent
	}
	return u.ChangeStatus(UserStatusActive)
}

func (u *User) Suspend() error {
	if u.Status == UserStatusSuspended {
		return nil // Already suspended, idempotent
	}
	return u.ChangeStatus(UserStatusSuspended)
}

func (u *User) VerifyEmail() {
	u.EmailVerified = true
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) VerifyPhone() {
	u.PhoneVerified = true
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) IsActive() bool {
	return u.Status == UserStatusActive && u.DeletedAt == nil
}

func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

func (u *User) Delete() {
	now := time.Now().UTC()
	u.DeletedAt = &now
	u.UpdatedAt = now
}

func (u *User) HasRole(roleName string) bool {
	for _, r := range u.Roles {
		if r.Name == roleName {
			return true
		}
	}
	return false
}

func (u *User) HasPermission(resource, action string) bool {
	for _, role := range u.Roles {
		if role.HasPermission(resource, action) {
			return true
		}
	}
	return false
}

func (u *User) AllPermissions() []Permission {
	seen := make(map[string]bool)
	var perms []Permission
	for _, role := range u.Roles {
		for _, p := range role.Permissions {
			key := p.Resource + ":" + p.Action
			if !seen[key] {
				seen[key] = true
				perms = append(perms, p)
			}
		}
	}
	return perms
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isValidUsername(s string) bool {
	return usernameRegex.MatchString(s)
}

var phoneRegex = regexp.MustCompile(`^\+?[\d\s\-()]+$`)

func isValidPhone(s string) bool {
	// At least 7 digits
	digitCount := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digitCount++
		}
	}
	return phoneRegex.MatchString(s) && digitCount >= 7
}
