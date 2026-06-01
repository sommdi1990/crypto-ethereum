package market

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Market defines a trading pair.
type Market struct {
	ID           uuid.UUID
	Symbol       string
	BaseAssetID  uuid.UUID
	QuoteAssetID uuid.UUID
	MinQty       decimal.Decimal
	TickSize     decimal.Decimal
	Status       string
}
