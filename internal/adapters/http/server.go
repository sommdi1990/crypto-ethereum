package httpadapter

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/soroush/eth-exchange-lab/internal/adapters/http/handlers"
)

// ReadinessChecker checks dependency health.
type ReadinessChecker interface {
	PingPostgres(ctx context.Context) error
	PingRedis(ctx context.Context) error
	PingRPC(ctx context.Context) error
}

// Server is the HTTP API server.
type Server struct {
	log     zerolog.Logger
	api     *handlers.API
	checker ReadinessChecker
	mux     *http.ServeMux
}

// NewServer creates HTTP server with routes.
func NewServer(log zerolog.Logger, api *handlers.API, checker ReadinessChecker) *Server {
	s := &Server{log: log, api: api, checker: checker, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health/live", s.live)
	s.mux.HandleFunc("GET /health/ready", s.ready)

	s.mux.HandleFunc("POST /api/v1/users", s.api.CreateUser)
	s.mux.HandleFunc("GET /api/v1/users/{id}", s.api.GetUser)
	s.mux.HandleFunc("POST /api/v1/wallets", s.api.CreateWallet)
	s.mux.HandleFunc("GET /api/v1/wallets/{user_id}", s.api.ListWallets)
	s.mux.HandleFunc("GET /api/v1/balances/{user_id}", s.api.GetBalances)
	s.mux.HandleFunc("GET /api/v1/ledger/{user_id}", s.api.GetLedger)
	s.mux.HandleFunc("POST /api/v1/orders", s.api.PlaceOrder)
	s.mux.HandleFunc("GET /api/v1/orders/{id}", s.api.GetOrder)
	s.mux.HandleFunc("DELETE /api/v1/orders/{id}", s.api.CancelOrder)
	s.mux.HandleFunc("GET /api/v1/orderbook", s.api.GetOrderbook)
	s.mux.HandleFunc("GET /api/v1/trades", s.api.GetTrades)
	s.mux.HandleFunc("POST /api/v1/withdrawals", s.api.RequestWithdrawal)
	s.mux.HandleFunc("GET /api/v1/withdrawals/{id}", s.api.GetWithdrawal)
	s.mux.HandleFunc("GET /api/v1/deposits/{user_id}", s.api.ListDeposits)
	s.mux.HandleFunc("POST /api/v1/admin/withdrawals/{id}/approve", s.api.ApproveWithdrawal)
}

// Handler returns the root http.Handler with middleware.
func (s *Server) Handler() http.Handler {
	return loggingMiddleware(s.log, s.mux)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	ok := true
	if err := s.checker.PingPostgres(ctx); err != nil {
		checks["postgres"] = err.Error()
		ok = false
	} else {
		checks["postgres"] = "ok"
	}
	if err := s.checker.PingRedis(ctx); err != nil {
		checks["redis"] = err.Error()
		ok = false
	} else {
		checks["redis"] = "ok"
	}
	if err := s.checker.PingRPC(ctx); err != nil {
		checks["rpc"] = err.Error()
		ok = false
	} else {
		checks["rpc"] = "ok"
	}
	status := http.StatusOK
	st := "ready"
	if !ok {
		status = http.StatusServiceUnavailable
		st = "not_ready"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"status":"` + st + `"}`))
}

func loggingMiddleware(log zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info().Str("method", r.Method).Str("path", r.URL.Path).Dur("duration", time.Since(start)).Msg("request")
	})
}
