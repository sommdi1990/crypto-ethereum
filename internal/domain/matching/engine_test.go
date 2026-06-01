package matching

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	domainorder "github.com/soroush/eth-exchange-lab/internal/domain/order"
)

func TestEngine_PriceTimePriority(t *testing.T) {
	marketID := uuid.New()
	engine := NewEngine(marketID, decimal.RequireFromString("0.001"))

	sell1 := newLimitOrder(domainorder.SideSell, "100", "1")
	sell2 := newLimitOrder(domainorder.SideSell, "100", "1")
	buy := newLimitOrder(domainorder.SideBuy, "100", "1.5")

	engine.SubmitOrder(sell1)
	engine.SubmitOrder(sell2)
	result := engine.SubmitOrder(buy)

	require.Len(t, result.Trades, 2)
	require.True(t, result.Trades[0].Quantity.Equal(decimal.RequireFromString("1")))
	require.True(t, result.Trades[1].Quantity.Equal(decimal.RequireFromString("0.5")))
}

func TestEngine_PartialFill(t *testing.T) {
	marketID := uuid.New()
	engine := NewEngine(marketID, decimal.Zero)

	sell := newLimitOrder(domainorder.SideSell, "50", "2")
	buy := newLimitOrder(domainorder.SideBuy, "50", "1")
	engine.SubmitOrder(sell)
	result := engine.SubmitOrder(buy)

	require.Len(t, result.Trades, 1)
	require.Equal(t, domainorder.StatusPartiallyFilled, result.UpdatedOrders[len(result.UpdatedOrders)-1].Status)
}

func newLimitOrder(side domainorder.Side, price, qty string) domainorder.Order {
	return domainorder.Order{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		MarketID:  uuid.New(),
		Side:      side,
		OrderType: domainorder.TypeLimit,
		Price:     decimal.RequireFromString(price),
		Quantity:  decimal.RequireFromString(qty),
		Status:    domainorder.StatusNew,
	}
}
