package document

import (
	"context"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/mq"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/storage"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/rag"
	"github.com/google/uuid"
)

type documentUsecase struct {
	repo     domain.DocumentRepository
	s3       *storage.S3Service
	mqClient *mq.AsynqClient
}

func NewDocumentUsecase(repo domain.DocumentRepository, s3 *storage.S3Service, mqClient *mq.AsynqClient) domain.DocumentUsecase {
	return &documentUsecase{repo: repo, s3: s3, mqClient: mqClient}
}

// GetUploadURL (Step 1: GET /presign)
// Generates a scoped, short-lived presigned URL for direct browser-to-S3 upload.
func (u *documentUsecase) GetUploadURL(ctx context.Context, tenantID, userID uuid.UUID, fileName string) (string, string, error) {
	objectKey := fmt.Sprintf("documents/%s/%s/%s_%s", tenantID.String(), userID.String(), uuid.NewString(), fileName)
	url, err := u.s3.PresignPutURL(ctx, objectKey, 15*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("presign failed: %w", err)
	}
	return url, objectKey, nil
}

// ConfirmUpload (Step 3: POST /confirm)
// Creates the DB record and dispatches the vectorization task to Asynq.
func (u *documentUsecase) ConfirmUpload(ctx context.Context, tenantID, userID uuid.UUID, title, objectKey string) (*domain.Document, error) {
	doc := &domain.Document{
		TenantID:  tenantID,
		UserID:    userID,
		Title:     title,
		SourceURI: objectKey,
		Status:    "pending",
	}

	// 1. Persist the initial document record
	if err := u.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	// 2. Enqueue vectorization task (non-blocking)
	task, err := rag.NewProcessDocumentTask(doc.ID.String(), tenantID.String(), objectKey)
	if err != nil {
		// Mark as failed but return the document ID so frontend can retry
		_ = u.repo.UpdateStatus(ctx, doc.ID, "queued_failed", nil)
		return doc, fmt.Errorf("failed to create vectorization task: %w", err)
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
