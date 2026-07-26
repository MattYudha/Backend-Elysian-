package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/blockchain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/mq"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type SwarmUsecase struct {
	swarmRepo         *postgres.SwarmRepository
	overrideRepo      repository.OverrideRepository
	redis             cache.Cache
	blockchainService *blockchain.AuditTrailService
	mqClient          mq.TaskQueue
	nftService        *blockchain.NFTService
}

func NewSwarmUsecase(
	swarmRepo *postgres.SwarmRepository,
	overrideRepo repository.OverrideRepository,
	redis cache.Cache,
	bcService *blockchain.AuditTrailService,
	mqClient mq.TaskQueue,
	nftService *blockchain.NFTService,
) *SwarmUsecase {
	return &SwarmUsecase{
		swarmRepo:         swarmRepo,
		overrideRepo:      overrideRepo,
		redis:             redis,
		blockchainService: bcService,
		mqClient:          mqClient,
		nftService:        nftService,
	}
}

func (u *SwarmUsecase) TriggerSwarm(ctx context.Context, documentID string, items []map[string]interface{}, tenantIDStr string, userIDStr string) (*domain.SwarmTask, error) {
	var finalDocID string = documentID
	var targetDocUUID uuid.UUID

	tenantUUID, _ := uuid.Parse(tenantIDStr)
	userUUID, _ := uuid.Parse(userIDStr)

	if parsedUUID, err := uuid.Parse(documentID); err == nil {
		targetDocUUID = parsedUUID
		finalDocID = parsedUUID.String()
	} else {
		// Document ID is not a valid UUID (e.g. "draft-1" or "audit-001")
		// Generate deterministic draft UUID based on tenant ID and original identifier
		targetDocUUID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("draft-"+tenantIDStr+"-"+documentID))
		finalDocID = targetDocUUID.String()
	}

	// Verify or create the document in the database to satisfy the foreign key constraint
	db := u.swarmRepo.GetDB()
	var count int64
	err := db.WithContext(ctx).Table("documents").Where("id = ?", targetDocUUID).Count(&count).Error
	if err != nil {
		return nil, fmt.Errorf("failed to check existing document: %w", err)
	}

	if count == 0 {
		draftDoc := &domain.Document{
			ID:            targetDocUUID,
			TenantID:      tenantUUID,
			UserID:        userUUID,
			Title:         "Document (" + documentID + ")",
			Category:      "general",
			Status:        "draft",
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		}
		err = db.WithContext(ctx).Table("documents").Create(draftDoc).Error
		if err != nil {
			return nil, fmt.Errorf("failed to auto-create document record: %w", err)
		}
	}

	// 1. Create Task in DB
	task := &domain.SwarmTask{
		DocumentID: finalDocID,
		Status:     "PENDING",
	}

	if err := u.swarmRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create swarm task: %w", err)
	}

	// 2. Fetch overrides and inject them if present
	for _, item := range items {
		var itemName string
		if nameVal, ok := item["item_name"].(string); ok {
			itemName = nameVal
		} else if nameVal, ok := item["name"].(string); ok {
			itemName = nameVal
		}
		if itemName != "" && u.overrideRepo != nil {
			override, err := u.overrideRepo.GetByItemName(ctx, tenantIDStr, itemName)
			if err == nil && override != nil {
				item["override"] = map[string]interface{}{
					"original_verdict": override.OriginalVerdict,
					"new_verdict":      override.NewVerdict,
					"justification":    override.Justification,
				}
			}
		}
	}

	// 2. Prepare Payload
	payload := domain.SwarmPayload{
		TaskID:       task.ID,
		DocumentID:   finalDocID,
		DocumentType: "RAPBD",
		Items:        items,
		WebhookURL:   "http://localhost:7777/api/v1/swarm/callback",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 3. LPUSH to Redis
	if redisCache, ok := u.redis.(*cache.RedisCache); ok {
		err = redisCache.GetClient().LPush(ctx, "swarm:tasks", payloadBytes).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to publish to redis: %w", err)
		}
	} else {
		return nil, fmt.Errorf("cache is not redis")
	}

	return task, nil
}

