package matching

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domainorder "github.com/soroush/eth-exchange-lab/internal/domain/order"
	domaintrade "github.com/soroush/eth-exchange-lab/internal/domain/trade"
)

// MatchResult contains trades and updated orders from a match attempt.
type MatchResult struct {
	Trades        []domaintrade.Trade
	UpdatedOrders []domainorder.Order
}

// Engine is an in-memory price-time priority matching engine.
type Engine struct {
	mu       sync.RWMutex
	marketID uuid.UUID
	feeRate  decimal.Decimal
	bids     orderHeap
	asks     orderHeap
	seq      int64
}

// NewEngine creates a matching engine for a market.
func NewEngine(marketID uuid.UUID, feeRate decimal.Decimal) *Engine {
	e := &Engine{marketID: marketID, feeRate: feeRate}
	heap.Init(&e.bids)
	heap.Init(&e.asks)
	return e
}

// SubmitOrder adds an order and attempts to match it.
func (e *Engine) SubmitOrder(o domainorder.Order) MatchResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	var result MatchResult
	incoming := o

	for incoming.RemainingQty().IsPositive() && incoming.IsOpen() {
		var contra *bookOrder
		if incoming.Side == domainorder.SideBuy {
			if e.asks.Len() == 0 {
				break
			}
			best := e.asks[0]
			if incoming.OrderType == domainorder.TypeLimit && incoming.Price.LessThan(best.price) {
				break
			}
			contra = heap.Pop(&e.asks).(*bookOrder)
		} else {
			if e.bids.Len() == 0 {
				break
			}
			best := e.bids[0]
			if incoming.OrderType == domainorder.TypeLimit && incoming.Price.GreaterThan(best.price) {
				break
			}
			contra = heap.Pop(&e.bids).(*bookOrder)
		}

		fillQty := decimal.Min(incoming.RemainingQty(), contra.remaining())
		tradePrice := contra.price
		if incoming.OrderType == domainorder.TypeLimit {
			tradePrice = incoming.Price
			if incoming.Side == domainorder.SideBuy && contra.price.LessThan(incoming.Price) {
				tradePrice = contra.price
			}
			if incoming.Side == domainorder.SideSell && contra.price.GreaterThan(incoming.Price) {
				tradePrice = contra.price
			}
		}

		var buyerID, sellerID uuid.UUID
		var buyOrderID, sellOrderID uuid.UUID
		if incoming.Side == domainorder.SideBuy {
			buyerID, sellerID = incoming.UserID, contra.userID
			buyOrderID, sellOrderID = incoming.ID, contra.id
		} else {
			buyerID, sellerID = contra.userID, incoming.UserID
			buyOrderID, sellOrderID = contra.id, incoming.ID
		}

		notional := tradePrice.Mul(fillQty)
		fee := notional.Mul(e.feeRate)

		result.Trades = append(result.Trades, domaintrade.Trade{
			ID:          uuid.New(),
			MarketID:    e.marketID,
			BuyOrderID:  buyOrderID,
			SellOrderID: sellOrderID,
			Price:       tradePrice,
			Quantity:    fillQty,
			BuyerID:     buyerID,
			SellerID:    sellerID,
			Fee:         fee,
			CreatedAt:   time.Now().UTC(),
		})

		incoming.FilledQty = incoming.FilledQty.Add(fillQty)
		contra.filled = contra.filled.Add(fillQty)

		if contra.remaining().IsPositive() {
			contra.order.FilledQty = contra.filled
			contra.order.Status = domainorder.StatusPartiallyFilled
			result.UpdatedOrders = append(result.UpdatedOrders, contra.order)
			e.push(contra)
		} else {
			contra.order.Status = domainorder.StatusFilled
			result.UpdatedOrders = append(result.UpdatedOrders, contra.order)
		}
	}

	incoming.UpdatedAt = time.Now().UTC()
	if incoming.RemainingQty().IsZero() {
		incoming.Status = domainorder.StatusFilled
	} else if incoming.FilledQty.IsPositive() {
		incoming.Status = domainorder.StatusPartiallyFilled
		e.push(newBookOrder(incoming, e.nextSeq()))
	} else {
		incoming.Status = domainorder.StatusNew
		e.push(newBookOrder(incoming, e.nextSeq()))
	}
	result.UpdatedOrders = append(result.UpdatedOrders, incoming)
	return result
}

// CancelOrder removes an order from the book by ID.
func (e *Engine) CancelOrder(orderID uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.removeFromHeap(&e.bids, orderID) || e.removeFromHeap(&e.asks, orderID) {
		return nil
	}
	return errors.New("order not in book")
}

// Snapshot returns current order book levels.
func (e *Engine) Snapshot() (bids, asks [][2]string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, o := range e.bids {
		bids = append(bids, [2]string{o.price.String(), o.remaining().String()})
	}
	for _, o := range e.asks {
		asks = append(asks, [2]string{o.price.String(), o.remaining().String()})
	}
	return bids, asks
}

func (e *Engine) push(o *bookOrder) {
	if o.side == domainorder.SideBuy {
		heap.Push(&e.bids, o)
	} else {
		heap.Push(&e.asks, o)
	}
}

func (e *Engine) nextSeq() int64 {
	e.seq++
	return e.seq
}

func (e *Engine) removeFromHeap(h *orderHeap, orderID uuid.UUID) bool {
	for i, o := range *h {
		if o.id == orderID {
			heap.Remove(h, i)
			return true
		}
	}
	return false
}

type bookOrder struct {
	id      uuid.UUID
	userID  uuid.UUID
	side    domainorder.Side
	price   decimal.Decimal
	filled  decimal.Decimal
	qty     decimal.Decimal
	seq     int64
	order   domainorder.Order
}

func newBookOrder(o domainorder.Order, seq int64) *bookOrder {
	return &bookOrder{
		id: o.ID, userID: o.UserID, side: o.Side, price: o.Price,
		filled: o.FilledQty, qty: o.Quantity, seq: seq, order: o,
	}
}

func (o *bookOrder) remaining() decimal.Decimal {
	return o.qty.Sub(o.filled)
}

type orderHeap []*bookOrder

func (h orderHeap) Len() int { return len(h) }

func (h orderHeap) Less(i, j int) bool {
	a, b := h[i], h[j]
	if a.side == domainorder.SideBuy {
		if a.price.Equal(b.price) {
			return a.seq < b.seq
		}
		return a.price.GreaterThan(b.price)
	}
	if a.price.Equal(b.price) {
		return a.seq < b.seq
	}
	return a.price.LessThan(b.price)
}

func (h orderHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *orderHeap) Push(x interface{}) {
	*h = append(*h, x.(*bookOrder))
}

func (h *orderHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
