package user

import (
	"time"

	"github.com/google/uuid"
)

// Status represents user account status.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

// User is a registered exchange user (mock registration).
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
