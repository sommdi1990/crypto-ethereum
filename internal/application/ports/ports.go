package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domaindeposit "github.com/soroush/eth-exchange-lab/internal/domain/deposit"
	domainledger "github.com/soroush/eth-exchange-lab/internal/domain/ledger"
	domainmarket "github.com/soroush/eth-exchange-lab/internal/domain/market"
	domainorder "github.com/soroush/eth-exchange-lab/internal/domain/order"
	domaintrade "github.com/soroush/eth-exchange-lab/internal/domain/trade"
	domainuser "github.com/soroush/eth-exchange-lab/internal/domain/user"
	domainwallet "github.com/soroush/eth-exchange-lab/internal/domain/wallet"
	domainwithdrawal "github.com/soroush/eth-exchange-lab/internal/domain/withdrawal"
)

// UnitOfWork manages database transactions.
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// UserRepository persists users.
type UserRepository interface {
	Create(ctx context.Context, email string) (*domainuser.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domainuser.User, error)
}

// WalletRepository persists wallets and addresses.
type WalletRepository interface {
	CreateWallet(ctx context.Context, userID uuid.UUID, label, address string, chainID int64) (*domainwallet.Wallet, *domainwallet.Address, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domainwallet.Wallet, []domainwallet.Address, error)
	FindAddress(ctx context.Context, chainID int64, address string) (*domainwallet.Address, *uuid.UUID, error)
}

// MarketRepository loads markets.
type MarketRepository interface {
	GetBySymbol(ctx context.Context, symbol string) (*domainmarket.Market, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domainmarket.Market, error)
}

// LedgerRepository handles double-entry accounting.
type LedgerRepository interface {
	EnsureAccount(ctx context.Context, code string, accountType domainledger.AccountType, userID *uuid.UUID, assetID uuid.UUID) (uuid.UUID, error)
	PostJournal(ctx context.Context, journal domainledger.Journal, lines []domainledger.Line) (*domainledger.Journal, error)
	GetBalances(ctx context.Context, userID uuid.UUID) (map[string]map[string]decimal.Decimal, error)
	ListJournals(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domainledger.Journal, error)
}

// IdempotencyStore prevents duplicate financial commands.
type IdempotencyStore interface {
	Begin(ctx context.Context, key, scope, requestHash string) (alreadyDone bool, response []byte, err error)
	Complete(ctx context.Context, key string, response []byte) error
}

// OrderRepository persists orders.
type OrderRepository interface {
	Create(ctx context.Context, o domainorder.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*domainorder.Order, error)
	Update(ctx context.Context, o domainorder.Order) error
	ListOpenByMarket(ctx context.Context, marketID uuid.UUID) ([]domainorder.Order, error)
}

// TradeRepository persists trades.
type TradeRepository interface {
	Create(ctx context.Context, t domaintrade.Trade) error
	ListByMarket(ctx context.Context, marketSymbol string, limit int) ([]domaintrade.Trade, error)
}

// DepositRepository persists deposits.
type DepositRepository interface {
	Create(ctx context.Context, d domaindeposit.Deposit) error
	Update(ctx context.Context, d domaindeposit.Deposit) error
	GetByID(ctx context.Context, id uuid.UUID) (*domaindeposit.Deposit, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domaindeposit.Deposit, error)
	FindByDedupKey(ctx context.Context, chainID int64, txHash string, logIndex int) (*domaindeposit.Deposit, error)
	ListPendingConfirmation(ctx context.Context) ([]domaindeposit.Deposit, error)
}

// WithdrawalRepository persists withdrawals.
type WithdrawalRepository interface {
	Create(ctx context.Context, w domainwithdrawal.Withdrawal) error
	Update(ctx context.Context, w domainwithdrawal.Withdrawal) error
	GetByID(ctx context.Context, id uuid.UUID) (*domainwithdrawal.Withdrawal, error)
}

// BlockchainRepository tracks blocks and events.
type BlockchainRepository interface {
	SaveBlock(ctx context.Context, chainID, blockNumber int64, blockHash, parentHash string) error
	GetBlock(ctx context.Context, chainID, blockNumber int64) (blockHash, parentHash string, found bool, err error)
	SaveEvent(ctx context.Context, chainID int64, blockNumber int64, blockHash, txHash string, logIndex int, contractAddress, eventName string, payload map[string]interface{}) (inserted bool, err error)
	OrphanEventsFromBlock(ctx context.Context, chainID, fromBlock int64) error
	GetScannerState(ctx context.Context, chainID int64) (lastBlock int64, err error)
	SetScannerState(ctx context.Context, chainID, lastBlock int64) error
}

// OutboxRepository stores domain events for reliable publishing.
type OutboxRepository interface {
	Insert(ctx context.Context, aggregateType string, aggregateID uuid.UUID, eventType string, payload map[string]interface{}) error
	FetchUnpublished(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
}

// OutboxEvent is a pending outbox row.
type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       map[string]interface{}
}

// AuditRepository records sensitive actions.
type AuditRepository interface {
	Log(ctx context.Context, actorType, actorID, action, resourceType, resourceID string, metadata map[string]interface{}) error
}

// AssetRepository loads assets.
type AssetRepository interface {
	GetBySymbol(ctx context.Context, symbol string) (uuid.UUID, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (symbol string, decimals int, err error)
	UpdateContractAddress(ctx context.Context, symbol, address string) error
}

// EventPublisher publishes domain events to Kafka.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, payload []byte) error
}

// EthereumClient reads chain state and sends transactions.
type EthereumClient interface {
	ChainID(ctx context.Context) (int64, error)
	LatestBlockNumber(ctx context.Context) (uint64, error)
	BlockByNumber(ctx context.Context, number uint64) (hash, parentHash string, err error)
	Ping(ctx context.Context) error
}

// TxSender broadcasts signed transactions (local/testnet only).
type TxSender interface {
	SendVaultWithdrawETH(ctx context.Context, to string, amountWei string, withdrawalID [32]byte) (txHash string, err error)
	SendVaultWithdrawERC20(ctx context.Context, token, to string, amount string, withdrawalID [32]byte) (txHash string, err error)
}

// AddressGenerator generates deposit addresses for local mode.
type AddressGenerator interface {
	GenerateAddress(ctx context.Context, userID uuid.UUID) (string, error)
}
