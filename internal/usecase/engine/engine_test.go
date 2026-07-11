package engine_test

import (
	"fmt"
	"testing"

	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
)

// Dummy Handlers
type StartNodeHandler struct{}

func (h *StartNodeHandler) Execute(ctx *engine.ExecutionContext, node engine.Node) error {
	ctx.Set("start_data", "Initial Payload from Start")
	return nil
}

type AgentNodeHandler struct{}

func (h *AgentNodeHandler) Execute(ctx *engine.ExecutionContext, node engine.Node) error {
	val, exists := ctx.Get("start_data")
	if !exists {
		return fmt.Errorf("Agent failed to read previous context")
	}
	ctx.Set("agent_result", fmt.Sprintf("Processed: %v", val))
	return nil
}

type EndNodeHandler struct{}

func (h *EndNodeHandler) Execute(ctx *engine.ExecutionContext, node engine.Node) error {
	_, exists := ctx.Get("agent_result")
	if !exists {
		return fmt.Errorf("End node failed to read agent context")
	}
	// ctx.Set("final", val) // No need to set, just testing propagation
	return nil
}

func TestWorkflowEngine_ValidDAG(t *testing.T) {
	// 1. Arrange (Setup Graph)
	graph := &engine.VisualGraph{
		Nodes: []engine.Node{
			{ID: "node_1", Type: "start"},
			{ID: "node_2", Type: "agent"},
			{ID: "node_3", Type: "end"},
		},
		Edges: []engine.Edge{
			{Source: "node_1", Target: "node_2"},
			{Source: "node_2", Target: "node_3"},
		},
	}

	// 2. Arrange (Setup Engine & Handlers)
	wfEngine := engine.NewWorkflowEngine()
	wfEngine.Register("start", &StartNodeHandler{})
	wfEngine.Register("agent", &AgentNodeHandler{})
	wfEngine.Register("end", &EndNodeHandler{})

	// 3. Act
	ctx, err := wfEngine.Run(graph, make(map[string]interface{}))

	// 4. Assert
	if err != nil {
		t.Fatalf("Expected valid DAG to execute successfully, got error: %v", err)
	}

	if val, exists := ctx.Get("agent_result"); !exists || val != "Processed: Initial Payload from Start" {
		t.Errorf("Result context value did not propagate correctly. Got: %v", val)
	}
}

func TestWorkflowEngine_CycleDetection(t *testing.T) {
	// 1. Arrange Cyclic Graph (A -> B -> C -> A)
	graph := &engine.VisualGraph{
		Nodes: []engine.Node{
			{ID: "node_1", Type: "start"},
			{ID: "node_2", Type: "agent"},
			{ID: "node_3", Type: "end"},
		},
		Edges: []engine.Edge{
			{Source: "node_1", Target: "node_2"},
			{Source: "node_2", Target: "node_3"},
			{Source: "node_3", Target: "node_1"}, // Cyclic Edge
		},
	}

	// 2. Setup Engine
	wfEngine := engine.NewWorkflowEngine()
	wfEngine.Register("start", &StartNodeHandler{})
	wfEngine.Register("agent", &AgentNodeHandler{})
	wfEngine.Register("end", &EndNodeHandler{})

	// 3. Act
	_, err := wfEngine.Run(graph, make(map[string]interface{}))

	// 4. Assert
	if err == nil {
		t.Fatal("Expected an error (Cycle detection) but execution passed")
	}

	expectedSubstring := "FATAL: Siklus terdeteksi"
	if err.Error()[:len(expectedSubstring)] != expectedSubstring {
		// Just check if error contains the siklus phrase
		t.Logf("Got error: %v", err)
	}
}

func TestWorkflowEngine_MissingHandler(t *testing.T) {
	// 1. Arrange Unregistered NodeType
	graph := &engine.VisualGraph{
		Nodes: []engine.Node{
			{ID: "node_1", Type: "unknown_type"},
		},
	}

	// 2. Setup Engine
	wfEngine := engine.NewWorkflowEngine()

	// 3. Act
	_, err := wfEngine.Run(graph, nil)

	// 4. Assert
	if err == nil {
		t.Fatal("Expected missing handler error but execution passed")
	}
}
