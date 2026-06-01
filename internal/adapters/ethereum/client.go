package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/soroush/eth-exchange-lab/internal/platform/retry"
)

// Client wraps go-ethereum RPC with circuit breaker semantics.
type Client struct {
	rpc       *ethclient.Client
	chainID   int64
	mu        sync.Mutex
	failures  int
	maxFails  int
	openUntil time.Time
}

// NewClient dials RPC and verifies chain ID.
func NewClient(ctx context.Context, rpcURL string, expectedChainID int64) (*Client, error) {
	rpc, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}
	if chainID.Int64() != expectedChainID {
		return nil, fmt.Errorf("chain id mismatch: got %d want %d", chainID.Int64(), expectedChainID)
	}
	return &Client{rpc: rpc, chainID: expectedChainID, maxFails: 5}, nil
}

// EthClient returns underlying client for log filtering.
func (c *Client) EthClient() *ethclient.Client {
	return c.rpc
}

func (c *Client) allow() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.openUntil) {
		return fmt.Errorf("circuit open")
	}
	return nil
}

func (c *Client) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
}

func (c *Client) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	if c.failures >= c.maxFails {
		c.openUntil = time.Now().Add(30 * time.Second)
		c.failures = 0
	}
}

// Ping checks RPC connectivity.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.allow(); err != nil {
		return err
	}
	err := retry.Do(ctx, 3, time.Second, func() error {
		_, err := c.rpc.BlockNumber(ctx)
		return err
	})
	if err != nil {
		c.recordFailure()
		return err
	}
	c.recordSuccess()
	return nil
}

// ChainID returns configured chain ID.
func (c *Client) ChainID(_ context.Context) (int64, error) {
	return c.chainID, nil
}

// LatestBlockNumber returns latest block.
func (c *Client) LatestBlockNumber(ctx context.Context) (uint64, error) {
	if err := c.allow(); err != nil {
		return 0, err
	}
	n, err := c.rpc.BlockNumber(ctx)
	if err != nil {
		c.recordFailure()
		return 0, err
	}
	c.recordSuccess()
	return n, nil
}

// BlockByNumber returns block hash and parent hash.
func (c *Client) BlockByNumber(ctx context.Context, number uint64) (hash, parentHash string, err error) {
	block, err := c.rpc.BlockByNumber(ctx, big.NewInt(int64(number)))
	if err != nil {
		c.recordFailure()
		return "", "", err
	}
	c.recordSuccess()
	return block.Hash().Hex(), block.ParentHash().Hex(), nil
}

// Close closes RPC connection.
func (c *Client) Close() {
	c.rpc.Close()
}

// EventTopics for ExchangeVault events.
var (
	TopicDepositETH   = crypto.Keccak256Hash([]byte("DepositETH(address,uint256,bytes32)"))
	TopicDepositERC20 = crypto.Keccak256Hash([]byte("DepositERC20(address,address,uint256,bytes32)"))
	TopicWithdrawETH  = crypto.Keccak256Hash([]byte("WithdrawETH(address,uint256,bytes32)"))
)

// ClassifyEvent returns event name from log topics.
func ClassifyEvent(lg types.Log) string {
	if len(lg.Topics) == 0 {
		return ""
	}
	switch lg.Topics[0] {
	case TopicDepositETH:
		return "DepositETH"
	case TopicDepositERC20:
		return "DepositERC20"
	case TopicWithdrawETH:
		return "WithdrawETH"
	default:
		return ""
	}
}

// ParseDepositETH extracts user and amount from DepositETH log.
func ParseDepositETH(lg types.Log) (user common.Address, amount *big.Int) {
	if len(lg.Topics) < 2 {
		return common.Address{}, big.NewInt(0)
	}
	user = common.BytesToAddress(lg.Topics[1].Bytes()[12:])
	amount = big.NewInt(0)
	if len(lg.Data) >= 32 {
		amount = new(big.Int).SetBytes(lg.Data[:32])
	}
	return user, amount
}

// ParseDepositERC20 extracts token, user and amount.
func ParseDepositERC20(lg types.Log) (token, user common.Address, amount *big.Int) {
	if len(lg.Topics) < 3 {
		return common.Address{}, common.Address{}, big.NewInt(0)
	}
	token = common.BytesToAddress(lg.Topics[1].Bytes()[12:])
	user = common.BytesToAddress(lg.Topics[2].Bytes()[12:])
	amount = big.NewInt(0)
	if len(lg.Data) >= 32 {
		amount = new(big.Int).SetBytes(lg.Data[:32])
	}
	return token, user, amount
}
