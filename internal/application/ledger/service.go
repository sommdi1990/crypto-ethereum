package ledgersvc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/soroush/eth-exchange-lab/internal/application/ports"
	domainledger "github.com/soroush/eth-exchange-lab/internal/domain/ledger"
)

// Service handles ledger operations.
type Service struct {
	uow    ports.UnitOfWork
	ledger ports.LedgerRepository
	assets ports.AssetRepository
}

func New(uow ports.UnitOfWork, ledger ports.LedgerRepository, assets ports.AssetRepository) *Service {
	return &Service{uow: uow, ledger: ledger, assets: assets}
}

// EnsureUserAccounts creates standard accounts for a user and asset.
func (s *Service) EnsureUserAccounts(ctx context.Context, userID uuid.UUID, symbol string) error {
	assetID, _, err := s.assets.GetBySymbol(ctx, symbol)
	if err != nil {
		return err
	}
	uid := userID
	for _, at := range []domainledger.AccountType{
		domainledger.AccountUserAvailable,
		domainledger.AccountUserLocked,
		domainledger.AccountUserWithdrawalLocked,
	} {
		code := domainledger.AccountCode(userID, at, symbol)
		if _, err := s.ledger.EnsureAccount(ctx, code, at, &uid, assetID); err != nil {
			return err
		}
	}
	return nil
}

// PostBalancedJournal posts a validated journal within optional transaction.
func (s *Service) PostBalancedJournal(ctx context.Context, journal domainledger.Journal, lines []domainledger.Line) (*domainledger.Journal, error) {
	return s.ledger.PostJournal(ctx, journal, lines)
}

// LockForOrder moves available balance to locked for order placement.
func (s *Service) LockForOrder(ctx context.Context, userID uuid.UUID, symbol string, amount decimal.Decimal, refID uuid.UUID, idempotencyKey string) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.EnsureUserAccounts(txCtx, userID, symbol); err != nil {
			return err
		}
		avail := domainledger.AccountCode(userID, domainledger.AccountUserAvailable, symbol)
		locked := domainledger.AccountCode(userID, domainledger.AccountUserLocked, symbol)
		_, err := s.ledger.PostJournal(txCtx, domainledger.Journal{
			JournalType:    domainledger.JournalOrderLock,
			ReferenceType:  "order",
			ReferenceID:    refID,
			IdempotencyKey: idempotencyKey,
			Description:    "lock balance for order",
		}, []domainledger.Line{
			{AccountCode: avail, Direction: domainledger.Debit, Amount: amount},
			{AccountCode: locked, Direction: domainledger.Credit, Amount: amount},
		})
		return err
	})
}

// CreditDeposit credits user after confirmed deposit.
func (s *Service) CreditDeposit(ctx context.Context, userID uuid.UUID, symbol string, amount decimal.Decimal, depositID uuid.UUID, idempotencyKey string) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.EnsureUserAccounts(txCtx, userID, symbol); err != nil {
			return err
		}
		assetID, _, err := s.assets.GetBySymbol(txCtx, symbol)
		if err != nil {
			return err
		}
		hotCode := fmt.Sprintf("exchange:EXCHANGE_HOT:%s", symbol)
		if _, err := s.ledger.EnsureAccount(txCtx, hotCode, domainledger.AccountExchangeHot, nil, assetID); err != nil {
			return err
		}
		userAvail := domainledger.AccountCode(userID, domainledger.AccountUserAvailable, symbol)
		_, err = s.ledger.PostJournal(txCtx, domainledger.Journal{
			JournalType:    domainledger.JournalDepositCredit,
			ReferenceType:  "deposit",
			ReferenceID:    depositID,
			IdempotencyKey: idempotencyKey,
			Description:    "deposit credited",
		}, []domainledger.Line{
			{AccountCode: hotCode, Direction: domainledger.Debit, Amount: amount},
			{AccountCode: userAvail, Direction: domainledger.Credit, Amount: amount},
		})
		return err
	})
}

