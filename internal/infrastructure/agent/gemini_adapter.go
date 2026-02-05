package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiStudioAdapter wraps the official Google GenAI library to adapt it to the SDK's LLM interface
type GeminiStudioAdapter struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	modelName string
}

// Ensure implementation of LLM interface
var _ interfaces.LLM = (*GeminiStudioAdapter)(nil)

// NewGeminiStudioAdapter creates a new adapter.
// It accepts an optional context for initialization timeout.
func NewGeminiStudioAdapter(ctx context.Context, apiKey string, modelName string) (*GeminiStudioAdapter, error) {
	// If context is nil, create one with a timeout to prevent hanging initialization
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
	}

	// KEY: Using WithAPIKey for Google AI Studio
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
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
	return g.client.Close()
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

	// Robust Parsing: Join all text parts
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

// GenerateWithTools implements interfaces.LLM
func (g *GeminiStudioAdapter) GenerateWithTools(ctx context.Context, prompt string, tools []interfaces.Tool, options ...interfaces.GenerateOption) (string, error) {
	return "", errors.New("tools not supported in simple adapter")
}

// GenerateWithToolsDetailed implements interfaces.LLM
func (g *GeminiStudioAdapter) GenerateWithToolsDetailed(ctx context.Context, prompt string, tools []interfaces.Tool, options ...interfaces.GenerateOption) (*interfaces.LLMResponse, error) {
	return nil, errors.New("tools detailed not supported in simple adapter")
}
