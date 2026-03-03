package repository

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
)

type DocumentRepository interface {
	Create(ctx context.Context, doc *domain.Document) error
	FindByID(ctx context.Context, id string) (*domain.Document, error)
	FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	CreateChunks(ctx context.Context, chunks []domain.DocumentChunk) error
}
