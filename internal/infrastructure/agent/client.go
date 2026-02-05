package agent

import (
	"context"

	sdkagent "github.com/Ingenimax/agent-sdk-go/pkg/agent"
	"github.com/Ingenimax/agent-sdk-go/pkg/memory"
)

type AgentFactory struct {
	apiKey    string
	redisAddr string
}

func NewAgentFactory(apiKey, redisAddr string) *AgentFactory {
	return &AgentFactory{
		apiKey:    apiKey,
		redisAddr: redisAddr,
	}
}

func (f *AgentFactory) CreateAgent(executionID string, systemPrompt string) (*sdkagent.Agent, error) {
	// 1. Init Adapter
	llmProvider, err := NewGeminiStudioAdapter(context.Background(), f.apiKey, "gemini-1.5-flash")
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
