package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	AppEnv                  string
	ChainID                 int64
	EthRPCURL               string
	EthWSURL                string
	VaultContractAddress    string
	MockUSDTContractAddress string
	PostgresDSN             string
	RedisAddr               string
	KafkaBrokers            string
	ConfirmationCount       int
	EnableSigning           bool
	EnableMainnet           bool
	PrivateKey              string
	LogLevel                string
	HTTPPort                string
	ScannerStartBlock       uint64
	ScannerPollInterval     time.Duration
	MatchingFeeRate         string
}

// Load reads configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:                  getEnv("APP_ENV", "local"),
		EthRPCURL:               getEnv("ETH_RPC_URL", "http://localhost:8545"),
		EthWSURL:                getEnv("ETH_WS_URL", ""),
		VaultContractAddress:    getEnv("VAULT_CONTRACT_ADDRESS", ""),
		MockUSDTContractAddress: getEnv("MOCK_USDT_CONTRACT_ADDRESS", ""),
		PostgresDSN:             getEnv("POSTGRES_DSN", "postgres://exchange:exchange@localhost:5432/exchange?sslmode=disable"),
		RedisAddr:               getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:            getEnv("KAFKA_BROKERS", "localhost:9092"),
		ConfirmationCount:       getEnvInt("CONFIRMATION_COUNT", 3),
		EnableSigning:           getEnvBool("ENABLE_SIGNING", false),
		EnableMainnet:           getEnvBool("ENABLE_MAINNET", false),
		PrivateKey:              getEnv("PRIVATE_KEY", ""),
		LogLevel:                getEnv("LOG_LEVEL", "info"),
		HTTPPort:                getEnv("HTTP_PORT", "8080"),
		ScannerStartBlock:       uint64(getEnvInt("SCANNER_START_BLOCK", 0)),
		ScannerPollInterval:     getEnvDuration("SCANNER_POLL_INTERVAL", 2*time.Second),
		MatchingFeeRate:         getEnv("MATCHING_FEE_RATE", "0.001"),
	}

	chainID, err := strconv.ParseInt(getEnv("CHAIN_ID", "31337"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse CHAIN_ID: %w", err)
	}
	cfg.ChainID = chainID

	if cfg.EnableMainnet && cfg.AppEnv != "production" {
		return nil, fmt.Errorf("ENABLE_MAINNET requires APP_ENV=production")
	}
	if cfg.EnableSigning && cfg.PrivateKey == "" {
		return nil, fmt.Errorf("ENABLE_SIGNING requires PRIVATE_KEY")
	}
	if cfg.EnableMainnet && !cfg.EnableSigning {
		return nil, fmt.Errorf("mainnet operations require explicit ENABLE_SIGNING=true")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
