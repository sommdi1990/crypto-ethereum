package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	domainwallet "github.com/soroush/eth-exchange-lab/internal/domain/wallet"
)

// WalletRepo implements wallet persistence.
type WalletRepo struct {
	db      *DB
	chainID int64
}

func NewWalletRepo(db *DB, chainID int64) *WalletRepo {
	return &WalletRepo{db: db, chainID: chainID}
}

func (r *WalletRepo) CreateWallet(ctx context.Context, userID uuid.UUID, label, address string, chainID int64) (*domainwallet.Wallet, *domainwallet.Address, error) {
	q := exec(ctx, r.db)
	var w domainwallet.Wallet
	if err := q.QueryRow(ctx, `
		INSERT INTO wallets (user_id, label) VALUES ($1, $2)
		RETURNING id, user_id, label, created_at`, userID, label).
		Scan(&w.ID, &w.UserID, &w.Label, &w.CreatedAt); err != nil {
		return nil, nil, fmt.Errorf("insert wallet: %w", err)
	}
	var a domainwallet.Address
	if err := q.QueryRow(ctx, `
		INSERT INTO wallet_addresses (wallet_id, chain_id, address, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id, wallet_id, chain_id, address, derivation_path, is_active, created_at`,
		w.ID, chainID, address).
		Scan(&a.ID, &a.WalletID, &a.ChainID, &a.Address, &a.DerivationPath, &a.IsActive, &a.CreatedAt); err != nil {
		return nil, nil, fmt.Errorf("insert address: %w", err)
	}
	return &w, &a, nil
}

func (r *WalletRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domainwallet.Wallet, []domainwallet.Address, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT w.id, w.user_id, w.label, w.created_at,
		       wa.id, wa.wallet_id, wa.chain_id, wa.address, wa.derivation_path, wa.is_active, wa.created_at
		FROM wallets w
		JOIN wallet_addresses wa ON wa.wallet_id = w.id
		WHERE w.user_id = $1`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var wallets []domainwallet.Wallet
	var addresses []domainwallet.Address
	for rows.Next() {
		var w domainwallet.Wallet
		var a domainwallet.Address
		if err := rows.Scan(&w.ID, &w.UserID, &w.Label, &w.CreatedAt,
			&a.ID, &a.WalletID, &a.ChainID, &a.Address, &a.DerivationPath, &a.IsActive, &a.CreatedAt); err != nil {
			return nil, nil, err
		}
		wallets = append(wallets, w)
		addresses = append(addresses, a)
	}
	return wallets, addresses, rows.Err()
}

func (r *WalletRepo) FindAddress(ctx context.Context, chainID int64, address string) (*domainwallet.Address, *uuid.UUID, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT wa.id, wa.wallet_id, wa.chain_id, wa.address, wa.derivation_path, wa.is_active, wa.created_at, w.user_id
		FROM wallet_addresses wa
		JOIN wallets w ON w.id = wa.wallet_id
		WHERE wa.chain_id = $1 AND lower(wa.address) = lower($2) AND wa.is_active = true`,
		chainID, address)
	var a domainwallet.Address
	var userID uuid.UUID
	if err := row.Scan(&a.ID, &a.WalletID, &a.ChainID, &a.Address, &a.DerivationPath, &a.IsActive, &a.CreatedAt, &userID); err != nil {
		return nil, nil, fmt.Errorf("address not found: %w", err)
	}
	return &a, &userID, nil
}

// LocalAddressGenerator derives deterministic local deposit addresses from user ID.
type LocalAddressGenerator struct{}

func (LocalAddressGenerator) GenerateAddress(_ context.Context, userID uuid.UUID) (string, error) {
	h := sha256.Sum256([]byte("eth-exchange-lab:" + userID.String()))
	// 20-byte pseudo address prefixed with 0x
	return "0x" + hex.EncodeToString(h[:20]), nil
}
