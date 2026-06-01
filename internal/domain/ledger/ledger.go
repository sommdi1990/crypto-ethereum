package ledger

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AccountType classifies ledger accounts.
type AccountType string

const (
	AccountUserAvailable         AccountType = "USER_AVAILABLE"
	AccountUserLocked            AccountType = "USER_LOCKED"
	AccountUserWithdrawalLocked  AccountType = "USER_WITHDRAWAL_LOCKED"
	AccountExchangeHot           AccountType = "EXCHANGE_HOT"
	AccountExchangeFee           AccountType = "EXCHANGE_FEE"
)

// Direction is debit or credit.
type Direction string

const (
	Debit  Direction = "DEBIT"
	Credit Direction = "CREDIT"
)

// JournalType classifies financial journals.
type JournalType string

const (
	JournalDepositCredit      JournalType = "DEPOSIT_CREDIT"
	JournalOrderLock          JournalType = "ORDER_LOCK"
	JournalTrade              JournalType = "TRADE"
	JournalOrderCancel        JournalType = "ORDER_CANCEL"
	JournalWithdrawalLock     JournalType = "WITHDRAWAL_LOCK"
	JournalWithdrawalComplete JournalType = "WITHDRAWAL_COMPLETE"
	JournalReorgReversal      JournalType = "REORG_REVERSAL"
)

// Account is a double-entry ledger account.
type Account struct {
	ID          uuid.UUID
	Code        string
	AccountType AccountType
	UserID      *uuid.UUID
	AssetID     uuid.UUID
	CreatedAt   time.Time
}

// Entry is a single debit or credit line.
type Entry struct {
	ID        uuid.UUID
	JournalID uuid.UUID
	AccountID uuid.UUID
	Direction Direction
	Amount    decimal.Decimal
	CreatedAt time.Time
}

// Journal groups balanced entries.
type Journal struct {
	ID             uuid.UUID
	JournalType    JournalType
	ReferenceType  string
	ReferenceID    uuid.UUID
	IdempotencyKey string
	Description    string
	Entries        []Entry
	CreatedAt      time.Time
}

// Line is an input line for journal creation.
type Line struct {
	AccountCode string
	Direction   Direction
	Amount      decimal.Decimal
}

var ErrUnbalancedJournal = errors.New("journal entries are not balanced")

// ValidateJournal ensures debits equal credits.
func ValidateJournal(lines []Line) error {
	var debits, credits decimal.Decimal
	for _, l := range lines {
		if l.Amount.IsNegative() {
			return errors.New("amount must be non-negative")
		}
		switch l.Direction {
		case Debit:
			debits = debits.Add(l.Amount)
		case Credit:
			credits = credits.Add(l.Amount)
		default:
			return errors.New("invalid direction")
		}
	}
	if !debits.Equal(credits) {
		return ErrUnbalancedJournal
	}
	return nil
}

// AccountCode builds a standard account code string.
func AccountCode(userID uuid.UUID, accountType AccountType, symbol string) string {
	switch accountType {
	case AccountExchangeHot, AccountExchangeFee:
		return "exchange:" + string(accountType) + ":" + symbol
	default:
		return "user:" + userID.String() + ":" + string(accountType) + ":" + symbol
	}
}
