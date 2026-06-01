package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	domainledger "github.com/soroush/eth-exchange-lab/internal/domain/ledger"
)

// LedgerRepo implements double-entry ledger persistence.
type LedgerRepo struct{ db *DB }

func NewLedgerRepo(db *DB) *LedgerRepo { return &LedgerRepo{db: db} }

func (r *LedgerRepo) EnsureAccount(ctx context.Context, code string, accountType domainledger.AccountType, userID *uuid.UUID, assetID uuid.UUID) (uuid.UUID, error) {
	q := exec(ctx, r.db)
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO ledger_accounts (code, account_type, user_id, asset_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (code) DO UPDATE SET code = EXCLUDED.code
		RETURNING id`, code, string(accountType), userID, assetID).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure account: %w", err)
	}
	return id, nil
}

func (r *LedgerRepo) PostJournal(ctx context.Context, journal domainledger.Journal, lines []domainledger.Line) (*domainledger.Journal, error) {
	if err := domainledger.ValidateJournal(lines); err != nil {
		return nil, err
	}
	q := exec(ctx, r.db)
	var j domainledger.Journal
	err := q.QueryRow(ctx, `
		INSERT INTO ledger_journals (journal_type, reference_type, reference_id, idempotency_key, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, journal_type, reference_type, reference_id, idempotency_key, description, created_at`,
		string(journal.JournalType), journal.ReferenceType, journal.ReferenceID, journal.IdempotencyKey, journal.Description).
		Scan(&j.ID, &j.JournalType, &j.ReferenceType, &j.ReferenceID, &j.IdempotencyKey, &j.Description, &j.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert journal: %w", err)
	}

	for _, line := range lines {
		var accountID uuid.UUID
		err := q.QueryRow(ctx, `SELECT id FROM ledger_accounts WHERE code = $1`, line.AccountCode).Scan(&accountID)
		if err != nil {
			return nil, fmt.Errorf("resolve account %s: %w", line.AccountCode, err)
		}
		var e domainledger.Entry
		err = q.QueryRow(ctx, `
			INSERT INTO ledger_entries (journal_id, account_id, direction, amount)
			VALUES ($1, $2, $3, $4)
			RETURNING id, journal_id, account_id, direction, amount, created_at`,
			j.ID, accountID, string(line.Direction), line.Amount.String()).
			Scan(&e.ID, &e.JournalID, &e.AccountID, &e.Direction, &e.Amount, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert entry: %w", err)
		}
		j.Entries = append(j.Entries, e)
	}
	return &j, nil
}

func (r *LedgerRepo) GetBalances(ctx context.Context, userID uuid.UUID) (map[string]map[string]decimal.Decimal, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT a.symbol, lac.account_type,
			COALESCE(SUM(CASE WHEN le.direction = 'CREDIT' THEN le.amount ELSE -le.amount END), 0)
		FROM ledger_accounts lac
		JOIN assets a ON a.id = lac.asset_id
		LEFT JOIN ledger_entries le ON le.account_id = lac.id
		WHERE lac.user_id = $1
		GROUP BY a.symbol, la.account_type`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]map[string]decimal.Decimal)
	for rows.Next() {
		var symbol, accountType string
		var balance decimal.Decimal
		if err := rows.Scan(&symbol, &accountType, &balance); err != nil {
			return nil, err
		}
		if result[symbol] == nil {
			result[symbol] = make(map[string]decimal.Decimal)
		}
		switch accountType {
		case string(domainledger.AccountUserAvailable):
			result[symbol]["available"] = balance
		case string(domainledger.AccountUserLocked):
			result[symbol]["locked"] = balance
		case string(domainledger.AccountUserWithdrawalLocked):
			result[symbol]["withdrawal_locked"] = balance
		}
	}
	return result, rows.Err()
}

func (r *LedgerRepo) ListJournals(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domainledger.Journal, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT j.id, j.journal_type, j.reference_type, j.reference_id, j.idempotency_key, j.description, j.created_at
		FROM ledger_journals j
		JOIN ledger_entries e ON e.journal_id = j.id
		JOIN ledger_accounts a ON a.id = e.account_id
		WHERE a.user_id = $1
		ORDER BY j.created_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var journals []domainledger.Journal
	for rows.Next() {
		var j domainledger.Journal
		if err := rows.Scan(&j.ID, &j.JournalType, &j.ReferenceType, &j.ReferenceID, &j.IdempotencyKey, &j.Description, &j.CreatedAt); err != nil {
			return nil, err
		}
		journals = append(journals, j)
	}
	return journals, rows.Err()
}

// IdempotencyRepo implements idempotency key storage.
type IdempotencyRepo struct{ db *DB }

func NewIdempotencyRepo(db *DB) *IdempotencyRepo { return &IdempotencyRepo{db: db} }

func (r *IdempotencyRepo) Begin(ctx context.Context, key, scope, requestHash string) (bool, []byte, error) {
	q := exec(ctx, r.db)
	var status string
	var response []byte
	err := q.QueryRow(ctx, `
		SELECT status, response FROM idempotency_keys WHERE key = $1`, key).Scan(&status, &response)
	if err == nil {
		if status == "COMPLETED" {
			return true, response, nil
		}
		return false, nil, fmt.Errorf("idempotency key in progress")
	}
	if err != pgx.ErrNoRows {
		return false, nil, err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO idempotency_keys (key, scope, request_hash, status, expires_at)
		VALUES ($1, $2, $3, 'IN_PROGRESS', $4)`,
		key, scope, requestHash, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		return false, nil, err
	}
	return false, nil, nil
}

func (r *IdempotencyRepo) Complete(ctx context.Context, key string, response []byte) error {
	q := exec(ctx, r.db)
	var respJSON json.RawMessage
	if response != nil {
		respJSON = response
	}
	_, err := q.Exec(ctx, `
		UPDATE idempotency_keys SET status = 'COMPLETED', response = $2 WHERE key = $1`,
		key, respJSON)
	return err
}
