package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Document struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	Title          string         `gorm:"type:varchar(255);not null" json:"title"`
	SourceURI      string         `gorm:"type:text" json:"source_uri"`                      // S3 Key
	Status         string         `gorm:"type:varchar(50);default:'pending'" json:"status"` // pending, processing, ready, failed
	AiAnalysisJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"ai_analysis_json"`
	CreatedAt      time.Time      `json:"created_at"`
	LastUpdatedAt  time.Time      `json:"last_updated_at"`
}

type DocumentChunk struct {
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID   uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"` // Untuk pre-filtering pgvector
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Embedding  []float32 `gorm:"type:vector(1536)" json:"-"` // pgvector integration
	ChunkIndex int       `gorm:"not null" json:"chunk_index"`
}

// DocumentRepository defines persistence operations for documents.
type DocumentRepository interface {
	Create(ctx context.Context, doc *Document) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, metadata map[string]interface{}) error
	StoreChunks(ctx context.Context, chunks []DocumentChunk) error
	FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*Document, int64, error)
	// HybridSearch performs tenant-scoped semantic (pgvector HNSW) + lexical (FTS) search
	// and fuses results using Reciprocal Rank Fusion (RRF).
	HybridSearch(ctx context.Context, params HybridSearchParams) ([]HybridSearchResult, error)
}

// HybridSearchParams carries all inputs for a hybrid RAG search query.
type HybridSearchParams struct {
	TenantID       string    // MANDATORY: all queries must be scoped to a tenant
	QueryText      string    // Raw query from the user
	QueryEmbedding []float32 // Query vector from Gemini text-embedding-004
	TopK           int       // Number of final results after RRF fusion
	EfSearch       int       // HNSW ef_search parameter (higher = more accurate, slower)
	RRFConstant    int       // RRF constant k (default 60, per the original paper)
}

// HybridSearchResult is a single fused result with lineage back to its source chunk and document.
type HybridSearchResult struct {
	DocumentID    uuid.UUID `json:"document_id"`
	DocumentTitle string    `json:"document_title"`
	ChunkID       uuid.UUID `json:"chunk_id"`
	Content       string    `json:"content"`
	RRFScore      float64   `json:"rrf_score"`
	VectorRank    int       `json:"vector_rank"`
	FTSRank       int       `json:"fts_rank"`
}

// DocumentUsecase defines business logic for document lifecycle.
type DocumentUsecase interface {
	GetUploadURL(ctx context.Context, tenantID, userID uuid.UUID, fileName string) (presignedURL string, objectKey string, err error)
	ConfirmUpload(ctx context.Context, tenantID, userID uuid.UUID, title, objectKey string) (*Document, error)
	ListDocuments(ctx context.Context, tenantID string, limit, offset int) ([]*Document, int64, error)
}
