package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/parsing"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/storage"
	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"google.golang.org/api/option"
)

// Task names
const (
	TypeParseDocument = "rag:parse_document"
	TypeEmbedDocument = "rag:embed_document"
)

// ParseDocumentPayload carries the IDs needed for text extraction.
type ParseDocumentPayload struct {
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`
	S3URI      string `json:"s3_uri"`
	Category   string `json:"category"`
}

// EmbedDocumentPayload carries the IDs needed for vectorization.
type EmbedDocumentPayload struct {
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`
	Category   string `json:"category"`
}

// NewParseDocumentTask creates an Asynq task to parse a document.
func NewParseDocumentTask(documentID, tenantID, s3URI string, category string) (*asynq.Task, error) {
	payload, err := json.Marshal(ParseDocumentPayload{
		DocumentID: documentID,
		TenantID:   tenantID,
		S3URI:      s3URI,
		Category:   category,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(
		TypeParseDocument,
		payload,
		asynq.MaxRetry(3),
		asynq.Queue("heavy_parsing"),
	), nil
}

// NewEmbedDocumentTask creates an Asynq task to chunk and embed parsed text.
func NewEmbedDocumentTask(documentID, tenantID string, category string) (*asynq.Task, error) {
	payload, err := json.Marshal(EmbedDocumentPayload{
		DocumentID: documentID,
		TenantID:   tenantID,
		Category:   category,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(
		TypeEmbedDocument,
		payload,
		asynq.MaxRetry(3),
		asynq.Queue("heavy_parsing"),
	), nil
}

// DocumentTaskHandler is the concrete Asynq handler for the split RAG pipeline.
type DocumentTaskHandler struct {
	docRepo      domain.DocumentRepository
	s3           *storage.S3Service
	parser       *parsing.DocumentParser
	geminiAPIKey string
	mongoClient  *database.MongoClient
}

func NewDocumentTaskHandler(
	docRepo domain.DocumentRepository,
	s3 *storage.S3Service,
	parser *parsing.DocumentParser,
	geminiAPIKey string,
	mongoClient *database.MongoClient,
) *DocumentTaskHandler {
	return &DocumentTaskHandler{
		docRepo:      docRepo,
		s3:           s3,
		parser:       parser,
		geminiAPIKey: geminiAPIKey,
		mongoClient:  mongoClient,
	}
}

// HandleParseDocument performs Step 1: Download -> Extract Text -> Save to DB (pending_qa)
func (h *DocumentTaskHandler) HandleParseDocument(ctx context.Context, t *asynq.Task) error {
	var payload ParseDocumentPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	docID, _ := uuid.Parse(payload.DocumentID)

	log.Printf("[RAG-Worker] ▶ Parsing Document %s for Tenant %s (S3: %s)",
		payload.DocumentID, payload.TenantID, payload.S3URI)

	// 1. Mark as processing/parsing
	_ = h.docRepo.UpdateStatus(ctx, docID, "processing", nil)

	// 2. Download from S3 to local temp file
	localPath, err := h.s3.DownloadToTemp(ctx, payload.S3URI)
	if err != nil {
		h.failDoc(ctx, docID, "S3 download failed: "+err.Error())
		return fmt.Errorf("S3 download failed: %w", err)
	}
	defer os.Remove(localPath)

	// 3. Extract text
	rawText, err := h.parser.ExtractText(ctx, localPath)
	if err != nil {
		h.failDoc(ctx, docID, "text extraction failed: "+err.Error())
		return fmt.Errorf("text extraction failed: %w", err)
	}

	// Fetch document info to get details (e.g. title) for MongoDB metadata
	doc, err := h.docRepo.FindByID(ctx, payload.DocumentID)
	if err != nil {
		h.failDoc(ctx, docID, "failed to find document: "+err.Error())
		return fmt.Errorf("failed to find document: %w", err)
	}

	// 4. Save raw parsed text to MongoDB Staging Area
	stagingDoc := &database.StagingDocument{
		ID:        payload.DocumentID,
		TenantID:  payload.TenantID,
		FileName:  doc.Title,
		RawText:   rawText,
		Status:    database.StatusPendingQA,
	}
	if err := h.mongoClient.SaveDocument(ctx, stagingDoc); err != nil {
		h.failDoc(ctx, docID, "failed to save parsed text to MongoDB: "+err.Error())
		return fmt.Errorf("failed to save parsed text to MongoDB: %w", err)
	}

	// 5. Update status to pending_qa in PostgreSQL with non-heavy metadata
	metadata := map[string]interface{}{
		"parser": "docling",
	}
	if err := h.docRepo.UpdateStatus(ctx, docID, "pending_qa", metadata); err != nil {
		return fmt.Errorf("failed to mark document pending_qa: %w", err)
	}

	log.Printf("[RAG-Worker] ✅ Document %s parsed and staged in MongoDB, status set to pending_qa", payload.DocumentID)
	return nil
}

// HandleEmbedDocument performs Step 2: Retrieve Extracted Text -> Chunk -> Gemini Embed -> Store chunks (ready)
func (h *DocumentTaskHandler) HandleEmbedDocument(ctx context.Context, t *asynq.Task) error {
	var payload EmbedDocumentPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	docID, _ := uuid.Parse(payload.DocumentID)
	tenantID, _ := uuid.Parse(payload.TenantID)

	log.Printf("[RAG-Worker] ▶ Embedding Document %s for Tenant %s", payload.DocumentID, payload.TenantID)

	// 1. Mark as processing/embedding
	_ = h.docRepo.UpdateStatus(ctx, docID, "processing", nil)

	// 2. Retrieve document from DB
	doc, err := h.docRepo.FindByID(ctx, payload.DocumentID)
	if err != nil {
		h.failDoc(ctx, docID, "failed to find document record: "+err.Error())
		return fmt.Errorf("failed to find document: %w", err)
	}

	// 3. Retrieve raw text from MongoDB Staging Area
	stagingDoc, err := h.mongoClient.GetDocument(ctx, payload.DocumentID)
	if err != nil {
		h.failDoc(ctx, docID, "failed to retrieve staging document from MongoDB: "+err.Error())
		return fmt.Errorf("failed to retrieve staging document: %w", err)
	}
	extractedText := stagingDoc.RawText

	var metadata map[string]interface{}
	if len(doc.AiAnalysisJSON) > 0 {
		_ = json.Unmarshal([]byte(doc.AiAnalysisJSON), &metadata)
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// 4. Markdown-Header-Aware Chunking
	mdChunks := MarkdownAwareChunker(extractedText, 1000)
	log.Printf("[RAG-Worker] ✂ %d semantic chunks from Document %s", len(mdChunks), payload.DocumentID)

	if len(mdChunks) == 0 {
		h.failDoc(ctx, docID, "chunker produced no output")
		return fmt.Errorf("chunker produced no output: %w", asynq.SkipRetry)
	}

	// 5. Batch embed via Gemini
	fullTexts := make([]string, len(mdChunks))
	for i, c := range mdChunks {
		fullTexts[i] = c.FullContent
	}

	embeddings, err := h.batchEmbed(ctx, fullTexts)
	if err != nil {
		h.failDoc(ctx, docID, "Gemini embedding failed: "+err.Error())
		return fmt.Errorf("Gemini embedding failed: %w", err)
	}

	// 6. Build DocumentChunk slice
	var docChunks []domain.DocumentChunk
	for i, mdChunk := range mdChunks {
		approxPageNum := (i / 3) + 1

		docChunks = append(docChunks, domain.DocumentChunk{
			TenantID:   tenantID,
			DocumentID: docID,
			Content:    mdChunk.FullContent,
			Embedding:  embeddings[i],
			ChunkIndex: mdChunk.Index,
			PageNumber: approxPageNum,
			Category:   payload.Category,
		})
	}

	// 7. Atomic batch insert
	if err := h.docRepo.StoreChunks(ctx, docChunks); err != nil {
		h.failDoc(ctx, docID, "atomic chunk insert failed: "+err.Error())
		return fmt.Errorf("atomic chunk insert failed: %w", err)
	}

	// 8. Update status to ready, preserving metadata but removing extracted_text to save DB storage space
	delete(metadata, "extracted_text")
	metadata["chunks_count"] = len(docChunks)
	metadata["model"] = "text-embedding-004"
	metadata["parser"] = "docling"

	if err := h.docRepo.UpdateStatus(ctx, docID, "ready", metadata); err != nil {
		return fmt.Errorf("failed to mark document ready: %w", err)
	}

	log.Printf("[RAG-Worker] ✅ Document %s ready: %d chunks", payload.DocumentID, len(docChunks))
	return nil
}

// failDoc is a helper that logs and marks the document as failed with an error reason.
func (h *DocumentTaskHandler) failDoc(ctx context.Context, docID uuid.UUID, reason string) {
	log.Printf("[RAG-Worker] ❌ Document %s failed: %s", docID, reason)
	_ = h.docRepo.UpdateStatus(ctx, docID, "failed", map[string]interface{}{"error": reason})
}

// batchEmbed sends all chunk texts to Gemini text-embedding-004 in a single batch call.
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
