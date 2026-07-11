package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type documentRepository struct {
	db *gorm.DB
}

func NewDocumentRepository(db *gorm.DB) domain.DocumentRepository {
	return &documentRepository{db: db}
}

func (r *documentRepository) Create(ctx context.Context, doc *domain.Document) error {
	if err := r.db.WithContext(ctx).Create(doc).Error; err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	return nil
}

func (r *documentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, metadata map[string]interface{}) error {
	updates := map[string]interface{}{
		"status":          status,
		"last_updated_at": gorm.Expr("NOW()"),
	}

	if metadata != nil {
		metaBytes, err := json.Marshal(metadata)
		if err == nil {
			updates["ai_analysis_json"] = string(metaBytes)
		}
	}

	return r.db.WithContext(ctx).Model(&domain.Document{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *documentRepository) UpdateTitle(ctx context.Context, id uuid.UUID, title string) error {
	return r.db.WithContext(ctx).Model(&domain.Document{}).
		Where("id = ?", id).
		Update("title", title).Error
}

// StoreChunks atomically inserts all chunks within a single DB transaction.
// If any insert fails, the entire batch is rolled back.
func (r *documentRepository) StoreChunks(ctx context.Context, chunks []domain.DocumentChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&chunks).Error; err != nil {
			return fmt.Errorf("atomic chunk insert failed: %w", err)
		}
		return nil
	})
}

func (r *documentRepository) FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error) {
	var docs []*domain.Document
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Document{}).Where("tenant_id = ?", tenantID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	err := db.Limit(limit).Offset(offset).Order("created_at DESC").Find(&docs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	return docs, total, nil
}

func (r *documentRepository) FindByID(ctx context.Context, id string) (*domain.Document, error) {
	var doc domain.Document
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&doc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find document: %w", err)
	}
	return &doc, nil
}

