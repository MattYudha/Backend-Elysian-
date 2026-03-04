-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: RAG Hybrid Search Infrastructure
-- Adds FTS + HNSW indices to document_chunks for Hybrid Search
-- with Reciprocal Rank Fusion (RRF) at query time.
-- ============================================================

-- 1. Add tsvector column for GIN Full-Text Search index
--    Using 'english' dictionary for stemming (change to 'simple' for multilingual)
ALTER TABLE document_chunks
    ADD COLUMN IF NOT EXISTS content_tsv tsvector
        GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;

-- 2. GIN index for FTS lexical retrieval
CREATE INDEX IF NOT EXISTS idx_chunks_fts
    ON document_chunks USING GIN (content_tsv);

-- 3. HNSW index for pgvector approximate nearest-neighbor semantic search.
--    m=16, ef_construction=64 is a good trade-off for 1536-dim embeddings.
--    ef_search is tuned per-session at query time (see repository layer).
-- CREATE INDEX IF NOT EXISTS idx_chunks_hnsw_embedding
--     ON document_chunks USING hnsw (embedding vector_cosine_ops)
--     WITH (m = 16, ef_construction = 64);

-- 4. Composite B-Tree index for tenant-scoped pre-filtering
--    (always filter by tenant_id BEFORE touching the HNSW index)
CREATE INDEX IF NOT EXISTS idx_chunks_tenant_document
    ON document_chunks (tenant_id, document_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_chunks_tenant_document;
DROP INDEX IF EXISTS idx_chunks_hnsw_embedding;
DROP INDEX IF EXISTS idx_chunks_fts;
ALTER TABLE document_chunks DROP COLUMN IF EXISTS content_tsv;
-- +goose StatementEnd
