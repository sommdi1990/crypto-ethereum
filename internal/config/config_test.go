package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "local", cfg.AppEnv)
	require.False(t, cfg.EnableSigning)
	require.False(t, cfg.EnableMainnet)
	require.Equal(t, int64(31337), cfg.ChainID)
}

func TestLoad_SigningRequiresKey(t *testing.T) {
	os.Clearenv()
	os.Setenv("ENABLE_SIGNING", "true")
	_, err := Load()
	require.Error(t, err)
}
