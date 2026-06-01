package ordersvc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/soroush/eth-exchange-lab/internal/application/ports"
	ledgersvc "github.com/soroush/eth-exchange-lab/internal/application/ledger"
	domainmatching "github.com/soroush/eth-exchange-lab/internal/domain/matching"
	domainorder "github.com/soroush/eth-exchange-lab/internal/domain/order"
)

// Service handles order placement and cancellation.
type Service struct {
	uow      ports.UnitOfWork
	orders   ports.OrderRepository
	trades   ports.TradeRepository
	markets  ports.MarketRepository
	ledger   *ledgersvc.Service
	outbox   ports.OutboxRepository
	engine   *domainmatching.Engine
	feeRate  decimal.Decimal
}

// New creates an order service.
func New(uow ports.UnitOfWork, orders ports.OrderRepository, trades ports.TradeRepository, markets ports.MarketRepository, ledger *ledgersvc.Service, outbox ports.OutboxRepository, engine *domainmatching.Engine, feeRate decimal.Decimal) *Service {
	return &Service{uow: uow, orders: orders, trades: trades, markets: markets, ledger: ledger, outbox: outbox, engine: engine, feeRate: feeRate}
}

// PlaceOrderRequest is input for order placement.
type PlaceOrderRequest struct {
	UserID         uuid.UUID
	Market         string
	Side           domainorder.Side
	Price          decimal.Decimal
	Quantity       decimal.Decimal
	IdempotencyKey string
}

// PlaceOrder locks balance, persists order, and runs matching.
func (s *Service) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*domainorder.Order, []domainorder.Order, error) {
	market, err := s.markets.GetBySymbol(ctx, req.Market)
	if err != nil {
		return nil, nil, fmt.Errorf("market: %w", err)
	}

	lockSymbol := "USDT"
	lockAmount := req.Price.Mul(req.Quantity)
	if req.Side == domainorder.SideSell {
		lockSymbol = "ETH"
		lockAmount = req.Quantity
	}

	orderID := uuid.New()
	if err := s.ledger.LockForOrder(ctx, req.UserID, lockSymbol, lockAmount, orderID, "order-lock:"+req.IdempotencyKey); err != nil {
		return nil, nil, err
	}

	order := domainorder.Order{
		ID:             orderID,
		UserID:         req.UserID,
		MarketID:       market.ID,
		MarketSymbol:   market.Symbol,
		Side:           req.Side,
		OrderType:      domainorder.TypeLimit,
		Price:          req.Price,
		Quantity:       req.Quantity,
		Status:         domainorder.StatusNew,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.orders.Create(ctx, order); err != nil {
		return nil, nil, err
	}

	matchResult := s.engine.SubmitOrder(order)
	for _, t := range matchResult.Trades {
		if err := s.trades.Create(ctx, t); err != nil {
			return nil, nil, err
		}
		buyer := t.BuyerID
		seller := t.SellerID
		if err := s.ledger.SettleTrade(ctx, buyer, seller, "ETH", "USDT", t.Price, t.Quantity, t.Fee, t.ID, "trade:"+t.ID.String()); err != nil {
			return nil, nil, err
		}
		_ = s.outbox.Insert(ctx, "trade", t.ID, "exchange.trade.executed", map[string]interface{}{
			"trade_id": t.ID.String(), "price": t.Price.String(), "quantity": t.Quantity.String(),
		})
	}
	for _, updated := range matchResult.UpdatedOrders {
		if err := s.orders.Update(ctx, updated); err != nil {
			return nil, nil, err
		}
	}
	_ = s.outbox.Insert(ctx, "order", order.ID, "exchange.order.placed", map[string]interface{}{
		"order_id": order.ID.String(), "market": req.Market,
	})

	final, err := s.orders.GetByID(ctx, order.ID)
	return final, matchResult.UpdatedOrders, err
}

// CancelOrder cancels an open order and unlocks balance.
func (s *Service) CancelOrder(ctx context.Context, orderID uuid.UUID) (*domainorder.Order, error) {
	o, err := s.orders.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !o.IsOpen() {
		return nil, fmt.Errorf("order not cancellable")
	}
	if err := s.engine.CancelOrder(orderID); err != nil {
		// order may not be in book if partially filled resting
	}
	remaining := o.RemainingQty()
	lockSymbol := "USDT"
	lockAmount := o.Price.Mul(remaining)
	if o.Side == domainorder.SideSell {
		lockSymbol = "ETH"
		lockAmount = remaining
	}
	if err := s.ledger.UnlockOrder(ctx, o.UserID, lockSymbol, lockAmount, orderID, "order-cancel:"+orderID.String()); err != nil {
		return nil, err
	}
	o.Status = domainorder.StatusCancelled
	o.UpdatedAt = time.Now().UTC()
	if err := s.orders.Update(ctx, *o); err != nil {
		return nil, err
	}
	_ = s.outbox.Insert(ctx, "order", orderID, "exchange.order.cancelled", map[string]interface{}{"order_id": orderID.String()})
	return o, nil
}

// GetOrder returns order by ID.
func (s *Service) GetOrder(ctx context.Context, id uuid.UUID) (*domainorder.Order, error) {
	return s.orders.GetByID(ctx, id)
}

// ListTrades returns recent trades for a market.
func (s *Service) ListTrades(ctx context.Context, market string, limit int) ([]interface{}, error) {
	trades, err := s.trades.ListByMarket(ctx, market, limit)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, len(trades))
	for i, t := range trades {
		result[i] = t
	}
	return result, nil
}

// OrderbookSnapshot returns current book.
func (s *Service) OrderbookSnapshot() (bids, asks [][2]string) {
	return s.engine.Snapshot()
}

// RebuildEngine loads open orders into matching engine on startup.
func (s *Service) RebuildEngine(ctx context.Context, marketID uuid.UUID) error {
	orders, err := s.orders.ListOpenByMarket(ctx, marketID)
	if err != nil {
		return err
	}
	for _, o := range orders {
		s.engine.SubmitOrder(o)
	}
	return nil
}
