package asset

import "github.com/google/uuid"

// Type classifies an asset.
type Type string

const (
	TypeNative Type = "NATIVE"
	TypeERC20  Type = "ERC20"
)

// Asset represents a tradable asset (ETH, USDT).
type Asset struct {
	ID              uuid.UUID
	Symbol          string
	Decimals        int
	AssetType       Type
	ContractAddress *string
	ChainID         int64
}
