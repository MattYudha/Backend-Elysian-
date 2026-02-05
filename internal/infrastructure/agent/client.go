package agent

import (
	"context"
	"fmt"

	sdkagent "github.com/Ingenimax/agent-sdk-go/pkg/agent"
	"github.com/Ingenimax/agent-sdk-go/pkg/memory"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type AgentFactory struct {
	client    *genai.Client // Singleton Client
	apiKey    string
	redisAddr string
}

// NewAgentFactory initializes the factory AND the GenAI client connection ONCE.
func NewAgentFactory(ctx context.Context, apiKey, redisAddr string) (*AgentFactory, error) {
	// Initialize Singleton Client Here
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &AgentFactory{
		client:    client,
		apiKey:    apiKey,
		redisAddr: redisAddr,
	}, nil
}

// Close should be called when shutting down the application
func (f *AgentFactory) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

func (f *AgentFactory) CreateAgent(executionID string, systemPrompt string) (*sdkagent.Agent, error) {
	// 1. Init Adapter with Reused Client
	// Notice we pass f.client, NOT creating a new one.
	llmProvider, err := NewGeminiStudioAdapter(f.client, "gemini-1.5-flash")
	if err != nil {
		return nil, err
	}

	// 2. Init Memory (Linked to Execution ID)
	// Fallback to simple buffer as RedisBuffer signature is unverified
	mem := memory.NewConversationBuffer()

	// 3. Create Agent
	return sdkagent.NewAgent(
		sdkagent.WithLLM(llmProvider),
		sdkagent.WithMemory(mem),
		sdkagent.WithSystemPrompt(systemPrompt),
		sdkagent.WithName("ElysianWorker-"+executionID),
	)
}
