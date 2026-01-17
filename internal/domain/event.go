package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event represents a domain event that occurred.
// Events are immutable facts about something that happened.
type Event struct {
	ID        uuid.UUID
	Type      string
	Timestamp time.Time
	UserID    uuid.UUID
	Data      map[string]any
}

// Event type constants
const (
	EventUserCreated       = "user.created"
	EventUserUpdated       = "user.updated"
	EventUserDeleted       = "user.deleted"
	EventUserActivated     = "user.activated"
	EventUserSuspended     = "user.suspended"
	EventUserDeactivated   = "user.deactivated"
	EventUserEmailVerified = "user.email_verified"
	EventUserPhoneVerified = "user.phone_verified"
	EventUserLoggedIn      = "user.logged_in"
	EventUserLoggedOut     = "user.logged_out"
	EventUserRoleAssigned  = "user.role_assigned"
	EventUserRoleRemoved   = "user.role_removed"
	EventPasswordChanged   = "user.password_changed"
	EventPasswordReset     = "user.password_reset"
)

// NewEvent creates a new domain event.
func NewEvent(eventType string, userID uuid.UUID, data map[string]any) Event {
	if data == nil {
		data = make(map[string]any)
	}
	return Event{
		ID:        uuid.New(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		UserID:    userID,
		Data:      data,
	}
}

func UserCreatedEvent(u *User) Event {
	return NewEvent(EventUserCreated, u.ID, map[string]any{
		"email":     u.Email,
		"username":  u.Username,
		"user_type": string(u.Type),
	})
}

func UserActivatedEvent(u *User) Event {
	return NewEvent(EventUserActivated, u.ID, map[string]any{
		"email":    u.Email,
		"username": u.Username,
	})
}

func UserSuspendedEvent(u *User, reason string) Event {
	return NewEvent(EventUserSuspended, u.ID, map[string]any{
		"email":    u.Email,
		"username": u.Username,
		"reason":   reason,
	})
}

func UserDeletedEvent(userID uuid.UUID) Event {
	return NewEvent(EventUserDeleted, userID, nil)
}

func UserLoggedInEvent(userID uuid.UUID, ipAddress, userAgent string) Event {
	return NewEvent(EventUserLoggedIn, userID, map[string]any{
		"ip_address": ipAddress,
		"user_agent": userAgent,
	})
}

func RoleAssignedEvent(userID uuid.UUID, roleName string) Event {
	return NewEvent(EventUserRoleAssigned, userID, map[string]any{
		"role": roleName,
	})
}

func RoleRemovedEvent(userID uuid.UUID, roleName string) Event {
	return NewEvent(EventUserRoleRemoved, userID, map[string]any{
		"role": roleName,
	})
}
