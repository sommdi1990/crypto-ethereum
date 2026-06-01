package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Side is buy or sell.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// Type is order type.
type Type string

const (
	TypeLimit Type = "LIMIT"
)

// Status is order lifecycle status.
type Status string

const (
	StatusNew             Status = "NEW"
	StatusPartiallyFilled Status = "PARTIALLY_FILLED"
	StatusFilled          Status = "FILLED"
	StatusCancelled       Status = "CANCELLED"
	StatusRejected        Status = "REJECTED"
)

// Order is a limit order on a market.
type Order struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	MarketID       uuid.UUID
	MarketSymbol   string
	Side           Side
	OrderType      Type
	Price          decimal.Decimal
	Quantity       decimal.Decimal
	FilledQty      decimal.Decimal
	Status         Status
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int
}

// RemainingQty returns unfilled quantity.
func (o Order) RemainingQty() decimal.Decimal {
	return o.Quantity.Sub(o.FilledQty)
}

// IsOpen returns true if order can still be matched or cancelled.
func (o Order) IsOpen() bool {
	return o.Status == StatusNew || o.Status == StatusPartiallyFilled
}
