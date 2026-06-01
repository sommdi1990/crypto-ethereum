package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	ethadapter "github.com/soroush/eth-exchange-lab/internal/adapters/ethereum"
	httpadapter "github.com/soroush/eth-exchange-lab/internal/adapters/http"
	"github.com/soroush/eth-exchange-lab/internal/adapters/http/handlers"
	kafkaadapter "github.com/soroush/eth-exchange-lab/internal/adapters/kafka"
	"github.com/soroush/eth-exchange-lab/internal/adapters/postgres"
	redisadapter "github.com/soroush/eth-exchange-lab/internal/adapters/redis"
	scanneradapter "github.com/soroush/eth-exchange-lab/internal/adapters/scanner"
	depositsvc "github.com/soroush/eth-exchange-lab/internal/application/deposit"
	ledgersvc "github.com/soroush/eth-exchange-lab/internal/application/ledger"
	ordersvc "github.com/soroush/eth-exchange-lab/internal/application/order"
	withdrawalsvc "github.com/soroush/eth-exchange-lab/internal/application/withdrawal"
	"github.com/soroush/eth-exchange-lab/internal/config"
	domainmatching "github.com/soroush/eth-exchange-lab/internal/domain/matching"
	"github.com/soroush/eth-exchange-lab/internal/platform/logger"
	"github.com/soroush/eth-exchange-lab/internal/platform/metrics"
	"github.com/soroush/eth-exchange-lab/internal/platform/shutdown"
)

type readiness struct {
	pg  *postgres.DB
	rdb *redisadapter.Client
	eth *ethadapter.Client
}

func (r *readiness) PingPostgres(ctx context.Context) error { return r.pg.Ping(ctx) }
func (r *readiness) PingRedis(ctx context.Context) error   { return r.rdb.Ping(ctx) }
func (r *readiness) PingRPC(ctx context.Context) error {
	if r.eth == nil {
		return fmt.Errorf("ethereum client not configured")
	}
	return r.eth.Ping(ctx)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)
	_ = metrics.NewRegistry()

	ctx := context.Background()
	db, err := postgres.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("postgres")
	}
	defer db.Close()

	rdb, err := redisadapter.New(ctx, cfg.RedisAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("redis")
	}
	defer rdb.Close()

	var ethClient *ethadapter.Client
	if cfg.EthRPCURL != "" {
		ethClient, err = ethadapter.NewClient(ctx, cfg.EthRPCURL, cfg.ChainID)
		if err != nil {
			log.Warn().Err(err).Msg("ethereum client unavailable")
		}
	}

	uow := postgres.NewUnitOfWork(db)
	userRepo := postgres.NewUserRepo(db)
	walletRepo := postgres.NewWalletRepo(db, cfg.ChainID)
	ledgerRepo := postgres.NewLedgerRepo(db)
	marketRepo := postgres.NewMarketRepo(db)
	orderRepo := postgres.NewOrderRepo(db)
	tradeRepo := postgres.NewTradeRepo(db)
	depositRepo := postgres.NewDepositRepo(db)
	withdrawalRepo := postgres.NewWithdrawalRepo(db)
	blockchainRepo := postgres.NewBlockchainRepo(db)
	outboxRepo := postgres.NewOutboxRepo(db)
	auditRepo := postgres.NewAuditRepo(db)
	assetRepo := postgres.NewAssetRepo(db)

	ledgerService := ledgersvc.New(uow, ledgerRepo, assetRepo)
	depositService := depositsvc.New(depositRepo, ledgerService, outboxRepo)

	feeRate, _ := decimal.NewFromString(cfg.MatchingFeeRate)
	market, _ := marketRepo.GetBySymbol(ctx, "ETH-USDT")
	var engine *domainmatching.Engine
	if market != nil {
		engine = domainmatching.NewEngine(market.ID, feeRate)
	} else {
		engine = domainmatching.NewEngine(uuid.Nil, feeRate)
	}

	orderService := ordersvc.New(uow, orderRepo, tradeRepo, marketRepo, ledgerService, outboxRepo, engine, feeRate)
	if market != nil {
		_ = orderService.RebuildEngine(ctx, market.ID)
	}

	txSender, _ := ethadapter.NewTxSender(ethClient, cfg.PrivateKey, cfg.VaultContractAddress, cfg.EnableSigning)
	withdrawalService := withdrawalsvc.New(uow, withdrawalRepo, assetRepo, ledgerService, outboxRepo, txSender, auditRepo, cfg.EnableSigning)

	api := &handlers.API{
		Users: userRepo, Wallets: walletRepo, Ledger: ledgerService, LedgerRepo: ledgerRepo,
		Orders: orderService, Deposits: depositService, Withdrawals: withdrawalService,
		ChainID: cfg.ChainID, AppEnv: cfg.AppEnv, AddrGen: postgres.LocalAddressGenerator{},
	}

	srv := httpadapter.NewServer(log, api, &readiness{pg: db, rdb: rdb, eth: ethClient})
	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	runCtx, cancel := shutdown.WaitForSignal()
	defer cancel()

	// Background workers
	if ethClient != nil && cfg.VaultContractAddress != "" {
		scannerSvc := scanneradapter.New(log, ethClient, cfg.ChainID, cfg.VaultContractAddress,
			blockchainRepo, walletRepo, depositRepo, depositService, assetRepo, outboxRepo,
			cfg.ConfirmationCount, cfg.ScannerStartBlock, cfg.ScannerPollInterval)
		go func() {
			if err := scannerSvc.Run(runCtx); err != nil && runCtx.Err() == nil {
				log.Error().Err(err).Msg("scanner stopped")
			}
		}()
	}

	producer := kafkaadapter.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()
	outboxWorker := kafkaadapter.NewOutboxWorker(log, outboxRepo, producer, 2*time.Second)
	go func() {
		if err := outboxWorker.Run(runCtx); err != nil && runCtx.Err() == nil {
			log.Error().Err(err).Msg("outbox worker stopped")
		}
	}()

	go func() {
		log.Info().Str("port", cfg.HTTPPort).Msg("starting http server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server")
		}
	}()

	<-runCtx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	log.Info().Msg("shutdown complete")
}
