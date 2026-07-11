package blockchain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// AuditTrailABI is the ABI for the AuditTrail contract
const AuditTrailABI = `[
	{"inputs":[],"stateMutability":"nonpayable","type":"constructor"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"oldIndex","type":"uint256"},{"indexed":true,"internalType":"uint256","name":"newIndex","type":"uint256"},{"indexed":false,"internalType":"string","name":"taskId","type":"string"}],"name":"LogCorrected","type":"event"},
	{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"index","type":"uint256"},{"indexed":false,"internalType":"string","name":"taskId","type":"string"},{"indexed":false,"internalType":"bytes32","name":"rationaleHash","type":"bytes32"},{"indexed":false,"internalType":"bytes32","name":"consensusHash","type":"bytes32"}],"name":"LogInserted","type":"event"},
	{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"authorizedSubmitters","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"internalType":"string","name":"taskId","type":"string"},{"internalType":"bytes32","name":"rationaleHash","type":"bytes32"},{"internalType":"bytes32","name":"consensusHash","type":"bytes32"}],"name":"insertLog","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
	{"inputs":[{"internalType":"string[]","name":"_taskIds","type":"string[]"},{"internalType":"bytes32[]","name":"_rationaleHashes","type":"bytes32[]"},{"internalType":"bytes32[]","name":"_consensusHashes","type":"bytes32[]"}],"name":"insertLogBatch","outputs":[],"stateMutability":"nonpayable","type":"function"},
	{"inputs":[{"internalType":"string","name":"oldTaskId","type":"string"},{"internalType":"bytes32","name":"rationaleHash","type":"bytes32"},{"internalType":"bytes32","name":"consensusHash","type":"bytes32"}],"name":"correctLog","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
	{"inputs":[{"internalType":"string","name":"taskId","type":"string"}],"name":"getActiveLog","outputs":[{"components":[{"internalType":"bytes32","name":"rationaleHash","type":"bytes32"},{"internalType":"bytes32","name":"consensusHash","type":"bytes32"},{"internalType":"address","name":"submitter","type":"address"},{"internalType":"uint256","name":"timestamp","type":"uint256"},{"internalType":"uint256","name":"blockNumber","type":"uint256"},{"internalType":"uint8","name":"status","type":"uint8"},{"internalType":"uint256","name":"supersededBy","type":"uint256"}],"internalType":"struct AuditTrail.LogEntry","name":"","type":"tuple"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"internalType":"string","name":"taskId","type":"string"}],"name":"getTaskHistory","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"internalType":"string","name":"taskId","type":"string"},{"internalType":"bytes32","name":"rationaleHash","type":"bytes32"},{"internalType":"bytes32","name":"consensusHash","type":"bytes32"}],"name":"verifyHashes","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"internalType":"address","name":"submitter","type":"address"}],"name":"authorizeSubmitter","outputs":[],"stateMutability":"nonpayable","type":"function"}
]`

// AuditTrailService wraps the blockchain interaction
type AuditTrailService struct {
	client     *ethclient.Client
	contract   common.Address
	privateKey *ecdsa.PrivateKey
	abi        abi.ABI
	network    string
}

