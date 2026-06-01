package ethereum

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// NonceManager tracks pending nonces for the hot wallet.
type NonceManager struct {
	mu    sync.Mutex
	nonce uint64
}

// TxSender broadcasts Vault withdrawal transactions (local/testnet only).
type TxSender struct {
	client        *Client
	privateKey    *ecdsa.PrivateKey
	from          common.Address
	vaultAddress  common.Address
	chainID       *big.Int
	enableSigning bool
	nonceMgr      NonceManager
}

// NewTxSender creates a transaction sender guarded by enableSigning flag.
func NewTxSender(client *Client, privateKeyHex, vaultAddress string, enableSigning bool) (*TxSender, error) {
	if !enableSigning {
		return &TxSender{client: client, enableSigning: false}, nil
	}
	keyHex := strings.TrimPrefix(privateKeyHex, "0x")
	key, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	return &TxSender{
		client:        client,
		privateKey:    key,
		from:          from,
		vaultAddress:  common.HexToAddress(vaultAddress),
		chainID:       big.NewInt(client.chainID),
		enableSigning: true,
	}, nil
}

// SendVaultWithdrawETH builds and sends ETH withdrawal (stub - requires contract binding for production path).
func (s *TxSender) SendVaultWithdrawETH(ctx context.Context, to string, amountWei string, withdrawalID [32]byte) (string, error) {
	if !s.enableSigning {
		return "", fmt.Errorf("signing disabled")
	}
	// Lab mode: return simulated tx hash when contract bindings not deployed
	_ = to
	_ = amountWei
	_ = withdrawalID
	return "0x" + crypto.Keccak256Hash(withdrawalID[:]).Hex()[2:], nil
}

// SendVaultWithdrawERC20 sends ERC20 withdrawal (stub).
func (s *TxSender) SendVaultWithdrawERC20(ctx context.Context, token, to string, amount string, withdrawalID [32]byte) (string, error) {
	if !s.enableSigning {
		return "", fmt.Errorf("signing disabled")
	}
	_ = token
	_ = to
	_ = amount
	return "0x" + crypto.Keccak256Hash(withdrawalID[:]).Hex()[2:], nil
}

func (s *TxSender) nextNonce(ctx context.Context, rpc *ethclient.Client) (uint64, error) {
	s.nonceMgr.mu.Lock()
	defer s.nonceMgr.mu.Unlock()
	if s.nonceMgr.nonce == 0 {
		n, err := rpc.PendingNonceAt(ctx, s.from)
		if err != nil {
			return 0, err
		}
		s.nonceMgr.nonce = n
	}
	n := s.nonceMgr.nonce
	s.nonceMgr.nonce++
	return n, nil
}

func (s *TxSender) signAndSend(ctx context.Context, tx *types.Transaction) (string, error) {
	signed, err := types.SignTx(tx, types.NewEIP155Signer(s.chainID), s.privateKey)
	if err != nil {
		return "", err
	}
	if err := s.client.rpc.SendTransaction(ctx, signed); err != nil {
		return "", err
	}
	return signed.Hash().Hex(), nil
}

// TransactOpts returns bind transactor options for contract calls.
func (s *TxSender) TransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	if !s.enableSigning {
		return nil, fmt.Errorf("signing disabled")
	}
	nonce, err := s.nextNonce(ctx, s.client.rpc)
	if err != nil {
		return nil, err
	}
	return &bind.TransactOpts{
		From:     s.from,
		Nonce:    big.NewInt(int64(nonce)),
		Signer:   func(addr common.Address, tx *types.Transaction) (*types.Transaction, error) { return types.SignTx(tx, types.NewEIP155Signer(s.chainID), s.privateKey) },
		Context:  ctx,
		GasLimit: 300000,
	}, nil
}
