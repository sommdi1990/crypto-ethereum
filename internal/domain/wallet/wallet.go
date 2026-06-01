package wallet

import (
	"time"

	"github.com/google/uuid"
)

// Wallet groups deposit addresses for a user.
type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Label     string
	CreatedAt time.Time
}

// Address is an on-chain deposit address assigned to a wallet.
type Address struct {
	ID              uuid.UUID
	WalletID        uuid.UUID
	ChainID         int64
	Address         string
	DerivationPath  *string
	IsActive        bool
	CreatedAt       time.Time
}
