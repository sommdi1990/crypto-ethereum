package withdrawalsvc

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/soroush/eth-exchange-lab/internal/application/ports"
	ledgersvc "github.com/soroush/eth-exchange-lab/internal/application/ledger"
	domainwithdrawal "github.com/soroush/eth-exchange-lab/internal/domain/withdrawal"
)

// Service handles withdrawal lifecycle.
type Service struct {
	uow        ports.UnitOfWork
	withdrawals ports.WithdrawalRepository
	assets     ports.AssetRepository
	ledger     *ledgersvc.Service
	outbox     ports.OutboxRepository
	txSender   ports.TxSender
	audit      ports.AuditRepository
	enableSigning bool
}

// New creates a withdrawal service.
func New(uow ports.UnitOfWork, withdrawals ports.WithdrawalRepository, assets ports.AssetRepository, ledger *ledgersvc.Service, outbox ports.OutboxRepository, txSender ports.TxSender, audit ports.AuditRepository, enableSigning bool) *Service {
	return &Service{uow: uow, withdrawals: withdrawals, assets: assets, ledger: ledger, outbox: outbox, txSender: txSender, audit: audit, enableSigning: enableSigning}
}

// RequestWithdrawal creates an idempotent withdrawal request.
func (s *Service) RequestWithdrawal(ctx context.Context, userID uuid.UUID, assetSymbol, amountStr, destination, idempotencyKey string) (*domainwithdrawal.Withdrawal, error) {
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount")
	}
	assetID, _, err := s.assets.GetBySymbol(ctx, assetSymbol)
	if err != nil {
		return nil, err
	}
	wid := uuid.New()
	if err := s.ledger.LockWithdrawal(ctx, userID, assetSymbol, amount, wid, "withdrawal-lock:"+idempotencyKey); err != nil {
		return nil, err
	}
	w := domainwithdrawal.Withdrawal{
		ID:                 wid,
		UserID:             userID,
		AssetID:            assetID,
		AssetSymbol:        assetSymbol,
		Amount:             amount,
		DestinationAddress: destination,
		Status:             domainwithdrawal.StatusPendingApproval,
		IdempotencyKey:     idempotencyKey,
		ApprovalStatus:     "PENDING",
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
		Version:            1,
	}
	if err := s.withdrawals.Create(ctx, w); err != nil {
		return nil, err
	}
	_ = s.outbox.Insert(ctx, "withdrawal", wid, "exchange.withdrawal.requested", map[string]interface{}{
		"withdrawal_id": wid.String(), "asset": assetSymbol, "amount": amount.String(),
	})
	_ = s.audit.Log(ctx, "user", userID.String(), "withdrawal.requested", "withdrawal", wid.String(), nil)
	return &w, nil
}

// Approve marks withdrawal approved (mock local workflow).
func (s *Service) Approve(ctx context.Context, withdrawalID uuid.UUID) (*domainwithdrawal.Withdrawal, error) {
	w, err := s.withdrawals.GetByID(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if w.Status != domainwithdrawal.StatusPendingApproval {
		return nil, fmt.Errorf("invalid status for approval")
	}
	w.Status = domainwithdrawal.StatusApproved
	w.ApprovalStatus = "APPROVED"
	w.UpdatedAt = time.Now().UTC()
	if err := s.withdrawals.Update(ctx, *w); err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, "admin", "local", "withdrawal.approved", "withdrawal", withdrawalID.String(), nil)
	return w, nil
}

// Broadcast builds and sends on-chain transaction when signing enabled.
func (s *Service) Broadcast(ctx context.Context, withdrawalID uuid.UUID) (*domainwithdrawal.Withdrawal, error) {
	if !s.enableSigning {
		return nil, fmt.Errorf("signing disabled")
	}
	w, err := s.withdrawals.GetByID(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if w.Status != domainwithdrawal.StatusApproved {
		return nil, fmt.Errorf("withdrawal not approved")
	}
	w.Status = domainwithdrawal.StatusSigning
	if err := s.withdrawals.Update(ctx, *w); err != nil {
		return nil, err
	}

	var withdrawalIDBytes [32]byte
	copy(withdrawalIDBytes[:], withdrawalID[:])

	var txHash string
	if w.AssetSymbol == "ETH" {
		txHash, err = s.txSender.SendVaultWithdrawETH(ctx, w.DestinationAddress, w.Amount.String(), withdrawalIDBytes)
	} else {
		txHash, err = s.txSender.SendVaultWithdrawERC20(ctx, "USDT", w.DestinationAddress, w.Amount.String(), withdrawalIDBytes)
	}
	if err != nil {
		reason := err.Error()
		w.Status = domainwithdrawal.StatusFailed
		w.FailureReason = &reason
		_ = s.withdrawals.Update(ctx, *w)
		return nil, err
	}
	w.Status = domainwithdrawal.StatusBroadcasted
	w.TxHash = &txHash
	w.UpdatedAt = time.Now().UTC()
	if err := s.withdrawals.Update(ctx, *w); err != nil {
		return nil, err
	}
	_ = s.outbox.Insert(ctx, "withdrawal", withdrawalID, "exchange.withdrawal.broadcasted", map[string]interface{}{
		"withdrawal_id": withdrawalID.String(), "tx_hash": txHash,
	})
	return w, nil
}

// Confirm marks withdrawal confirmed and completes ledger.
func (s *Service) Confirm(ctx context.Context, withdrawalID uuid.UUID) error {
	w, err := s.withdrawals.GetByID(ctx, withdrawalID)
	if err != nil {
		return err
	}
	if w.Status != domainwithdrawal.StatusBroadcasted {
		return fmt.Errorf("invalid status")
	}
	if err := s.ledger.CompleteWithdrawal(ctx, w.UserID, w.AssetSymbol, w.Amount, withdrawalID, "withdrawal-complete:"+withdrawalID.String()); err != nil {
		return err
	}
	w.Status = domainwithdrawal.StatusConfirmed
	w.UpdatedAt = time.Now().UTC()
	return s.withdrawals.Update(ctx, *w)
}

// GetByID returns withdrawal.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*domainwithdrawal.Withdrawal, error) {
	return s.withdrawals.GetByID(ctx, id)
}

// WithdrawalIDToBytes converts UUID to bytes32 for contract calls.
func WithdrawalIDToBytes(id uuid.UUID) [32]byte {
	var b [32]byte
	h, _ := hex.DecodeString(id.String()[0:8] + id.String()[9:13])
	copy(b[:], h)
	return b
}
