package handler

import (
	"fmt"
	"net/http"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/blockchain"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/gin-gonic/gin"
)

type BlockchainHandler struct {
	swarmRepo *postgres.SwarmRepository
	bcService *blockchain.AuditTrailService
}

func NewBlockchainHandler(swarmRepo *postgres.SwarmRepository, bcService *blockchain.AuditTrailService) *BlockchainHandler {
	return &BlockchainHandler{
		swarmRepo: swarmRepo,
		bcService: bcService,
	}
}

func (h *BlockchainHandler) GetStatus(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	task, err := h.swarmRepo.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Swarm task not found: %v", err)})
		return
	}

	// Determine blockchain status
	status := task.BlockchainStat
	if status == "" {
		status = "UNCOMMITTED"
	}

	network := task.BlockchainNet
	if network == "" && h.bcService != nil {
		// Fallback if not recorded in task
		// But let's check what network name is in the service
		// Since Network field in AuditTrailService is not exported directly, we just return task's value or Sepolia
		network = "Sepolia"
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"taskId":             task.ID,
			"blockchainStatus":   status,
			"blockchainTx":       task.BlockchainTx,
			"blockchainNetwork":  network,
			"rationaleHash":      task.RationaleHash,
			"consensusHash":      task.ConsensusHash,
			"updatedAt":          task.UpdatedAt,
		},
	})
}

func (h *BlockchainHandler) Verify(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	task, err := h.swarmRepo.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Swarm task not found: %v", err)})
		return
	}

	if h.bcService == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"verified":             false,
				"onChainRationaleHash": "",
				"onChainConsensusHash": "",
				"localRationaleHash":   task.RationaleHash,
				"localConsensusHash":   task.ConsensusHash,
				"blockNumber":          "0",
				"timestamp":            "0",
				"owner":                "",
				"error":                "Blockchain service not enabled or configured on backend",
			},
		})
		return
	}

	if task.RationaleHash == "" || task.ConsensusHash == "" {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"verified":             false,
				"onChainRationaleHash": "",
				"onChainConsensusHash": "",
				"localRationaleHash":   "",
				"localConsensusHash":   "",
				"blockNumber":          "0",
				"timestamp":            "0",
				"owner":                "",
				"error":                "Local task hashes are not generated yet (task may be incomplete)",
			},
		})
		return
	}

	// 1. Verify Hashes via Smart Contract
	verified, err := h.bcService.VerifyHashes(c.Request.Context(), task.ID, task.RationaleHash, task.ConsensusHash)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"verified":             false,
				"onChainRationaleHash": "",
				"onChainConsensusHash": "",
				"localRationaleHash":   task.RationaleHash,
				"localConsensusHash":   task.ConsensusHash,
				"blockNumber":          "0",
				"timestamp":            "0",
				"owner":                "",
				"error":                fmt.Sprintf("Failed to call verifyHashes contract method: %v", err),
			},
		})
		return
	}

	// 2. Fetch Active Log details from Smart Contract
	logEntry, err := h.bcService.GetActiveLog(c.Request.Context(), task.ID)
	if err != nil {
		// Log might not exist on-chain
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"verified":             verified,
				"onChainRationaleHash": "",
				"onChainConsensusHash": "",
				"localRationaleHash":   task.RationaleHash,
				"localConsensusHash":   task.ConsensusHash,
				"blockNumber":          "0",
				"timestamp":            "0",
				"owner":                "",
				"error":                fmt.Sprintf("Failed to retrieve active log details: %v", err),
			},
		})
		return
	}

	// Extract details
	onChainRationale, _ := logEntry["rationale_hash"].(string)
	onChainConsensus, _ := logEntry["consensus_hash"].(string)
	submitter, _ := logEntry["submitter"].(string)
	blockNum := fmt.Sprintf("%v", logEntry["block_number"])
	timestamp := fmt.Sprintf("%v", logEntry["timestamp"])

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"verified":             verified,
			"onChainRationaleHash": onChainRationale,
			"onChainConsensusHash": onChainConsensus,
			"localRationaleHash":   "0x" + task.RationaleHash,
			"localConsensusHash":   "0x" + task.ConsensusHash,
			"blockNumber":          blockNum,
			"timestamp":            timestamp,
			"owner":                submitter,
		},
	})
}
