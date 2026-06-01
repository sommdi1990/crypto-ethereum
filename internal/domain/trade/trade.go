package trade

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Trade represents an executed match between two orders.
type Trade struct {
	ID          uuid.UUID
	MarketID    uuid.UUID
	BuyOrderID  uuid.UUID
	SellOrderID uuid.UUID
	Price       decimal.Decimal
	Quantity    decimal.Decimal
	BuyerID     uuid.UUID
	SellerID    uuid.UUID
	Fee         decimal.Decimal
	CreatedAt   time.Time
}
