package scanner

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	ethadapter "github.com/soroush/eth-exchange-lab/internal/adapters/ethereum"
	depositsvc "github.com/soroush/eth-exchange-lab/internal/application/deposit"
	"github.com/soroush/eth-exchange-lab/internal/application/ports"
)

// Service polls blocks and processes Vault deposit events.
type Service struct {
	log               zerolog.Logger
	eth               *ethadapter.Client
	chainID           int64
	vaultAddress      common.Address
	blockchain        ports.BlockchainRepository
	wallets           ports.WalletRepository
	deposits          ports.DepositRepository
	depositSvc        *depositsvc.Service
	assets            ports.AssetRepository
	outbox            ports.OutboxRepository
	confirmationCount int
	startBlock        uint64
	pollInterval      time.Duration
	lastProcessed     int64
}

// New creates a block scanner service.
func New(log zerolog.Logger, eth *ethadapter.Client, chainID int64, vaultAddress string, blockchain ports.BlockchainRepository, wallets ports.WalletRepository, deposits ports.DepositRepository, depositSvc *depositsvc.Service, assets ports.AssetRepository, outbox ports.OutboxRepository, confirmationCount int, startBlock uint64, pollInterval time.Duration) *Service {
	return &Service{
		log: log, eth: eth, chainID: chainID,
		vaultAddress: common.HexToAddress(vaultAddress),
		blockchain: blockchain, wallets: wallets, deposits: deposits, depositSvc: depositSvc,
		assets: assets, outbox: outbox, confirmationCount: confirmationCount,
		startBlock: startBlock, pollInterval: pollInterval,
	}
}

// Run starts polling until context cancelled.
func (s *Service) Run(ctx context.Context) error {
	last, err := s.blockchain.GetScannerState(ctx, s.chainID)
	if err != nil {
		return err
	}
	if last == 0 && s.startBlock > 0 {
		last = int64(s.startBlock) - 1
	}
	s.lastProcessed = last

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.poll(ctx); err != nil {
				s.log.Error().Err(err).Msg("scanner poll")
			}
		}
	}
}

func (s *Service) poll(ctx context.Context) error {
	latest, err := s.eth.LatestBlockNumber(ctx)
	if err != nil {
		return err
	}
	for n := s.lastProcessed + 1; n <= int64(latest); n++ {
		if err := s.processBlock(ctx, uint64(n)); err != nil {
			return fmt.Errorf("block %d: %w", n, err)
		}
		s.lastProcessed = n
		_ = s.blockchain.SetScannerState(ctx, s.chainID, n)
	}
	return s.updateConfirmations(ctx, int64(latest))
}

func (s *Service) processBlock(ctx context.Context, number uint64) error {
	hash, parentHash, err := s.eth.BlockByNumber(ctx, number)
	if err != nil {
		return err
	}

	if number > 0 {
		storedHash, _, found, err := s.blockchain.GetBlock(ctx, s.chainID, int64(number))
		if err != nil {
			return err
		}
		if found && !strings.EqualFold(storedHash, hash) {
			s.log.Warn().Uint64("block", number).Str("old", storedHash).Str("new", hash).Msg("reorg detected")
			_ = s.blockchain.OrphanEventsFromBlock(ctx, s.chainID, int64(number))
			_ = s.outbox.Insert(ctx, "blockchain", uuid.New(), "exchange.blockchain.reorg", map[string]interface{}{
				"chain_id": s.chainID, "block_number": number,
			})
		}
	}

	if err := s.blockchain.SaveBlock(ctx, s.chainID, int64(number), hash, parentHash); err != nil {
		return err
	}

	if s.vaultAddress == (common.Address{}) {
		return nil
	}

	logs, err := s.eth.EthClient().FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(number)),
		ToBlock:   big.NewInt(int64(number)),
		Addresses: []common.Address{s.vaultAddress},
	})
	if err != nil {
		return err
	}

	for _, lg := range logs {
		eventName := ethadapter.ClassifyEvent(lg)
		if eventName == "" {
			continue
		}
		inserted, err := s.blockchain.SaveEvent(ctx, s.chainID, int64(number), hash, lg.TxHash.Hex(), int(lg.Index), lg.Address.Hex(), eventName, map[string]interface{}{})
		if err != nil || !inserted {
			continue
		}
		if err := s.handleDeposit(ctx, eventName, lg, hash); err != nil {
			s.log.Error().Err(err).Str("event", eventName).Msg("deposit handler")
		}
	}
	return nil
}

func (s *Service) handleDeposit(ctx context.Context, eventName string, lg types.Log, blockHash string) error {
	var userAddr common.Address
	var amount *big.Int
	symbol := "ETH"
	switch eventName {
	case "DepositETH":
		userAddr, amount = ethadapter.ParseDepositETH(lg)
	case "DepositERC20":
		_, userAddr, amount = ethadapter.ParseDepositERC20(lg)
		symbol = "USDT"
	default:
		return nil
	}

	addr, userID, err := s.wallets.FindAddress(ctx, s.chainID, userAddr.Hex())
	if err != nil {
		return nil
	}
	assetID, _, err := s.assets.GetBySymbol(ctx, symbol)
	if err != nil {
		return err
	}
	amt := decimal.NewFromBigInt(amount, 0)
	if symbol == "ETH" {
		amt = amt.Shift(-18)
	} else {
		amt = amt.Shift(-6)
	}
	_, err = s.depositSvc.RecordDetected(ctx, *userID, addr.ID, assetID, symbol, amt.String(), lg.TxHash.Hex(), blockHash, "", int(lg.Index), int64(lg.BlockNumber), s.chainID, s.confirmationCount)
	return err
}

func (s *Service) updateConfirmations(ctx context.Context, latestBlock int64) error {
	pending, err := s.deposits.ListPendingConfirmation(ctx)
	if err != nil {
		return err
	}
	for _, d := range pending {
		confirmations := int(latestBlock - d.BlockNumber + 1)
		if confirmations < 0 {
			confirmations = 0
		}
		symbol, _, _ := s.assets.GetByID(ctx, d.AssetID)
		if err := s.depositSvc.UpdateConfirmations(ctx, d.ID, confirmations, symbol); err != nil {
			s.log.Error().Err(err).Str("deposit_id", d.ID.String()).Msg("update confirmations")
		}
	}
	return nil
}

// LastProcessedBlock returns last scanned block.
func (s *Service) LastProcessedBlock() int64 {
	return s.lastProcessed
}
