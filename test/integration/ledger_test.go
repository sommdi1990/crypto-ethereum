package integration

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	domainledger "github.com/soroush/eth-exchange-lab/internal/domain/ledger"
)

func TestLedgerValidateDepositFlow(t *testing.T) {
	lines := []domainledger.Line{
		{AccountCode: "exchange:EXCHANGE_HOT:ETH", Direction: domainledger.Debit, Amount: decimal.NewFromInt(1)},
		{AccountCode: "user:u1:USER_AVAILABLE:ETH", Direction: domainledger.Credit, Amount: decimal.NewFromInt(1)},
	}
	require.NoError(t, domainledger.ValidateJournal(lines))
}
