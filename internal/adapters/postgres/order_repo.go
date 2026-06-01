package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainmarket "github.com/soroush/eth-exchange-lab/internal/domain/market"
	domainorder "github.com/soroush/eth-exchange-lab/internal/domain/order"
	domaintrade "github.com/soroush/eth-exchange-lab/internal/domain/trade"
)

// MarketRepo loads markets.
type MarketRepo struct{ db *DB }

func NewMarketRepo(db *DB) *MarketRepo { return &MarketRepo{db: db} }

func (r *MarketRepo) GetBySymbol(ctx context.Context, symbol string) (*domainmarket.Market, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, symbol, base_asset_id, quote_asset_id, min_qty, tick_size, status
		FROM markets WHERE symbol = $1`, symbol)
	var m domainmarket.Market
	if err := row.Scan(&m.ID, &m.Symbol, &m.BaseAssetID, &m.QuoteAssetID, &m.MinQty, &m.TickSize, &m.Status); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MarketRepo) GetByID(ctx context.Context, id uuid.UUID) (*domainmarket.Market, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, symbol, base_asset_id, quote_asset_id, min_qty, tick_size, status
		FROM markets WHERE id = $1`, id)
	var m domainmarket.Market
	if err := row.Scan(&m.ID, &m.Symbol, &m.BaseAssetID, &m.QuoteAssetID, &m.MinQty, &m.TickSize, &m.Status); err != nil {
		return nil, err
	}
	return &m, nil
}

// OrderRepo persists orders.
type OrderRepo struct{ db *DB }

func NewOrderRepo(db *DB) *OrderRepo { return &OrderRepo{db: db} }

func (r *OrderRepo) Create(ctx context.Context, o domainorder.Order) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		INSERT INTO orders (id, user_id, market_id, side, order_type, price, quantity, filled_qty, status, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		o.ID, o.UserID, o.MarketID, string(o.Side), string(o.OrderType),
		o.Price.String(), o.Quantity.String(), o.FilledQty.String(), string(o.Status), o.IdempotencyKey)
	return err
}

func (r *OrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*domainorder.Order, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT o.id, o.user_id, o.market_id, m.symbol, o.side, o.order_type, o.price, o.quantity, o.filled_qty, o.status, o.idempotency_key, o.created_at, o.updated_at, o.version
		FROM orders o JOIN markets m ON m.id = o.market_id WHERE o.id = $1`, id)
	var o domainorder.Order
	if err := row.Scan(&o.ID, &o.UserID, &o.MarketID, &o.MarketSymbol, &o.Side, &o.OrderType,
		&o.Price, &o.Quantity, &o.FilledQty, &o.Status, &o.IdempotencyKey, &o.CreatedAt, &o.UpdatedAt, &o.Version); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepo) Update(ctx context.Context, o domainorder.Order) error {
	q := exec(ctx, r.db)
	tag, err := q.Exec(ctx, `
		UPDATE orders SET filled_qty = $2, status = $3, updated_at = NOW(), version = version + 1
		WHERE id = $1 AND version = $4`,
		o.ID, o.FilledQty.String(), string(o.Status), o.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("optimistic lock conflict")
	}
	return nil
}

func (r *OrderRepo) ListOpenByMarket(ctx context.Context, marketID uuid.UUID) ([]domainorder.Order, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT o.id, o.user_id, o.market_id, m.symbol, o.side, o.order_type, o.price, o.quantity, o.filled_qty, o.status, o.idempotency_key, o.created_at, o.updated_at, o.version
		FROM orders o JOIN markets m ON m.id = o.market_id
		WHERE o.market_id = $1 AND o.status IN ('NEW', 'PARTIALLY_FILLED')
		ORDER BY o.created_at`, marketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []domainorder.Order
	for rows.Next() {
		var o domainorder.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.MarketID, &o.MarketSymbol, &o.Side, &o.OrderType,
			&o.Price, &o.Quantity, &o.FilledQty, &o.Status, &o.IdempotencyKey, &o.CreatedAt, &o.UpdatedAt, &o.Version); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// TradeRepo persists trades.
type TradeRepo struct{ db *DB }

func NewTradeRepo(db *DB) *TradeRepo { return &TradeRepo{db: db} }

func (r *TradeRepo) Create(ctx context.Context, t domaintrade.Trade) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		INSERT INTO trades (id, market_id, buy_order_id, sell_order_id, price, quantity, buyer_id, seller_id, fee)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		t.ID, t.MarketID, t.BuyOrderID, t.SellOrderID, t.Price.String(), t.Quantity.String(), t.BuyerID, t.SellerID, t.Fee.String())
	return err
}

func (r *TradeRepo) ListByMarket(ctx context.Context, marketSymbol string, limit int) ([]domaintrade.Trade, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT t.id, t.market_id, t.buy_order_id, t.sell_order_id, t.price, t.quantity, t.buyer_id, t.seller_id, t.fee, t.created_at
		FROM trades t JOIN markets m ON m.id = t.market_id
		WHERE m.symbol = $1 ORDER BY t.created_at DESC LIMIT $2`, marketSymbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trades []domaintrade.Trade
	for rows.Next() {
		var t domaintrade.Trade
		if err := rows.Scan(&t.ID, &t.MarketID, &t.BuyOrderID, &t.SellOrderID, &t.Price, &t.Quantity, &t.BuyerID, &t.SellerID, &t.Fee, &t.CreatedAt); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}
