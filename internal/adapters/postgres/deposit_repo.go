package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domaindeposit "github.com/soroush/eth-exchange-lab/internal/domain/deposit"
	domainwithdrawal "github.com/soroush/eth-exchange-lab/internal/domain/withdrawal"
	"github.com/soroush/eth-exchange-lab/internal/application/ports"
)

// DepositRepo persists deposits.
type DepositRepo struct{ db *DB }

func NewDepositRepo(db *DB) *DepositRepo { return &DepositRepo{db: db} }

func (r *DepositRepo) Create(ctx context.Context, d domaindeposit.Deposit) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		INSERT INTO deposits (id, user_id, wallet_address_id, asset_id, amount, tx_hash, log_index, block_number, block_hash, confirmation_count, required_confirmations, status, chain_id, deposit_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		d.ID, d.UserID, d.WalletAddressID, d.AssetID, d.Amount.String(), d.TxHash, d.LogIndex,
		d.BlockNumber, d.BlockHash, d.ConfirmationCount, d.RequiredConfirmations, string(d.Status), d.ChainID, d.DepositRef)
	return err
}

func (r *DepositRepo) Update(ctx context.Context, d domaindeposit.Deposit) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		UPDATE deposits SET confirmation_count = $2, status = $3, block_hash = $4, updated_at = NOW()
		WHERE id = $1`, d.ID, d.ConfirmationCount, string(d.Status), d.BlockHash)
	return err
}

func (r *DepositRepo) GetByID(ctx context.Context, id uuid.UUID) (*domaindeposit.Deposit, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, wallet_address_id, asset_id, amount, tx_hash, log_index, block_number, block_hash, confirmation_count, required_confirmations, status, chain_id, deposit_ref, created_at, updated_at
		FROM deposits WHERE id = $1`, id)
	return scanDeposit(row)
}

func (r *DepositRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domaindeposit.Deposit, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, wallet_address_id, asset_id, amount, tx_hash, log_index, block_number, block_hash, confirmation_count, required_confirmations, status, chain_id, deposit_ref, created_at, updated_at
		FROM deposits WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deposits []domaindeposit.Deposit
	for rows.Next() {
		d, err := scanDeposit(rows)
		if err != nil {
			return nil, err
		}
		deposits = append(deposits, *d)
	}
	return deposits, rows.Err()
}