func (u *SwarmUsecase) SaveOverride(ctx context.Context, override *domain.AuditorOverride) error {
	if u.overrideRepo == nil {
		return fmt.Errorf("override repository is not initialized")
	}
	return u.overrideRepo.Save(ctx, override)
}

func (u *SwarmUsecase) HandleCallback(ctx context.Context, callback domain.SwarmCallback) error {
	task, err := u.swarmRepo.GetByID(ctx, callback.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	task.Status = callback.Status
	task.Summary = callback.Summary
	task.RationaleHash = callback.Hashes.RationaleHash
	task.ConsensusHash = callback.Hashes.ConsensusHash
	task.BlockchainNet = callback.Blockchain.Network
	task.BlockchainStat = callback.Blockchain.Status

	resultsBytes, _ := json.Marshal(callback.Results)
	task.Results = datatypes.JSON(resultsBytes)
	task.UpdatedAt = time.Now()

	if err := u.swarmRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Insert token usage record if provided in callback
	if callback.TokenUsage != nil {
		var doc struct {
			TenantID uuid.UUID
		}
		db := u.swarmRepo.GetDB()
		if err := db.Table("documents").Select("tenant_id").Where("id = ?", task.DocumentID).Scan(&doc).Error; err == nil {
			totalTokens := callback.TokenUsage.PromptTokens + callback.TokenUsage.CompletionTokens
			cost := float64(totalTokens) * 0.00000015
			
			ledger := map[string]interface{}{
				"id":                uuid.New(),
				"tenant_id":         doc.TenantID,
				"model":             callback.TokenUsage.Model,
				"prompt_tokens":     callback.TokenUsage.PromptTokens,
				"completion_tokens": callback.TokenUsage.CompletionTokens,
				"cost":              cost,
				"created_at":        time.Now().UTC(),
			}
			if err := db.Table("token_usage_ledgers").Create(&ledger).Error; err != nil {
				log.Printf("[Swarm Callback] Failed to log token usage to ledger: %v", err)
			} else {
				log.Printf("[Swarm Callback] Logged %d tokens to ledger for Tenant %s", totalTokens, doc.TenantID.String())
			}
		} else {
			log.Printf("[Swarm Callback] Failed to fetch document tenant_id for logging token usage: %v", err)
		}
	}

	// Publish to Redis PubSub for SSE streaming
	if redisCache, ok := u.redis.(*cache.RedisCache); ok {
		ssePayload := map[string]interface{}{
			"task_id":    task.ID,
			"status":     task.Status,
			"results":    callback.Results,
			"blockchain": map[string]interface{}{
				"tx_hash": task.BlockchainTx,
				"network": task.BlockchainNet,
				"status":  task.BlockchainStat,
			},
			"timestamp": time.Now().UnixNano() / int64(time.Millisecond),
		}
		sseBytes, _ := json.Marshal(ssePayload)
		redisCache.GetClient().Publish(ctx, "swarm:events", sseBytes)
	}

	// Step 5 — Push hash to blockchain asynchronously via Asynq queue
	if u.blockchainService != nil && task.RationaleHash != "" && task.ConsensusHash != "" {
		asynqTask, err := NewCommitSwarmToBlockchainTask(task.ID, task.RationaleHash, task.ConsensusHash)
		if err != nil {
			log.Printf("[Swarm] Failed to create blockchain commit task for task %s: %v", task.ID, err)
		} else {
			if _, err := u.mqClient.EnqueueTask(asynqTask); err != nil {
				log.Printf("[Swarm] Failed to enqueue blockchain commit task for task %s: %v. Running direct blockchain commit fallback.", task.ID, err)
				u.CommitToBlockchainDirect(task.ID, task.RationaleHash, task.ConsensusHash)
			} else {
				log.Printf("[Swarm] Successfully enqueued blockchain commit task for task %s", task.ID)
			}
		}
	}

	return nil
}

// CommitToBlockchainDirect commits the task consensus hashes to blockchain directly in a background goroutine
func (u *SwarmUsecase) CommitToBlockchainDirect(taskID, rationaleHash, consensusHash string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		log.Printf("[Swarm-Direct] ▶ Direct commit to Sepolia for Swarm Task %s", taskID)
		u.updateBlockchainStatus(ctx, taskID, "", "PENDING_COMMIT")

		txHash, err := u.blockchainService.InsertLog(ctx, taskID, rationaleHash, consensusHash)
		if err != nil {
			log.Printf("[Swarm-Direct] ❌ insertLog failed for task %s: %v", taskID, err)
			u.updateBlockchainStatus(ctx, taskID, "", "FAILED")
			u.publishBlockchainUpdate(ctx, taskID, "", "FAILED")
			return
		}

		log.Printf("[Swarm-Direct] 📨 insertLog submitted tx %s. Waiting for confirmation...", txHash)
		u.updateBlockchainStatus(ctx, taskID, txHash, "PENDING_CONFIRMATION")
		u.publishBlockchainUpdate(ctx, taskID, txHash, "PENDING_CONFIRMATION")

		receipt, err := u.blockchainService.WaitForConfirmation(ctx, txHash, 120*time.Second)
		if err != nil {
			log.Printf("[Swarm-Direct] ⚠️ Wait for confirmation timed out for task %s, tx: %s", taskID, txHash)
			u.updateBlockchainStatus(ctx, taskID, txHash, "FAILED")
			u.publishBlockchainUpdate(ctx, taskID, txHash, "FAILED")
			return
		}

		if receipt.Status == 1 {
			log.Printf("[Swarm-Direct] ✅ Tx confirmed for task %s in block %d", taskID, receipt.BlockNumber)
			u.updateBlockchainStatus(ctx, taskID, txHash, "VERIFIED")
			u.publishBlockchainUpdate(ctx, taskID, txHash, "VERIFIED")
			// Automatically mint digital audit certificate NFT
			go u.MintDigitalCertificate(context.Background(), taskID)
		} else {
			log.Printf("[Swarm-Direct] ❌ Tx execution failed on-chain for task %s", taskID)
			u.updateBlockchainStatus(ctx, taskID, txHash, "FAILED")
			u.publishBlockchainUpdate(ctx, taskID, txHash, "FAILED")
		}
	}()
}

