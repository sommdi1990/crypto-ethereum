package depositsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/soroush/eth-exchange-lab/internal/application/ports"
	ledgersvc "github.com/soroush/eth-exchange-lab/internal/application/ledger"
	domaindeposit "github.com/soroush/eth-exchange-lab/internal/domain/deposit"
)

// Service handles deposit crediting.
type Service struct {
	deposits ports.DepositRepository
	ledger   *ledgersvc.Service
	outbox   ports.OutboxRepository
}

func New(deposits ports.DepositRepository, ledger *ledgersvc.Service, outbox ports.OutboxRepository) *Service {
	return &Service{deposits: deposits, ledger: ledger, outbox: outbox}
}

// RecordDetected creates a new deposit from scanner.
func (s *Service) RecordDetected(ctx context.Context, userID, walletAddressID, assetID uuid.UUID, symbol, amount, txHash, blockHash, depositRef string, logIndex int, blockNumber, chainID int64, requiredConfirmations int) (*domaindeposit.Deposit, error) {
	existing, err := s.deposits.FindByDedupKey(ctx, chainID, txHash, logIndex)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	amt, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	d := domaindeposit.Deposit{
		ID:                    uuid.New(),
		UserID:                userID,
		WalletAddressID:       walletAddressID,
		AssetID:               assetID,
		Amount:                amt,
		TxHash:                txHash,
		LogIndex:              logIndex,
		BlockNumber:           blockNumber,
		BlockHash:             blockHash,
		ConfirmationCount:     0,
		RequiredConfirmations: requiredConfirmations,
		Status:                domaindeposit.StatusDetected,
		ChainID:               chainID,
		DepositRef:            depositRef,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}
	if err := s.deposits.Create(ctx, d); err != nil {
		return nil, err
	}
	_ = s.outbox.Insert(ctx, "deposit", d.ID, "exchange.deposit.detected", map[string]interface{}{
		"deposit_id": d.ID.String(), "tx_hash": txHash,
	})
	return &d, nil
}

// UpdateConfirmations updates confirmation count and credits when ready.
func (s *Service) UpdateConfirmations(ctx context.Context, depositID uuid.UUID, confirmationCount int, symbol string) error {
	d, err := s.deposits.GetByID(ctx, depositID)
	if err != nil {
		return err
	}
	if d.Status == domaindeposit.StatusCredited || d.Status == domaindeposit.StatusOrphaned {
		return nil
	}
	d.ConfirmationCount = confirmationCount
	if confirmationCount < d.RequiredConfirmations {
		d.Status = domaindeposit.StatusConfirming
		return s.deposits.Update(ctx, *d)
	}
	if err := s.ledger.CreditDeposit(ctx, d.UserID, symbol, d.Amount, d.ID, "deposit-credit:"+d.ID.String()); err != nil {
		return err
	}
	d.Status = domaindeposit.StatusCredited
	if err := s.deposits.Update(ctx, *d); err != nil {
		return err
	}
	_ = s.outbox.Insert(ctx, "deposit", d.ID, "exchange.deposit.credited", map[string]interface{}{
		"deposit_id": d.ID.String(), "amount": d.Amount.String(),
	})
	return nil
}

// ListByUser returns deposits for user.
func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID) ([]domaindeposit.Deposit, error) {
	return s.deposits.ListByUserID(ctx, userID)
}

// OrphanDeposit marks deposit orphaned due to reorg.
func (s *Service) OrphanDeposit(ctx context.Context, depositID uuid.UUID) error {
	d, err := s.deposits.GetByID(ctx, depositID)
	if err != nil {
		return err
	}
	if d.Status == domaindeposit.StatusCredited {
		return fmt.Errorf("cannot orphan credited deposit without reversal journal")
	}
	d.Status = domaindeposit.StatusOrphaned
	return s.deposits.Update(ctx, *d)
}