func (r *DepositRepo) FindByDedupKey(ctx context.Context, chainID int64, txHash string, logIndex int) (*domaindeposit.Deposit, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, wallet_address_id, asset_id, amount, tx_hash, log_index, block_number, block_hash, confirmation_count, required_confirmations, status, chain_id, deposit_ref, created_at, updated_at
		FROM deposits WHERE chain_id = $1 AND tx_hash = $2 AND log_index = $3`, chainID, txHash, logIndex)
	d, err := scanDeposit(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func (r *DepositRepo) ListPendingConfirmation(ctx context.Context) ([]domaindeposit.Deposit, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, wallet_address_id, asset_id, amount, tx_hash, log_index, block_number, block_hash, confirmation_count, required_confirmations, status, chain_id, deposit_ref, created_at, updated_at
		FROM deposits WHERE status IN ('DETECTED', 'CONFIRMING')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deposits []domaindeposit.Deposit
	for rows.Next() {
		d, err := scanDeposit(rows)
		if err != nil {
			return nil, err
		}
		deposits = append(deposits, *d)
	}
	return deposits, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanDeposit(row scannable) (*domaindeposit.Deposit, error) {
	var d domaindeposit.Deposit
	if err := row.Scan(&d.ID, &d.UserID, &d.WalletAddressID, &d.AssetID, &d.Amount, &d.TxHash, &d.LogIndex,
		&d.BlockNumber, &d.BlockHash, &d.ConfirmationCount, &d.RequiredConfirmations, &d.Status, &d.ChainID, &d.DepositRef, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

// WithdrawalRepo persists withdrawals.
type WithdrawalRepo struct{ db *DB }

func NewWithdrawalRepo(db *DB) *WithdrawalRepo { return &WithdrawalRepo{db: db} }

func (r *WithdrawalRepo) Create(ctx context.Context, w domainwithdrawal.Withdrawal) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		INSERT INTO withdrawals (id, user_id, asset_id, amount, destination_address, status, idempotency_key, approval_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		w.ID, w.UserID, w.AssetID, w.Amount.String(), w.DestinationAddress, string(w.Status), w.IdempotencyKey, w.ApprovalStatus)
	return err
}

func (r *WithdrawalRepo) Update(ctx context.Context, w domainwithdrawal.Withdrawal) error {
	q := exec(ctx, r.db)
	tag, err := q.Exec(ctx, `
		UPDATE withdrawals SET status = $2, approval_status = $3, tx_hash = $4, nonce = $5, failure_reason = $6, updated_at = NOW(), version = version + 1
		WHERE id = $1 AND version = $7`,
		w.ID, string(w.Status), w.ApprovalStatus, w.TxHash, w.Nonce, w.FailureReason, w.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("optimistic lock conflict")
	}
	return nil
}

func (r *WithdrawalRepo) GetByID(ctx context.Context, id uuid.UUID) (*domainwithdrawal.Withdrawal, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT w.id, w.user_id, w.asset_id, a.symbol, w.amount, w.destination_address, w.status, w.idempotency_key, w.approval_status, w.tx_hash, w.nonce, w.failure_reason, w.created_at, w.updated_at, w.version
		FROM withdrawals w JOIN assets a ON a.id = w.asset_id WHERE w.id = $1`, id)
	var wd domainwithdrawal.Withdrawal
	if err := row.Scan(&wd.ID, &wd.UserID, &wd.AssetID, &wd.AssetSymbol, &wd.Amount, &wd.DestinationAddress, &wd.Status, &wd.IdempotencyKey, &wd.ApprovalStatus, &wd.TxHash, &wd.Nonce, &wd.FailureReason, &wd.CreatedAt, &wd.UpdatedAt, &wd.Version); err != nil {
		return nil, err
	}
	return &wd, nil
}

// BlockchainRepo tracks blocks and events.
type BlockchainRepo struct{ db *DB }

func NewBlockchainRepo(db *DB) *BlockchainRepo { return &BlockchainRepo{db: db} }

