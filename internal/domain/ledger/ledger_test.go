package ledger

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestValidateJournal_Balanced(t *testing.T) {
	lines := []Line{
		{AccountCode: "a", Direction: Debit, Amount: decimal.NewFromInt(10)},
		{AccountCode: "b", Direction: Credit, Amount: decimal.NewFromInt(10)},
	}
	require.NoError(t, ValidateJournal(lines))
}

func TestValidateJournal_Unbalanced(t *testing.T) {
	lines := []Line{
		{AccountCode: "a", Direction: Debit, Amount: decimal.NewFromInt(10)},
		{AccountCode: "b", Direction: Credit, Amount: decimal.NewFromInt(9)},
	}
	require.ErrorIs(t, ValidateJournal(lines), ErrUnbalancedJournal)
}

func TestAccountCode(t *testing.T) {
	uid := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	code := AccountCode(uid, AccountUserAvailable, "ETH")
	require.Contains(t, code, "user:")
	require.Contains(t, code, "AVAILABLE")
}
