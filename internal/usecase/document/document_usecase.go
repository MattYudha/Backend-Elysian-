package document

import (
	"context"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/mq"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/storage"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/rag"
	"github.com/google/uuid"
)

type documentUsecase struct {
	repo        domain.DocumentRepository
	s3          *storage.S3Service
	mqClient    mq.TaskQueue
	mongoClient *database.MongoClient
}

func NewDocumentUsecase(repo domain.DocumentRepository, s3 *storage.S3Service, mqClient mq.TaskQueue, mongoClient *database.MongoClient) domain.DocumentUsecase {
	return &documentUsecase{repo: repo, s3: s3, mqClient: mqClient, mongoClient: mongoClient}
}

// GetUploadURL (Step 1: GET /presign)
// Generates a scoped, short-lived presigned URL for direct browser-to-S3 upload.
func (u *documentUsecase) GetUploadURL(ctx context.Context, tenantID, userID uuid.UUID, fileName string) (string, string, error) {
	objectKey := fmt.Sprintf("documents/%s/%s/%s_%s", tenantID.String(), userID.String(), uuid.NewString(), fileName)
	if u.s3 == nil {
		// Mock upload URL for local development/offline testing
		mockURL := fmt.Sprintf("http://localhost:7777/api/v1/documents/mock-upload?key=%s", objectKey)
		return mockURL, objectKey, nil
	}
	url, err := u.s3.PresignPutURL(ctx, objectKey, 15*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("presign failed: %w", err)
	}
	return url, objectKey, nil
}

// ConfirmUpload (Step 3: POST /confirm)
// Creates the DB record and dispatches the parsing task to Asynq.
func (u *documentUsecase) ConfirmUpload(ctx context.Context, tenantID, userID uuid.UUID, title, objectKey string, category string) (*domain.Document, error) {
	doc := &domain.Document{
		TenantID:  tenantID,
		UserID:    userID,
		Title:     title,
		Category:  category,
		SourceURI: objectKey,
		Status:    "pending",
	}

	// 1. Persist the initial document record
	if err := u.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	// 2. Save raw staging record in MongoDB Staging with PENDING_QA status
	stagingDoc := &database.StagingDocument{
		ID:       doc.ID.String(),
		TenantID: tenantID.String(),
		FileName: title,
		RawText:  "",
		Status:   database.StatusPendingQA,
	}
	if err := u.mongoClient.SaveDocument(ctx, stagingDoc); err != nil {
		// Log the warning but don't fail the upload confirmation
		fmt.Printf("[WARN] Failed to write initial skeleton document to MongoDB staging: %v\n", err)
	}

	// 3. Enqueue parsing task (non-blocking)
	task, err := rag.NewParseDocumentTask(doc.ID.String(), tenantID.String(), objectKey, category)
	if err != nil {
		// Mark as failed but return the document ID so frontend can retry
		_ = u.repo.UpdateStatus(ctx, doc.ID, "queued_failed", nil)
		return doc, fmt.Errorf("failed to create parsing task: %w", err)
	}

	if _, err := u.mqClient.EnqueueTask(task); err != nil {
		// Log enqueue failure but don't block the API response
		_ = u.repo.UpdateStatus(ctx, doc.ID, "queued_failed", nil)
		return doc, fmt.Errorf("worker queue unavailable, document saved but not yet processing: %w", err)
	}

	return doc, nil
}

// ListDocuments returns paginated documents for a given tenant.
func (u *documentUsecase) ListDocuments(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error) {
	return u.repo.FindByTenant(ctx, tenantID, limit, offset)
}

// Approve reviews a document that is pending QA, and triggers vector embedding.
func (u *documentUsecase) Approve(ctx context.Context, tenantID, docID uuid.UUID) error {
	// 1. Fetch document from DB
	doc, err := u.repo.FindByID(ctx, docID.String())
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// 2. Verify tenant scope (multi-tenancy check)
	if doc.TenantID != tenantID {
		return fmt.Errorf("unauthorized: document does not belong to your tenant")
	}

	// 3. Verify current status is pending_qa
	if doc.Status != "pending_qa" {
		return fmt.Errorf("cannot approve document: current status is %s, expected pending_qa", doc.Status)
	}

	// 4. Update MongoDB document status to APPROVED
	if err := u.mongoClient.ApproveDocument(ctx, docID.String(), "human_qa_operator"); err != nil {
		// Log but don't block PostgreSQL update in case MongoDB is running in loose sync
		fmt.Printf("[WARN] Failed to mark staging document APPROVED in MongoDB: %v\n", err)
	}

	// 5. Update status to processing
	if err := u.repo.UpdateStatus(ctx, docID, "processing", nil); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// 6. Dispatch embedding task
	task, err := rag.NewEmbedDocumentTask(docID.String(), tenantID.String(), doc.Category)
	if err != nil {
		_ = u.repo.UpdateStatus(ctx, docID, "pending_qa", nil) // rollback status
		return fmt.Errorf("failed to create embedding task: %w", err)
	}

	if _, err := u.mqClient.EnqueueTask(task); err != nil {
		_ = u.repo.UpdateStatus(ctx, docID, "pending_qa", nil) // rollback status
		return fmt.Errorf("failed to enqueue embedding task: %w", err)
	}

	return nil
}

func (u *documentUsecase) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	doc, err := u.repo.FindByID(ctx, docID.String())
	if err != nil {
		return err
	}
	if doc.TenantID != tenantID {
		return fmt.Errorf("unauthorized: document does not belong to your tenant")
	}
	return u.repo.Delete(ctx, tenantID, docID)
}

func (u *documentUsecase) UpdateText(ctx context.Context, tenantID, docID uuid.UUID, text string) error {
	doc, err := u.repo.FindByID(ctx, docID.String())
	if err != nil {
		return err
	}
	if doc.TenantID != tenantID {
		return fmt.Errorf("unauthorized: document does not belong to your tenant")
	}

	// Update raw plain text in MongoDB Staging Area
	if err := u.mongoClient.UpdateText(ctx, docID.String(), text); err != nil {
		return fmt.Errorf("failed to update text in MongoDB staging: %w", err)
	}

	return nil
}
