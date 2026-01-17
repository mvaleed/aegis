package domain

import (
	"github.com/google/uuid"
)

func UUIDFromString(uuidStr string) uuid.UUID {
	id, _ := uuid.Parse(uuidStr)
	return id
}