// NewAuditTrailService creates a new blockchain service
func NewAuditTrailService(rpcURL, contractAddr, privateKeyHex, network string) (*AuditTrailService, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blockchain: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(AuditTrailABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	var pk *ecdsa.PrivateKey
	if privateKeyHex != "" {
		pk, err = crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	return &AuditTrailService{
		client:     client,
		contract:   common.HexToAddress(contractAddr),
		privateKey: pk,
		abi:        parsedABI,
		network:    network,
	}, nil
}

// InsertLog submits a new audit log to the blockchain
func (s *AuditTrailService) InsertLog(ctx context.Context, taskID, rationaleHash, consensusHash string) (string, error) {
	if s.privateKey == nil {
		return "", fmt.Errorf("private key not configured")
	}

	data, err := s.abi.Pack("insertLog", taskID, common.HexToHash(rationaleHash), common.HexToHash(consensusHash))
	if err != nil {
		return "", fmt.Errorf("failed to pack insertLog: %w", err)
	}

	txHash, err := s.sendTransaction(ctx, data)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return txHash, nil
}

// InsertLogBatch submits multiple audit logs in a single transaction
func (s *AuditTrailService) InsertLogBatch(ctx context.Context, taskIDs []string, rationaleHashes []string, consensusHashes []string) (string, error) {
	if s.privateKey == nil {
		return "", fmt.Errorf("private key not configured")
	}

	if len(taskIDs) == 0 {
		return "", fmt.Errorf("empty batch")
	}

	var ratHashes [][32]byte
	for _, h := range rationaleHashes {
		ratHashes = append(ratHashes, common.HexToHash(h))
	}

	var conHashes [][32]byte
	for _, h := range consensusHashes {
		conHashes = append(conHashes, common.HexToHash(h))
	}

	data, err := s.abi.Pack("insertLogBatch", taskIDs, ratHashes, conHashes)
	if err != nil {
		return "", fmt.Errorf("failed to pack insertLogBatch: %w", err)
	}

	txHash, err := s.sendTransaction(ctx, data)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return txHash, nil
}

// VerifyHashes checks if the given hashes match the stored log
func (s *AuditTrailService) VerifyHashes(ctx context.Context, taskID, rationaleHash, consensusHash string) (bool, error) {
	data, err := s.abi.Pack("verifyHashes", taskID, common.HexToHash(rationaleHash), common.HexToHash(consensusHash))
	if err != nil {
		return false, fmt.Errorf("failed to pack verifyHashes: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &s.contract,
		Data: data,
	}

	result, err := s.client.CallContract(ctx, msg, nil)
	if err != nil {
		return false, fmt.Errorf("failed to call contract: %w", err)
	}

	var verified bool
	if err := s.abi.UnpackIntoInterface(&verified, "verifyHashes", result); err != nil {
		return false, fmt.Errorf("failed to unpack result: %w", err)
	}

	return verified, nil
}

// GetActiveLog retrieves the active log for a task
func (s *AuditTrailService) GetActiveLog(ctx context.Context, taskID string) (map[string]interface{}, error) {
	data, err := s.abi.Pack("getActiveLog", taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to pack getActiveLog: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &s.contract,
		Data: data,
	}

	result, err := s.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %w", err)
	}

	// Unpack tuple result
	var unpacked struct {
		RationaleHash [32]byte
		ConsensusHash [32]byte
		Submitter     common.Address
		Timestamp     *big.Int
		BlockNumber   *big.Int
		Status        uint8
		SupersededBy  *big.Int
	}

	if err := s.abi.UnpackIntoInterface(&unpacked, "getActiveLog", result); err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	return map[string]interface{}{
		"rationale_hash": fmt.Sprintf("0x%x", unpacked.RationaleHash),
		"consensus_hash": fmt.Sprintf("0x%x", unpacked.ConsensusHash),
		"submitter":      unpacked.Submitter.Hex(),
		"timestamp":      unpacked.Timestamp.Int64(),
		"block_number":   unpacked.BlockNumber.Int64(),
		"status":         unpacked.Status,
		"superseded_by":  unpacked.SupersededBy.Int64(),
	}, nil
}

// sendTransaction builds and sends a raw transaction using EIP-1559 dynamic fee pricing with exponential backoff retries and gas fallbacks
func (s *AuditTrailService) sendTransaction(ctx context.Context, data []byte) (string, error) {
	publicKey := s.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("failed to cast public key")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	var lastErr error
	backoff := 1 * time.Second
	maxRetries := 4

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[Blockchain] Send transaction attempt %d failed: %v. Retrying in %v...", attempt, lastErr, backoff)
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retries: %w", ctx.Err())
			case <-time.After(backoff):
				backoff *= 2 // exponential backoff
			}
		}

		nonce, err := s.client.PendingNonceAt(ctx, fromAddress)
		if err != nil {
			lastErr = fmt.Errorf("failed to get nonce: %w", err)
			continue
		}

		// 1. Try to get EIP-1559 gas tip suggestion
		tip, err := s.client.SuggestGasTipCap(ctx)
		if err != nil {
			log.Printf("[Blockchain] SuggestGasTipCap failed: %v. Falling back to SuggestGasPrice.", err)
			// Fallback to legacy SuggestGasPrice
			gasPrice, errLegacy := s.client.SuggestGasPrice(ctx)
			if errLegacy != nil {
				lastErr = fmt.Errorf("failed to suggest gas price: %w", errLegacy)
				continue
			}

			gasLimit, errEstimate := s.client.EstimateGas(ctx, ethereum.CallMsg{
				From: fromAddress,
				To:   &s.contract,
				Data: data,
			})
			if errEstimate != nil {
				gasLimit = uint64(150000)
			} else {
				gasLimit = gasLimit * uint64(120+attempt*10) / 100
			}

			// Add 20% congestion fee buffer per retry
			bufferMultiplier := new(big.Int).SetInt64(100 + int64(attempt*20))
			adjustedGasPrice := new(big.Int).Div(new(big.Int).Mul(gasPrice, bufferMultiplier), big.NewInt(100))

			chainID, errChain := s.client.NetworkID(ctx)
			if errChain != nil {
				lastErr = fmt.Errorf("failed to get chain ID: %w", errChain)
				continue
			}

			txData := types.NewTx(&types.LegacyTx{
				Nonce:    nonce,
				GasPrice: adjustedGasPrice,
				Gas:      gasLimit,
				To:       &s.contract,
				Value:    big.NewInt(0),
				Data:     data,
			})

			signedTx, errSign := types.SignTx(txData, types.LatestSignerForChainID(chainID), s.privateKey)
			if errSign != nil {
				lastErr = fmt.Errorf("failed to sign legacy tx: %w", errSign)
				continue
			}

			if errSend := s.client.SendTransaction(ctx, signedTx); errSend != nil {
				lastErr = fmt.Errorf("failed to send legacy transaction: %w", errSend)
				continue
			}

			return signedTx.Hash().Hex(), nil
		}

		// EIP-1559 dynamic pricing logic
		// Add safety margin + attempt-based buffer for retries (e.g. +30% gas tip cap per retry)
		tipBufferMultiplier := new(big.Int).SetInt64(150 + int64(attempt*30))
		tipBuffer := new(big.Int).Div(new(big.Int).Mul(tip, tipBufferMultiplier), big.NewInt(100))
		
		// Ensure a minimum tip cap of 1.5 Gwei
		minTip := big.NewInt(1500000000)
		if tipBuffer.Cmp(minTip) < 0 {
			tipBuffer = minTip
		}

		header, err := s.client.HeaderByNumber(ctx, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to get latest block header: %w", err)
			continue
		}
		if header.BaseFee == nil {
			lastErr = fmt.Errorf("base fee is nil in block header")
			continue
		}

		// MaxFeePerGas = (BaseFee * 2) + Tip
		maxFeePerGas := new(big.Int).Add(
			new(big.Int).Mul(header.BaseFee, big.NewInt(2)),
			tipBuffer,
		)

		gasLimit, err := s.client.EstimateGas(ctx, ethereum.CallMsg{
			From: fromAddress,
			To:   &s.contract,
			Data: data,
		})
		if err != nil {
			log.Printf("[Blockchain] Gas estimation failed: %v. Using fallback of 150,000.", err)
			gasLimit = uint64(150000)
		} else {
			// Add 20% safety margin + extra 10% per attempt
			gasLimit = gasLimit * uint64(120+attempt*10) / 100
		}

		chainID, err := s.client.NetworkID(ctx)
		if err != nil {
			lastErr = fmt.Errorf("failed to get chain ID: %w", err)
			continue
		}

		txData := &types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: tipBuffer,
			GasFeeCap: maxFeePerGas,
			Gas:       gasLimit,
			To:        &s.contract,
			Value:     big.NewInt(0),
			Data:      data,
		}
		tx := types.NewTx(txData)

		signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), s.privateKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to sign transaction: %w", err)
			continue
		}

		if err := s.client.SendTransaction(ctx, signedTx); err != nil {
			lastErr = fmt.Errorf("failed to send transaction: %w", err)
			continue
		}

		return signedTx.Hash().Hex(), nil
	}

	return "", fmt.Errorf("transaction execution failed after %d attempts. Last error: %w", maxRetries, lastErr)
}

// WaitForConfirmation waits for a transaction to be mined
func (s *AuditTrailService) WaitForConfirmation(ctx context.Context, txHash string, timeout time.Duration) (*types.Receipt, error) {
	hash := common.HexToHash(txHash)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		receipt, err := s.client.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		if err != ethereum.NotFound {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for confirmation")
		case <-time.After(2 * time.Second):
			continue
		}
	}
}

// Close closes the blockchain client connection
func (s *AuditTrailService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}
