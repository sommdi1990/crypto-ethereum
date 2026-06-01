package deposit

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status is deposit lifecycle status.
type Status string

const (
	StatusDetected   Status = "DETECTED"
	StatusConfirming Status = "CONFIRMING"
	StatusCredited   Status = "CREDITED"
	StatusOrphaned   Status = "ORPHANED"
)

// Deposit tracks an on-chain deposit awaiting confirmation and credit.
type Deposit struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	WalletAddressID       uuid.UUID
	AssetID               uuid.UUID
	Amount                decimal.Decimal
	TxHash                string
	LogIndex              int
	BlockNumber           int64
	BlockHash             string
	ConfirmationCount     int
	RequiredConfirmations int
	Status                Status
	ChainID               int64
	DepositRef            string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
