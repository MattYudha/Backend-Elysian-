package agent

import (
	"context"
	"errors"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/minimax"
	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
)

type MiniMaxAdapter struct {
	client    *minimax.Client
	modelName string
}

var _ interfaces.LLM = (*MiniMaxAdapter)(nil)

func NewMiniMaxAdapter(client *minimax.Client, modelName string) (*MiniMaxAdapter, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}
	if modelName == "" {
		modelName = "MiniMax-M2.5"
	}
	return &MiniMaxAdapter{
		client:    client,
		modelName: modelName,
	}, nil
}

func (m *MiniMaxAdapter) Name() string {
	return "minimax-adapter"
}

func (m *MiniMaxAdapter) SupportsStreaming() bool {
	return false
}

func (m *MiniMaxAdapter) Close() error {
	return nil
}

func (m *MiniMaxAdapter) Generate(ctx context.Context, prompt string, opts ...interfaces.GenerateOption) (string, error) {
	messages := []minimax.ChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}
	content, _, err := m.client.GenerateContent(ctx, "", messages)
	return content, err
}

func (m *MiniMaxAdapter) GenerateDetailed(ctx context.Context, prompt string, opts ...interfaces.GenerateOption) (*interfaces.LLMResponse, error) {
	content, err := m.Generate(ctx, prompt, opts...)
	if err != nil {
		return nil, err
	}

	return &interfaces.LLMResponse{
		Content: content,
		Model:   m.modelName,
		Metadata: map[string]interface{}{
			"adapter":  "minimax",
			"provider": "minimax",
		},
	}, nil
}

func (m *MiniMaxAdapter) GenerateWithTools(ctx context.Context, prompt string, tools []interfaces.Tool, options ...interfaces.GenerateOption) (string, error) {
	return m.Generate(ctx, prompt, options...)
}

func (m *MiniMaxAdapter) GenerateWithToolsDetailed(ctx context.Context, prompt string, tools []interfaces.Tool, options ...interfaces.GenerateOption) (*interfaces.LLMResponse, error) {
	content, err := m.GenerateWithTools(ctx, prompt, tools, options...)
	if err != nil {
		return nil, err
	}
	return &interfaces.LLMResponse{
		Content:  content,
		Model:    m.modelName,
		Metadata: map[string]interface{}{"tools_used": false},
	}, nil
}
