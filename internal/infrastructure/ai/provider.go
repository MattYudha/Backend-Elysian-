package ai

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/minimax"
)

type Provider interface {
	Generate(ctx context.Context, prompt string, model string) (string, error)
}

type MiniMaxProvider struct {
	client *minimax.Client
}

func NewMiniMaxProvider(apiKey string) *MiniMaxProvider {
	return &MiniMaxProvider{
		client: minimax.NewClient(apiKey),
	}
}

func (p *MiniMaxProvider) Generate(ctx context.Context, prompt string, model string) (string, error) {
	messages := []minimax.ChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}
	text, _, err := p.client.GenerateContent(ctx, "", messages)
	return text, err
}

// GeminiProvider maps to MiniMaxProvider to satisfy the "all-in MiniMax" requirement across the codebase.
type GeminiProvider = MiniMaxProvider

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return NewMiniMaxProvider(apiKey)
}

