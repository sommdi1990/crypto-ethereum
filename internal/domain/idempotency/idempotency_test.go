package idempotency

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

var ErrInProgress = errors.New("idempotency key in progress")

func TestKeyFormat(t *testing.T) {
	key := "order:abc-123"
	require.Contains(t, key, "order:")
}