// SettleTrade performs double-entry for a trade between buyer and seller.
func (s *Service) SettleTrade(ctx context.Context, buyerID, sellerID uuid.UUID, baseSymbol, quoteSymbol string, price, qty, fee decimal.Decimal, tradeID uuid.UUID, idempotencyKey string) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.EnsureUserAccounts(txCtx, buyerID, baseSymbol); err != nil {
			return err
		}
		if err := s.EnsureUserAccounts(txCtx, buyerID, quoteSymbol); err != nil {
			return err
		}
		if err := s.EnsureUserAccounts(txCtx, sellerID, baseSymbol); err != nil {
			return err
		}
		if err := s.EnsureUserAccounts(txCtx, sellerID, quoteSymbol); err != nil {
			return err
		}
		notional := price.Mul(qty)
		feeAssetID, _, _ := s.assets.GetBySymbol(txCtx, quoteSymbol)
		feeCode := fmt.Sprintf("exchange:EXCHANGE_FEE:%s", quoteSymbol)
		_, _ = s.ledger.EnsureAccount(txCtx, feeCode, domainledger.AccountExchangeFee, nil, feeAssetID)

		lines := []domainledger.Line{
			{AccountCode: domainledger.AccountCode(buyerID, domainledger.AccountUserLocked, quoteSymbol), Direction: domainledger.Debit, Amount: notional},
			{AccountCode: domainledger.AccountCode(sellerID, domainledger.AccountUserAvailable, quoteSymbol), Direction: domainledger.Credit, Amount: notional.Sub(fee)},
			{AccountCode: domainledger.AccountCode(sellerID, domainledger.AccountUserLocked, baseSymbol), Direction: domainledger.Debit, Amount: qty},
			{AccountCode: domainledger.AccountCode(buyerID, domainledger.AccountUserAvailable, baseSymbol), Direction: domainledger.Credit, Amount: qty},
		}
		if fee.IsPositive() {
			lines = append(lines, domainledger.Line{AccountCode: feeCode, Direction: domainledger.Credit, Amount: fee})
		}
		_, err := s.ledger.PostJournal(txCtx, domainledger.Journal{
			JournalType:    domainledger.JournalTrade,
			ReferenceType:  "trade",
			ReferenceID:    tradeID,
			IdempotencyKey: idempotencyKey,
			Description:    "trade settlement",
		}, lines)
		return err
	})
}

// UnlockOrder returns locked balance on cancel.
func (s *Service) UnlockOrder(ctx context.Context, userID uuid.UUID, symbol string, amount decimal.Decimal, orderID uuid.UUID, idempotencyKey string) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context) error {
		avail := domainledger.AccountCode(userID, domainledger.AccountUserAvailable, symbol)
		locked := domainledger.AccountCode(userID, domainledger.AccountUserLocked, symbol)
		_, err := s.ledger.PostJournal(txCtx, domainledger.Journal{
			JournalType:    domainledger.JournalOrderCancel,
			ReferenceType:  "order",
			ReferenceID:    orderID,
			IdempotencyKey: idempotencyKey,
			Description:    "unlock on cancel",
		}, []domainledger.Line{
			{AccountCode: locked, Direction: domainledger.Debit, Amount: amount},
			{AccountCode: avail, Direction: domainledger.Credit, Amount: amount},
		})
		return err
	})
}

// LockWithdrawal locks available balance for withdrawal.
func (s *Service) LockWithdrawal(ctx context.Context, userID uuid.UUID, symbol string, amount decimal.Decimal, withdrawalID uuid.UUID, idempotencyKey string) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.EnsureUserAccounts(txCtx, userID, symbol); err != nil {
			return err
		}
		avail := domainledger.AccountCode(userID, domainledger.AccountUserAvailable, symbol)
		wlocked := domainledger.AccountCode(userID, domainledger.AccountUserWithdrawalLocked, symbol)
		_, err := s.ledger.PostJournal(txCtx, domainledger.Journal{
			JournalType:    domainledger.JournalWithdrawalLock,
			ReferenceType:  "withdrawal",
			ReferenceID:    withdrawalID,
			IdempotencyKey: idempotencyKey,
			Description:    "lock for withdrawal",
		}, []domainledger.Line{
			{AccountCode: avail, Direction: domainledger.Debit, Amount: amount},
			{AccountCode: wlocked, Direction: domainledger.Credit, Amount: amount},
		})
		return err
	})
}

// CompleteWithdrawal finalizes withdrawal after on-chain confirmation.
func (s *Service) CompleteWithdrawal(ctx context.Context, userID uuid.UUID, symbol string, amount decimal.Decimal, withdrawalID uuid.UUID, idempotencyKey string) error {
	return s.uow.WithinTx(ctx, func(txCtx context.Context) error {
		assetID, _, err := s.assets.GetBySymbol(txCtx, symbol)
		if err != nil {
			return err
		}
		hotCode := fmt.Sprintf("exchange:EXCHANGE_HOT:%s", symbol)
		if _, err := s.ledger.EnsureAccount(txCtx, hotCode, domainledger.AccountExchangeHot, nil, assetID); err != nil {
			return err
		}
		wlocked := domainledger.AccountCode(userID, domainledger.AccountUserWithdrawalLocked, symbol)
		_, err = s.ledger.PostJournal(txCtx, domainledger.Journal{
			JournalType:    domainledger.JournalWithdrawalComplete,
			ReferenceType:  "withdrawal",
			ReferenceID:    withdrawalID,
			IdempotencyKey: idempotencyKey,
			Description:    "withdrawal complete",
		}, []domainledger.Line{
			{AccountCode: wlocked, Direction: domainledger.Debit, Amount: amount},
			{AccountCode: hotCode, Direction: domainledger.Credit, Amount: amount},
		})
		return err
	})
}

// GetBalances returns user balances from ledger.
func (s *Service) GetBalances(ctx context.Context, userID uuid.UUID) (map[string]map[string]decimal.Decimal, error) {
	return s.ledger.GetBalances(ctx, userID)
}
