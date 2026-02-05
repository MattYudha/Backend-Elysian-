package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
	"github.com/google/generative-ai-go/genai"
)

// GeminiStudioAdapter wraps the official Google GenAI library to adapt it to the SDK's LLM interface
type GeminiStudioAdapter struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	modelName string
}

// Ensure implementation of LLM interface
var _ interfaces.LLM = (*GeminiStudioAdapter)(nil)

// NewGeminiStudioAdapter creates a new adapter using an EXISTING client.
// This supports the Singleton pattern to verify connection reuse.
func NewGeminiStudioAdapter(client *genai.Client, modelName string) (*GeminiStudioAdapter, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if modelName == "" {
		modelName = "gemini-1.5-flash"
	}

	model := client.GenerativeModel(modelName)
	return &GeminiStudioAdapter{
		client:    client,
		model:     model,
		modelName: modelName,
	}, nil
}

// Name returns the name of the LLM provider
func (g *GeminiStudioAdapter) Name() string {
	return "gemini-studio-adapter"
}

// SupportsStreaming returns true if this LLM supports streaming
// Currently false to avoid complexity in MVP, though Gemini supports it.
func (g *GeminiStudioAdapter) SupportsStreaming() bool {
	return false
}

// Close releases resources associated with the client
func (g *GeminiStudioAdapter) Close() error {
	// return g.client.Close() // DANGEROUS in Singleton pattern!
	return nil
}

// Generate implements interfaces.LLM
func (g *GeminiStudioAdapter) Generate(ctx context.Context, prompt string, opts ...interfaces.GenerateOption) (string, error) {
	// Apply options (basic support)
	options := &interfaces.GenerateOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Apply LLM Config if present
	if options.LLMConfig != nil {
		if options.LLMConfig.Temperature != 0 {
			g.model.SetTemperature(float32(options.LLMConfig.Temperature))
		}
		if options.LLMConfig.TopP != 0 {
			g.model.SetTopP(float32(options.LLMConfig.TopP))
		}
	}

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response from gemini")
	}

	// Parsing logic is now centralized if possible, but keep it simple here
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			sb.WriteString(string(txt))
		}
	}

	return sb.String(), nil
}

// GenerateDetailed implements interfaces.LLM
func (g *GeminiStudioAdapter) GenerateDetailed(ctx context.Context, prompt string, opts ...interfaces.GenerateOption) (*interfaces.LLMResponse, error) {
	content, err := g.Generate(ctx, prompt, opts...)
	if err != nil {
		return nil, err
	}

	return &interfaces.LLMResponse{
		Content: content,
		Model:   g.modelName,
		Metadata: map[string]interface{}{
			"adapter":  "gemini-studio",
			"provider": "google",
		},
	}, nil
}

// Helper: Mapping SDK Tool ke Google GenAI Tool
func (g *GeminiStudioAdapter) mapTools(sdkTools []interfaces.Tool) *genai.Tool {
	var funcDecls []*genai.FunctionDeclaration

	for _, t := range sdkTools {
		// Konversi Parameter Schema
		// Kita asumsikan t.Parameters adalah map[string]interface{} yang kompatibel dengan Schema
		// Untuk MVP, kita buat schema generik jika parsing kompleks belum siap.
		// Google GenAI butuh *genai.Schema.
		// Implementasi proper butuh rekursif, tapi ini 'cheat' agar code jalan dulu:

		decl := &genai.FunctionDeclaration{
			Name:        t.Name(),
			Description: t.Description(),
			// Schema dummy agar tidak error (Google mewajibkan schema valid)
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {Type: genai.TypeString}, // Default fallback param
				},
			},
		}

		funcDecls = append(funcDecls, decl)
	}

	return &genai.Tool{
		FunctionDeclarations: funcDecls,
	}
}

// GenerateWithTools implements interfaces.LLM
func (g *GeminiStudioAdapter) GenerateWithTools(ctx context.Context, prompt string, tools []interfaces.Tool, options ...interfaces.GenerateOption) (string, error) {
	// 1. Map Tools
	g.model.Tools = []*genai.Tool{g.mapTools(tools)}
	defer func() { g.model.Tools = nil }() // Cleanup tools setelah request selesai

	// 2. Generate
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response from gemini")
	}

	// 3. Handle Response (Text or Function Call)
	// Simple text extraction for now
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			sb.WriteString(string(txt))
		}
	}
	return sb.String(), nil
}

// GenerateWithToolsDetailed implements interfaces.LLM
func (g *GeminiStudioAdapter) GenerateWithToolsDetailed(ctx context.Context, prompt string, tools []interfaces.Tool, options ...interfaces.GenerateOption) (*interfaces.LLMResponse, error) {
	content, err := g.GenerateWithTools(ctx, prompt, tools, options...)
	if err != nil {
		return nil, err
	}
	// Note: Detailed output would ideally include Function Call details if distinct
	return &interfaces.LLMResponse{
		Content:  content,
		Model:    g.modelName,
		Metadata: map[string]interface{}{"tools_used": true},
	}, nil
}
