package metrics

import "sync/atomic"

// Registry holds simple counters for observability placeholders.
type Registry struct {
	HTTPRequests   atomic.Int64
	TradesExecuted atomic.Int64
	DepositsCredited atomic.Int64
	WithdrawalsBroadcast atomic.Int64
}

// NewRegistry creates a metrics registry.
func NewRegistry() *Registry {
	return &Registry{}
}