// publishBlockchainUpdate broadcasts the blockchain status update to Redis PubSub for live SSE delivery
func (u *SwarmUsecase) publishBlockchainUpdate(ctx context.Context, taskID, txHash, status string) {
	task, err := u.swarmRepo.GetByID(ctx, taskID)
	if err != nil {
		return
	}

	if redisCache, ok := u.redis.(*cache.RedisCache); ok {
		var results interface{}
		if len(task.Results) > 0 {
			_ = json.Unmarshal(task.Results, &results)
		}
		ssePayload := map[string]interface{}{
			"task_id":    task.ID,
			"status":     task.Status,
			"results":    results,
			"blockchain": map[string]interface{}{
				"tx_hash": txHash,
				"network": task.BlockchainNet,
				"status":  status,
			},
			"nft_token_id":  task.NFTTokenID,
			"ipfs_cid":      task.IPFSCID,
			"nft_tx_hash":   task.NFTTxHash,
			"timestamp":     time.Now().UnixNano() / int64(time.Millisecond),
		}
		sseBytes, _ := json.Marshal(ssePayload)
		redisCache.GetClient().Publish(ctx, "swarm:events", sseBytes)
	}
}

func (u *SwarmUsecase) updateBlockchainStatus(ctx context.Context, taskID, txHash, status string) {
	task, err := u.swarmRepo.GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[Blockchain] failed to get task %s for status update: %v", taskID, err)
		return
	}

	if txHash != "" {
		task.BlockchainTx = txHash
	}
	task.BlockchainStat = status
	task.UpdatedAt = time.Now()

	if err := u.swarmRepo.Update(ctx, task); err != nil {
		log.Printf("[Blockchain] failed to update task %s status: %v", taskID, err)
	}
}

func (u *SwarmUsecase) GetSwarmTask(ctx context.Context, id string) (*domain.SwarmTask, error) {
	return u.swarmRepo.GetByID(ctx, id)
}

