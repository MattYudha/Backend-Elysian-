package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/parsing"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/storage"
	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"google.golang.org/api/option"
)

// Task names
const (
	TypeProcessDocument = "rag:process_document"
)

// ProcessDocumentPayload carries the IDs needed by the background worker.
type ProcessDocumentPayload struct {
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`
	S3URI      string `json:"s3_uri"`
	Category   string `json:"category"`
}

// NewProcessDocumentTask creates an Asynq task pinned to the heavy_parsing queue.
// The heavy_parsing queue is limited to concurrency=2 in the server config to
// prevent Docling OOM when multiple large PDFs are uploaded simultaneously.
func NewProcessDocumentTask(documentID, tenantID, s3URI string, category string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProcessDocumentPayload{
		DocumentID: documentID,
		TenantID:   tenantID,
		S3URI:      s3URI,
		Category:   category,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(
		TypeProcessDocument,
		payload,
		asynq.MaxRetry(3),
		asynq.Queue("heavy_parsing"), // MANDATORY: isolates Docling workload from other queues
	), nil
}

// DocumentTaskHandler is the concrete Asynq handler for the full RAG pipeline.
// All heavy state (DB, S3, Docling, Gemini) is injected at startup — NOT per task invocation.
type DocumentTaskHandler struct {
	docRepo      domain.DocumentRepository
	s3           *storage.S3Service
	parser       *parsing.DocumentParser // Docling → Unstructured.io → plain text
	geminiAPIKey string                  // from config.AI.GeminiAPIKey, never hardcoded
}

func NewDocumentTaskHandler(
	docRepo domain.DocumentRepository,
	s3 *storage.S3Service,
	parser *parsing.DocumentParser,
	geminiAPIKey string,
) *DocumentTaskHandler {
	return &DocumentTaskHandler{
		docRepo:      docRepo,
		s3:           s3,
		parser:       parser,
		geminiAPIKey: geminiAPIKey,
	}
}

// Handle runs the full RAG ingestion pipeline:
//
//	S3 Download → Docling Parse → Markdown-Aware Chunk → Gemini Embed → Atomic DB Insert
func (h *DocumentTaskHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var payload ProcessDocumentPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		// Malformed payload can never succeed — skip retry immediately
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	docID, _ := uuid.Parse(payload.DocumentID)
	tenantID, _ := uuid.Parse(payload.TenantID)

	log.Printf("[RAG-Worker] ▶ Processing Document %s for Tenant %s (S3: %s)",
		payload.DocumentID, payload.TenantID, payload.S3URI)

	// ── Step 1: Mark as processing ─────────────────────────────────────────────
	_ = h.docRepo.UpdateStatus(ctx, docID, "processing", nil)

	// ── Step 2: Download from S3 to local temp file ─────────────────────────────
	// Zero server RAM for the file itself — only metadata lives in Go memory.
	localPath, err := h.s3.DownloadToTemp(ctx, payload.S3URI)
	if err != nil {
		h.failDoc(ctx, docID, "S3 download failed: "+err.Error())
		return fmt.Errorf("S3 download failed: %w", err) // Asynq retries
	}
	defer os.Remove(localPath) // Always clean up temp file

	// ── Step 3: Enterprise document parsing via Docling ──────────────────────────
	// Docling preserves multi-column layout, tables, and OCR text from scanned pages.
	// Falls back to Unstructured.io, then plain text reader.
	rawText, err := h.parser.ExtractText(ctx, localPath)
	if err != nil {
		h.failDoc(ctx, docID, "text extraction failed: "+err.Error())
		return fmt.Errorf("text extraction failed: %w", err) // Asynq retries
	}

	// ── Step 4: Markdown-Header-Aware Chunking ──────────────────────────────────
	// Rules enforced here:
	//  a) Split on # ## ### header boundaries — NEVER in the middle of a section
	//  b) Tables are atomic units — NEVER split between table rows
	//  c) Every chunk is prefixed with its full header breadcrumb (parent context injection)
	//     e.g. "[Context: HR Policy > Leave > Sick Leave]\n\n<chunk content>"
	mdChunks := MarkdownAwareChunker(rawText, 1000)
	log.Printf("[RAG-Worker] ✂ %d semantic chunks from Document %s", len(mdChunks), payload.DocumentID)

	if len(mdChunks) == 0 {
		h.failDoc(ctx, docID, "chunker produced no output")
		return fmt.Errorf("chunker produced no output: %w", asynq.SkipRetry)
	}

	// ── Step 5: Batch embed via Gemini text-embedding-004 ──────────────────────
	// We embed FullContent (header breadcrumb + chunk text) so the vector
	// encodes the document position, not just the fragment meaning.
	fullTexts := make([]string, len(mdChunks))
	for i, c := range mdChunks {
		fullTexts[i] = c.FullContent
	}

	embeddings, err := h.batchEmbed(ctx, fullTexts)
	if err != nil {
		h.failDoc(ctx, docID, "Gemini embedding failed: "+err.Error())
		return fmt.Errorf("Gemini embedding failed: %w", err) // Asynq retries
	}

	// ── Step 6: Build DocumentChunk slice ──────────────────────────────────────
	// TenantID on every chunk is mandatory for pgvector pre-filtering.
	var docChunks []domain.DocumentChunk
	for i, mdChunk := range mdChunks {
		// Mock PageNumber mapping (assuming ~3 chunks per literal page of A4 text)
		approxPageNum := (i / 3) + 1

		docChunks = append(docChunks, domain.DocumentChunk{
			TenantID:   tenantID,
			DocumentID: docID,
			Content:    mdChunk.FullContent, // store with context prefix
			Embedding:  embeddings[i],
			ChunkIndex: mdChunk.Index,
			PageNumber: approxPageNum,
			Category:   payload.Category,
		})
	}

	// ── Step 7: Atomic batch insert ─────────────────────────────────────────────
	// ALL chunks are inserted in a single transaction.
	// If any chunk fails, the entire batch is rolled back.
	// No orphaned partial knowledge base states are possible.
	if err := h.docRepo.StoreChunks(ctx, docChunks); err != nil {
		h.failDoc(ctx, docID, "atomic chunk insert failed: "+err.Error())
		return fmt.Errorf("atomic chunk insert failed: %w", err) // Asynq retries
	}

	// ── Step 8: Mark document as ready ─────────────────────────────────────────
	metadata := map[string]interface{}{
		"chunks_count": len(docChunks),
		"model":        "text-embedding-004",
		"parser":       "docling",
	}
	if err := h.docRepo.UpdateStatus(ctx, docID, "ready", metadata); err != nil {
		return fmt.Errorf("failed to mark document ready: %w", err)
	}

	log.Printf("[RAG-Worker] ✅ Document %s ready: %d chunks, model=text-embedding-004",
		payload.DocumentID, len(docChunks))
	return nil
}

// failDoc is a helper that logs and marks the document as failed with an error reason.
func (h *DocumentTaskHandler) failDoc(ctx context.Context, docID uuid.UUID, reason string) {
	log.Printf("[RAG-Worker] ❌ Document %s failed: %s", docID, reason)
	_ = h.docRepo.UpdateStatus(ctx, docID, "failed", map[string]interface{}{"error": reason})
}

// batchEmbed sends all chunk texts to Gemini text-embedding-004 in a single batch call.
// One API call per task (not one per chunk) — minimizes latency and API quota usage.
func (h *DocumentTaskHandler) batchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(h.geminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("Gemini client init failed: %w", err)
	}
	defer client.Close()

	em := client.EmbeddingModel("text-embedding-004")
	batch := em.NewBatch()
	for _, text := range texts {
		batch.AddContent(genai.Text(text))
	}

	resp, err := em.BatchEmbedContents(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("Gemini BatchEmbedContents failed: %w", err)
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		embeddings[i] = e.Values
	}
	return embeddings, nil
}
