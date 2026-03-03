package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/telemetry"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type RAGRetrieverHandler struct {
	docRepo    domain.DocumentRepository
	gemini     *genai.Client
	embedModel *genai.EmbeddingModel
}

func NewRAGRetrieverHandler(docRepo domain.DocumentRepository, apiKey string) (*RAGRetrieverHandler, error) {
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	em := client.EmbeddingModel("text-embedding-004")

	return &RAGRetrieverHandler{
		docRepo:    docRepo,
		gemini:     client,
		embedModel: em,
	}, nil
}

// Execute performs hybrid document retrieval based on input from previous nodes.
// Results are filtered by RRF score and concatenated for downstream LLM nodes.
func (h *RAGRetrieverHandler) Execute(ctx *engine.ExecutionContext, node engine.Node) error {
	// 1. Mandatory Security: Retrieve tenant_id bound to this execution context.
	// This ensures a DAG process can NEVER accidentally search across tenants.
	tenantIDVal, ok := ctx.Get("tenant_id")
	if !ok {
		return fmt.Errorf("node %s: missing tenant_id in ExecutionContext (security violation)", node.ID)
	}
	tenantID, ok := tenantIDVal.(string)
	if !ok || tenantID == "" {
		return fmt.Errorf("node %s: invalid tenant_id type", node.ID)
	}

	// 2. Extract configuration from frontend node properties
	queryKey, _ := node.Data["query_input_key"].(string)
	if queryKey == "" {
		queryKey = "global_input" // fallback sequence
	}

	queryVal, ok := ctx.Get(queryKey)
	if !ok {
		log.Printf("[RAG Node] Query input %s is empty, skipping RAG retrieval", queryKey)
		ctx.Set(fmt.Sprintf("%s_result", node.ID), "")
		return nil
	}
	query := fmt.Sprintf("%v", queryVal)

	topK := 5
	if tk, ok := node.Data["top_k"].(float64); ok { // JSON numbers unmarshal to float64
		topK = int(tk)
	}
	minScore := 0.01 // minimum RRF score threshold to discard noise
	if ms, ok := node.Data["min_score"].(float64); ok {
		minScore = ms
	}

	log.Printf("[RAG Node:%s] Executing retrieval for Tenant %s. Query: %q", node.ID, tenantID, query)

	// 3. Generate vector embedding for the search query using Gemini.
	queryEmbedding, err := h.embedQuery(context.Background(), tenantID, query)
	if err != nil {
		return fmt.Errorf("node %s: failed to embed query: %w", node.ID, err)
	}

	// 4. Execute Hybrid Search (HNSW + FTS + RRF) through the postgres repository.
	searchStart := time.Now()
	params := domain.HybridSearchParams{
		TenantID:       tenantID,
		QueryText:      query,
		QueryEmbedding: queryEmbedding,
		TopK:           topK,
		EfSearch:       150, // Higher ef_search for workflow agents to prioritize accuracy over extreme latency
		RRFConstant:    60,
	}

	results, err := h.docRepo.HybridSearch(context.Background(), params)

	// Record RAG Latency Metric
	telemetry.RagLatency.WithLabelValues(tenantID).Observe(time.Since(searchStart).Seconds())

	if err != nil {
		return fmt.Errorf("node %s: HybridSearch failed: %w", node.ID, err)
	}

	// 5. Filter by minimum RRF score and build context string
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Relevent Knowledge Base Context:\n\n")

	validChunks := 0
	for _, res := range results {
		if res.RRFScore < minScore {
			continue // Discard low-relevance matches
		}
		validChunks++
		contextBuilder.WriteString(fmt.Sprintf("---\nDocument: %s\n%s\n", res.DocumentTitle, res.Content))
	}

	finalContext := ""
	if validChunks > 0 {
		finalContext = contextBuilder.String()
	} else {
		finalContext = "No relevant context found in the knowledge base."
	}

	// 6. Inject the formatted context back into the ExecutionContext for LLM consumption
	ctx.Set(fmt.Sprintf("%s_result", node.ID), finalContext)
	log.Printf("[RAG Node:%s] Complete. Kept %d/%d chunks (min_score=%.3f)", node.ID, validChunks, len(results), minScore)

	return nil
}

func (h *RAGRetrieverHandler) embedQuery(ctx context.Context, tenantID string, query string) ([]float32, error) {
	resp, err := h.embedModel.EmbedContent(ctx, genai.Text(query))
	if err != nil {
		return nil, err
	}

	return resp.Embedding.Values, nil
}
