package engine

import (
	"encoding/json"
	"sync"
)

type VisualGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"` // Contoh: "start", "llm_agent", "end"
	Data map[string]interface{} `json:"data"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// ExecutionContext menyimpan state (variabel) selama workflow berjalan
type ExecutionContext struct {
	mu      sync.RWMutex
	Payload map[string]interface{}
}

func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Payload: make(map[string]interface{}),
	}
}

// ParseWorkflow mem-parsing JSON bytes ke VisualGraph
func ParseWorkflow(data []byte) (*VisualGraph, error) {
	var graph VisualGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, err
	}
	return &graph, nil
}

func (c *ExecutionContext) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Payload[key] = value
}

func (c *ExecutionContext) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, exists := c.Payload[key]
	return val, exists
}
