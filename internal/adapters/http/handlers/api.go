package handlers

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	ledgersvc "github.com/soroush/eth-exchange-lab/internal/application/ledger"
	depositsvc "github.com/soroush/eth-exchange-lab/internal/application/deposit"
	ordersvc "github.com/soroush/eth-exchange-lab/internal/application/order"
	withdrawalsvc "github.com/soroush/eth-exchange-lab/internal/application/withdrawal"
	"github.com/soroush/eth-exchange-lab/internal/application/ports"
	domainorder "github.com/soroush/eth-exchange-lab/internal/domain/order"
	"github.com/soroush/eth-exchange-lab/pkg/httputil"
)

// API bundles HTTP handlers.
type API struct {
	Users       ports.UserRepository
	Wallets     ports.WalletRepository
	Ledger      *ledgersvc.Service
	LedgerRepo  ports.LedgerRepository
	Orders      *ordersvc.Service
	Deposits    *depositsvc.Service
	Withdrawals *withdrawalsvc.Service
	ChainID     int64
	AppEnv      string
	AddrGen     ports.AddressGenerator
}

func (a *API) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil || req.Email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "email required")
		return
	}
	u, err := a.Users.Create(r.Context(), req.Email)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, u)
}

func (a *API) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid user id")
		return
	}
	u, err := a.Users.GetByID(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, u)
}

func (a *API) CreateWallet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uuid.UUID `json:"user_id"`
		Label  string    `json:"label"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	addr, err := a.AddrGen.GenerateAddress(r.Context(), req.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	wallet, address, err := a.Wallets.CreateWallet(r.Context(), req.UserID, req.Label, addr, a.ChainID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"wallet": wallet, "address": address.Address, "chain_id": address.ChainID,
	})
}

func (a *API) ListWallets(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid user id")
		return
	}
	wallets, addresses, err := a.Wallets.ListByUserID(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"wallets": wallets, "addresses": addresses})
}

func (a *API) GetBalances(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid user id")
		return
	}
	balances, err := a.Ledger.GetBalances(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, balances)
}

func (a *API) GetLedger(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid user id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	journals, err := a.LedgerRepo.ListJournals(r.Context(), userID, limit, offset)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, journals)
}

func (a *API) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   uuid.UUID `json:"user_id"`
		Market   string    `json:"market"`
		Side     string    `json:"side"`
		Price    string    `json:"price"`
		Quantity string    `json:"quantity"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_PRICE", err.Error())
		return
	}
	qty, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_QUANTITY", err.Error())
		return
	}
	order, _, err := a.Orders.PlaceOrder(r.Context(), ordersvc.PlaceOrderRequest{
		UserID: req.UserID, Market: req.Market, Side: domainorder.Side(req.Side),
		Price: price, Quantity: qty, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "ORDER_FAILED", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, order)
}

func (a *API) GetOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id")
		return
	}
	o, err := a.Orders.GetOrder(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, o)
}

func (a *API) CancelOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id")
		return
	}
	o, err := a.Orders.CancelOrder(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "CANCEL_FAILED", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, o)
}

func (a *API) GetOrderbook(w http.ResponseWriter, r *http.Request) {
	bids, asks := a.Orders.OrderbookSnapshot()
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"bids": bids, "asks": asks})
}

func (a *API) GetTrades(w http.ResponseWriter, r *http.Request) {
	market := r.URL.Query().Get("market")
	if market == "" {
		market = "ETH-USDT"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	trades, err := a.Orders.ListTrades(r.Context(), market, limit)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, trades)
}

func (a *API) RequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  uuid.UUID `json:"user_id"`
		Asset   string    `json:"asset"`
		Amount  string    `json:"amount"`
		Address string    `json:"address"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = uuid.New().String()
	}
	wd, err := a.Withdrawals.RequestWithdrawal(r.Context(), req.UserID, req.Asset, req.Amount, req.Address, key)
	if err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "WITHDRAWAL_FAILED", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, wd)
}

func (a *API) GetWithdrawal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid withdrawal id")
		return
	}
	wd, err := a.Withdrawals.GetByID(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, wd)
}

func (a *API) ApproveWithdrawal(w http.ResponseWriter, r *http.Request) {
	if a.AppEnv != "local" {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin endpoint only in local env")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid withdrawal id")
		return
	}
	wd, err := a.Withdrawals.Approve(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "APPROVE_FAILED", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, wd)
}

func (a *API) ListDeposits(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_ID", "invalid user id")
		return
	}
	deposits, err := a.Deposits.ListByUser(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, deposits)
}
