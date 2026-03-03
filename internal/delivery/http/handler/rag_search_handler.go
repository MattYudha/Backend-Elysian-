package handler

import (
	"context"
	"net/http"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// RAGSearchHandler performs Hybrid RAG Search.
// It embeds the query via Gemini, then delegates to the repository's HybridSearch
// which applies HNSW semantic search + PostgreSQL FTS + RRF fusion — all scoped to a tenant.
type RAGSearchHandler struct {
	docRepo      domain.DocumentRepository
	geminiAPIKey string // read from config, never hardcoded
}

func NewRAGSearchHandler(docRepo domain.DocumentRepository, geminiAPIKey string) *RAGSearchHandler {
	return &RAGSearchHandler{docRepo: docRepo, geminiAPIKey: geminiAPIKey}
}

type SearchRequest struct {
	Query       string `json:"query" binding:"required"`
	TopK        int    `json:"top_k"`
	EfSearch    int    `json:"ef_search"`
	RRFConstant int    `json:"rrf_constant"`
}

// Search godoc
// @Summary      Hybrid RAG Search (HNSW + FTS + RRF)
// @Description  Tenant-scoped hybrid search: pgvector HNSW (semantic) + PostgreSQL FTS (lexical) fused via Reciprocal Rank Fusion. ef_search is tuned per query for accuracy/speed.
// @Tags         knowledge
// @Accept       json
// @Produce      json
// @Param        request body SearchRequest true "Search Request"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/search [post]
func (h *RAGSearchHandler) Search(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "X-Tenant-ID header required"})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Apply sensible defaults
	topK := 10
	if req.TopK > 0 {
		topK = req.TopK
	}
	efSearch := 100
	if req.EfSearch > 0 {
		efSearch = req.EfSearch
	}
	rrfConstant := 60
	if req.RRFConstant > 0 {
		rrfConstant = req.RRFConstant
	}

	// Step 1: Embed the query text using Gemini text-embedding-004.
	// The API key is injected from config — never hardcoded.
	queryEmbedding, err := h.getQueryEmbedding(c.Request.Context(), req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Query embedding failed: " + err.Error()})
		return
	}

	// Step 2: Hybrid Search at the repository layer.
	// Tenant pre-filtering is enforced INSIDE the SQL query — not just in Go.
	results, err := h.docRepo.HybridSearch(c.Request.Context(), domain.HybridSearchParams{
		TenantID:       tenantID,
		QueryText:      req.Query,
		QueryEmbedding: queryEmbedding,
		TopK:           topK,
		EfSearch:       efSearch,
		RRFConstant:    rrfConstant,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Hybrid search failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"results": results,
		"meta": gin.H{
			"strategy":     "hybrid_rrf",
			"top_k":        topK,
			"ef_search":    efSearch,
			"rrf_constant": rrfConstant,
			"count":        len(results),
		},
	})
}

// getQueryEmbedding calls Gemini text-embedding-004 to produce a 1536-dim vector
// for the user's query. This vector is then used in the HNSW cosine similarity search.
func (h *RAGSearchHandler) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(h.geminiAPIKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	em := client.EmbeddingModel("text-embedding-004")
	resp, err := em.EmbedContent(ctx, genai.Text(query))
	if err != nil {
		return nil, err
	}
	return resp.Embedding.Values, nil
}
