package agent

import (
	"context"

	sdkagent "github.com/Ingenimax/agent-sdk-go/pkg/agent"
	"github.com/Ingenimax/agent-sdk-go/pkg/memory"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/minimax"
)

type AgentFactory struct {
	client    *minimax.Client
	apiKey    string
	redisAddr string
}

// NewAgentFactory initializes the factory with the MiniMax client.
func NewAgentFactory(ctx context.Context, apiKey, redisAddr string) (*AgentFactory, error) {
	client := minimax.NewClient(apiKey)

	return &AgentFactory{
		client:    client,
		apiKey:    apiKey,
		redisAddr: redisAddr,
	}, nil
}

// Close should be called when shutting down the application
func (f *AgentFactory) Close() error {
	return nil
}

func (f *AgentFactory) CreateAgent(executionID string, systemPrompt string) (*sdkagent.Agent, error) {
	llmProvider, err := NewMiniMaxAdapter(f.client, "MiniMax-M2.5")
	if err != nil {
		return nil, err
	}

	mem := memory.NewConversationBuffer()

	return sdkagent.NewAgent(
		sdkagent.WithLLM(llmProvider),
		sdkagent.WithMemory(mem),
		sdkagent.WithSystemPrompt(systemPrompt),
		sdkagent.WithName("ElysianWorker-"+executionID),
	)
}