func (r *BlockchainRepo) SaveBlock(ctx context.Context, chainID, blockNumber int64, blockHash, parentHash string) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		INSERT INTO blockchain_blocks (chain_id, block_number, block_hash, parent_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chain_id, block_number) DO UPDATE SET block_hash = EXCLUDED.block_hash, parent_hash = EXCLUDED.parent_hash, processed_at = NOW()`,
		chainID, blockNumber, blockHash, parentHash)
	return err
}

func (r *BlockchainRepo) GetBlock(ctx context.Context, chainID, blockNumber int64) (string, string, bool, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT block_hash, parent_hash FROM blockchain_blocks WHERE chain_id = $1 AND block_number = $2`, chainID, blockNumber)
	var hash, parent string
	if err := row.Scan(&hash, &parent); err != nil {
		if err == pgx.ErrNoRows {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return hash, parent, true, nil
}

func (r *BlockchainRepo) SaveEvent(ctx context.Context, chainID int64, blockNumber int64, blockHash, txHash string, logIndex int, contractAddress, eventName string, payload map[string]interface{}) (bool, error) {
	q := exec(ctx, r.db)
	payloadJSON, _ := json.Marshal(payload)
	tag, err := q.Exec(ctx, `
		INSERT INTO blockchain_events (chain_id, block_number, block_hash, tx_hash, log_index, contract_address, event_name, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (chain_id, tx_hash, log_index) DO NOTHING`,
		chainID, blockNumber, blockHash, txHash, logIndex, contractAddress, eventName, payloadJSON)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *BlockchainRepo) OrphanEventsFromBlock(ctx context.Context, chainID, fromBlock int64) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		UPDATE blockchain_events SET status = 'ORPHANED'
		WHERE chain_id = $1 AND block_number >= $2`, chainID, fromBlock)
	return err
}

func (r *BlockchainRepo) GetScannerState(ctx context.Context, chainID int64) (int64, error) {
	row := r.db.Pool.QueryRow(ctx, `SELECT last_processed_block FROM scanner_state WHERE chain_id = $1`, chainID)
	var last int64
	if err := row.Scan(&last); err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return last, nil
}

func (r *BlockchainRepo) SetScannerState(ctx context.Context, chainID, lastBlock int64) error {
	q := exec(ctx, r.db)
	_, err := q.Exec(ctx, `
		INSERT INTO scanner_state (chain_id, last_processed_block) VALUES ($1, $2)
		ON CONFLICT (chain_id) DO UPDATE SET last_processed_block = EXCLUDED.last_processed_block, updated_at = NOW()`,
		chainID, lastBlock)
	return err
}

// OutboxRepo stores outbox events.
type OutboxRepo struct{ db *DB }

func NewOutboxRepo(db *DB) *OutboxRepo { return &OutboxRepo{db: db} }

func (r *OutboxRepo) Insert(ctx context.Context, aggregateType string, aggregateID uuid.UUID, eventType string, payload map[string]interface{}) error {
	q := exec(ctx, r.db)
	payloadJSON, _ := json.Marshal(payload)
	_, err := q.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)`, aggregateType, aggregateID, eventType, payloadJSON)
	return err
}

func (r *OutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, aggregate_type, aggregate_id, event_type, payload
		FROM outbox_events WHERE published_at IS NULL
		ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []ports.OutboxEvent
	for rows.Next() {
		var e ports.OutboxEvent
		var payloadJSON []byte
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &payloadJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payloadJSON, &e.Payload)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *OutboxRepo) MarkPublished(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE outbox_events SET published_at = NOW() WHERE id = $1`, id)
	return err
}

// AuditRepo records audit logs.
type AuditRepo struct{ db *DB }

func NewAuditRepo(db *DB) *AuditRepo { return &AuditRepo{db: db} }

func (r *AuditRepo) Log(ctx context.Context, actorType, actorID, action, resourceType, resourceID string, metadata map[string]interface{}) error {
	q := exec(ctx, r.db)
	metaJSON, _ := json.Marshal(metadata)
	_, err := q.Exec(ctx, `
		INSERT INTO audit_logs (actor_type, actor_id, action, resource_type, resource_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6)`, actorType, actorID, action, resourceType, resourceID, metaJSON)
	return err
}

// AssetRepo loads assets.
type AssetRepo struct{ db *DB }

func NewAssetRepo(db *DB) *AssetRepo { return &AssetRepo{db: db} }

func (r *AssetRepo) GetBySymbol(ctx context.Context, symbol string) (uuid.UUID, int, error) {
	row := r.db.Pool.QueryRow(ctx, `SELECT id, decimals FROM assets WHERE symbol = $1`, symbol)
	var id uuid.UUID
	var decimals int
	if err := row.Scan(&id, &decimals); err != nil {
		return uuid.Nil, 0, err
	}
	return id, decimals, nil
}

func (r *AssetRepo) GetByID(ctx context.Context, id uuid.UUID) (string, int, error) {
	row := r.db.Pool.QueryRow(ctx, `SELECT symbol, decimals FROM assets WHERE id = $1`, id)
	var symbol string
	var decimals int
	if err := row.Scan(&symbol, &decimals); err != nil {
		return "", 0, err
	}
	return symbol, decimals, nil
}

func (r *AssetRepo) UpdateContractAddress(ctx context.Context, symbol, address string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE assets SET contract_address = $2 WHERE symbol = $1`, symbol, address)
	return err
}
