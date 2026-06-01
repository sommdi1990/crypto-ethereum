package withdrawal

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status is withdrawal lifecycle status.
type Status string

const (
	StatusPendingApproval Status = "PENDING_APPROVAL"
	StatusApproved        Status = "APPROVED"
	StatusSigning         Status = "SIGNING"
	StatusBroadcasted     Status = "BROADCASTED"
	StatusConfirmed       Status = "CONFIRMED"
	StatusFailed          Status = "FAILED"
	StatusCancelled       Status = "CANCELLED"
)

// Withdrawal represents a user withdrawal request.
type Withdrawal struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	AssetID             uuid.UUID
	AssetSymbol         string
	Amount              decimal.Decimal
	DestinationAddress  string
	Status              Status
	IdempotencyKey      string
	ApprovalStatus      string
	TxHash              *string
	Nonce               *int64
	FailureReason       *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int
}