func (u *SwarmUsecase) ListSwarmTasks(ctx context.Context, tenantID string, documentID string, limit, offset int) ([]*domain.SwarmTask, int64, error) {
	var finalDocID string = documentID
	if documentID != "" {
		if _, err := uuid.Parse(documentID); err != nil {
			// Document ID is not a valid UUID (e.g. "draft-1")
			// Generate deterministic draft UUID based on the tenant ID and the original identifier
			draftUUID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("draft-"+tenantID+"-"+documentID))
			finalDocID = draftUUID.String()
		}
	}
	return u.swarmRepo.ListByTenant(ctx, tenantID, finalDocID, limit, offset)
}

func (u *SwarmUsecase) MintDigitalCertificate(ctx context.Context, taskID string) {
	if u.nftService == nil {
		log.Printf("[NFT] NFT Service is not enabled or initialized. Skipping NFT Certificate minting.")
		return
	}

	task, err := u.swarmRepo.GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[NFT] ❌ Failed to fetch swarm task %s for NFT minting: %v", taskID, err)
		return
	}

	log.Printf("[NFT] 🏅 Starting NFT Digital Certificate minting for Swarm Task %s", taskID)

	// 1. Fetch original document details
	db := u.swarmRepo.GetDB()
	var doc struct {
		Title string
	}
	err = db.WithContext(ctx).Table("documents").Select("title").Where("id = ?", task.DocumentID).First(&doc).Error
	if err != nil {
		log.Printf("[NFT] ⚠️ Failed to fetch document title for task %s, using fallback: %v", taskID, err)
		doc.Title = "Draft Proposal RAPBD"
	}

	// 2. Count number of issues/temuans
	var resultsArray []map[string]interface{}
	if len(task.Results) > 0 {
		_ = json.Unmarshal(task.Results, &resultsArray)
	}
	totalIssues := len(resultsArray)

	// 3. Generate QR Code pointing to the verification page
	qrURL := fmt.Sprintf("http://localhost:3000/dashboard/documents/%s", task.DocumentID)
	qrBytes, err := u.nftService.GenerateQRCode(qrURL)
	if err != nil {
		log.Printf("[NFT] ❌ Failed to generate QR Code: %v", err)
		return
	}

	// 4. Generate Certificate Cover PNG image
	coverBytes, err := u.nftService.GenerateCertificateCover(taskID, doc.Title, totalIssues, qrBytes)
	if err != nil {
		log.Printf("[NFT] ❌ Failed to generate certificate cover image: %v", err)
		return
	}

	// 5. Upload cover image and metadata to IPFS (via Pinata)
	ipfsCID, err := u.nftService.UploadToIPFS(ctx, coverBytes, taskID, doc.Title, totalIssues)
	if err != nil {
		log.Printf("[NFT] ❌ Failed to upload cover to IPFS: %v", err)
		return
	}
	log.Printf("[NFT] 🔗 IPFS Metadata Uploaded successfully. CID: %s", ipfsCID)

	// 6. Mint ERC-721 token
	tokenURI := fmt.Sprintf("ipfs://%s", ipfsCID)
	// We will mint the certificate to the platform owner or auditor.
	// For demo/simplicity, we mint it to the same admin address we are using for the platform wallet.
	recipientAddr := "0x03252339418744A98F03D4ED979dF36Cd75308D4" 
	
	txHash, tokenID, err := u.nftService.MintNFT(ctx, recipientAddr, tokenURI)
	if err != nil {
		log.Printf("[NFT] ❌ safeMint transaction failed: %v", err)
		return
	}

	log.Printf("[NFT] 🎉 Successfully Minted NFT Certificate! TokenID: %s, TxHash: %s", tokenID.String(), txHash)

	// 7. Update database with NFT info
	task.NFTTokenID = tokenID.String()
	task.IPFSCID = ipfsCID
	task.NFTTxHash = txHash
	task.UpdatedAt = time.Now()

	if err := u.swarmRepo.Update(ctx, task); err != nil {
		log.Printf("[NFT] ❌ Failed to update PostgreSQL task with NFT data: %v", err)
		return
	}

	// Broadcast update to the frontend via SSE
	u.publishBlockchainUpdate(ctx, taskID, task.BlockchainTx, task.BlockchainStat)
}