// HybridSearch executes the full Hybrid Search pipeline:
//  1. Pre-filter by tenant_id (authorization hard gate — no cross-tenant data leakage possible)
//  2. Tune HNSW ef_search for this session (accuracy vs speed trade-off)
//  3. Dense retrieval via pgvector cosine similarity (semantic)
//  4. Sparse retrieval via PostgreSQL tsvector GIN FTS (lexical / keyword)
//  5. Fuse both rankings with Reciprocal Rank Fusion (RRF)
func (r *documentRepository) HybridSearch(ctx context.Context, params domain.HybridSearchParams) ([]domain.HybridSearchResult, error) {
	if params.TenantID == "" {
		return nil, fmt.Errorf("HybridSearch: tenant_id is required")
	}
	if len(params.QueryEmbedding) == 0 {
		return nil, fmt.Errorf("HybridSearch: query embedding is required")
	}
	if params.TopK <= 0 {
		params.TopK = 10
	}
	if params.EfSearch <= 0 {
		params.EfSearch = 100 // default: good accuracy, ~5ms latency on 1M vectors
	}
	if params.RRFConstant <= 0 {
		params.RRFConstant = 60 // original RRF paper default
	}

	// Detect if pgvector embedding column exists
	var hasEmbedding bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'document_chunks' AND column_name = 'embedding'
		)
	`).Scan(&hasEmbedding).Error
	if err != nil {
		hasEmbedding = false
	}

	var searchSQL string
	var queryArgs []interface{}

	if hasEmbedding {
		// Step 1: Set ef_search for this session to control the HNSW accuracy/speed trade-off.
		// Higher values scan more nodes in the HNSW graph → more accurate but slower.
		// This MUST be done before the vector query on the same connection.
		efSearchSQL := fmt.Sprintf("SET LOCAL hnsw.ef_search = %d", params.EfSearch)
		if err := r.db.WithContext(ctx).Exec(efSearchSQL).Error; err != nil {
			return nil, fmt.Errorf("failed to set ef_search: %w", err)
		}

		// Step 2: Format the embedding vector for pgvector
		vecStr := pgvectorFormat(params.QueryEmbedding)

		// Step 3: RRF Hybrid Search query
		searchSQL = fmt.Sprintf(`
			WITH
			vector_search AS (
				SELECT
					dc.id        AS chunk_id,
					dc.document_id,
					dc.content,
					ROW_NUMBER() OVER (ORDER BY dc.embedding <=> '%s'::vector) AS rank
				FROM document_chunks dc
				WHERE dc.tenant_id = '%s'
				ORDER BY dc.embedding <=> '%s'::vector
				LIMIT %d
			),
			fts_search AS (
				SELECT
					dc.id        AS chunk_id,
					dc.document_id,
					dc.content,
					ROW_NUMBER() OVER (ORDER BY ts_rank(dc.content_tsv, query) DESC) AS rank
				FROM document_chunks dc,
					to_tsquery('english', ?) AS query
				WHERE dc.tenant_id = '%s'
				  AND dc.content_tsv @@ query
				ORDER BY ts_rank(dc.content_tsv, query) DESC
				LIMIT %d
			),
			rrf AS (
				SELECT
					COALESCE(v.chunk_id, f.chunk_id)       AS chunk_id,
					COALESCE(v.document_id, f.document_id) AS document_id,
					COALESCE(v.content, f.content)         AS content,
					COALESCE(v.rank, %d + 1)               AS vector_rank,
					COALESCE(f.rank, %d + 1)               AS fts_rank,
					(1.0 / (%d + COALESCE(v.rank, %d + 1)) +
					 1.0 / (%d + COALESCE(f.rank, %d + 1))) AS rrf_score
				FROM vector_search v
				FULL OUTER JOIN fts_search f ON v.chunk_id = f.chunk_id
			)
			SELECT
				r.chunk_id,
				r.document_id,
				r.content,
				r.vector_rank,
				r.fts_rank,
				r.rrf_score,
				d.title AS document_title
			FROM rrf r
			JOIN documents d ON d.id = r.document_id
			WHERE d.tenant_id = '%s'
			ORDER BY r.rrf_score DESC
			LIMIT %d`,
			vecStr, params.TenantID, vecStr, params.TopK*3,
			params.TenantID, params.TopK*3,
			params.TopK*3, params.TopK*3,
			params.RRFConstant, params.TopK*3,
			params.RRFConstant, params.TopK*3,
			params.TenantID, params.TopK,
		)
		queryArgs = append(queryArgs, pgFTSQuery(params.QueryText))
	} else {
		// Fallback to FTS only when pgvector is not available
		ftsQuery := pgFTSQuery(params.QueryText)
		if ftsQuery == "" {
			searchSQL = fmt.Sprintf(`
				SELECT
					dc.id AS chunk_id,
					dc.document_id,
					dc.content,
					0 AS vector_rank,
					1 AS fts_rank,
					1.0 AS rrf_score,
					d.title AS document_title
				FROM document_chunks dc
				JOIN documents d ON d.id = dc.document_id
				WHERE dc.tenant_id = '%s' AND d.tenant_id = '%s'
				LIMIT %d`,
				params.TenantID, params.TenantID, params.TopK,
			)
		} else {
			searchSQL = fmt.Sprintf(`
				WITH
				fts_search AS (
					SELECT
						dc.id        AS chunk_id,
						dc.document_id,
						dc.content,
						ROW_NUMBER() OVER (ORDER BY ts_rank(dc.content_tsv, query) DESC) AS rank
					FROM document_chunks dc,
						to_tsquery('english', ?) AS query
					WHERE dc.tenant_id = '%s'
					  AND dc.content_tsv @@ query
					ORDER BY ts_rank(dc.content_tsv, query) DESC
					LIMIT %d
				)
				SELECT
					f.chunk_id,
					f.document_id,
					f.content,
					0 AS vector_rank,
					f.rank AS fts_rank,
					(1.0 / (%d + f.rank)) AS rrf_score,
					d.title AS document_title
				FROM fts_search f
				JOIN documents d ON d.id = f.document_id
				WHERE d.tenant_id = '%s'
				ORDER BY rrf_score DESC
				LIMIT %d`,
				params.TenantID, params.TopK*3,
				params.RRFConstant,
				params.TenantID, params.TopK,
			)
			queryArgs = append(queryArgs, ftsQuery)
		}
	}

	// Use safe parameterized query
	rows, err := r.db.WithContext(ctx).Raw(searchSQL, queryArgs...).Rows()
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}
	defer rows.Close()

	var results []domain.HybridSearchResult
	for rows.Next() {
		var res domain.HybridSearchResult
		if err := rows.Scan(
			&res.ChunkID,
			&res.DocumentID,
			&res.Content,
			&res.VectorRank,
			&res.FTSRank,
			&res.RRFScore,
			&res.DocumentTitle,
		); err != nil {
			return nil, fmt.Errorf("failed to scan hybrid search result: %w", err)
		}
		results = append(results, res)
	}

	return results, nil
}

// pgvectorFormat converts a []float32 embedding to the pgvector literal format.
func pgvectorFormat(embedding []float32) string {
	if len(embedding) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range embedding {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", v)
	}
	sb.WriteByte(']')
	return sb.String()
}

// pgFTSQuery converts a plain-text query to a PostgreSQL tsquery-compatible string.
// Joins words with '|' for OR logic; removes special chars.
func pgFTSQuery(text string) string {
	words := strings.Fields(text)
	for i, w := range words {
		words[i] = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, w)
	}
	// Filter empty
	var valid []string
	for _, w := range words {
		if w != "" {
			valid = append(valid, w)
		}
	}
	if len(valid) == 0 {
		return ""
	}
	return strings.Join(valid, " | ")
}

func (r *documentRepository) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", docID, tenantID).
		Delete(&domain.Document{}).Error
}
