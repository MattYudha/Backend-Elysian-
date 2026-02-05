package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/agent"
)

type Engine struct {
	repo         domain.ExecutionRepository
	agentFactory *agent.AgentFactory
}

func NewEngine(repo domain.ExecutionRepository, agentFactory *agent.AgentFactory) *Engine {
	return &Engine{repo: repo, agentFactory: agentFactory}
}

// StartAsync initiates the workflow execution in a background goroutine
func (e *Engine) StartAsync(execution *domain.Execution, workflow *domain.Workflow) {
	// Use a background context with a reasonable timeout for the entire execution
	// In production, this might come from configuration
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

	go func() {
		defer cancel() // Release resources when done
		e.run(ctx, execution, workflow)
	}()
}

func (e *Engine) run(ctx context.Context, execution *domain.Execution, wf *domain.Workflow) {
	// Panic Recovery Boundary
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Engine Execution: %v\nStack: %s", r, debug.Stack())
			_ = e.repo.UpdateStatus(ctx, execution.ID, domain.ExecutionStatusFailed, nil)
		}
	}()

	// 1. Update Status to RUNNING
	if err := e.repo.UpdateStatus(ctx, execution.ID, domain.ExecutionStatusRunning, nil); err != nil {
		log.Printf("Failed to update execution status to RUNNING: %v", err)
		return
	}

	// 2. Pre-Execution Graph Validation
	if err := e.validateGraph(wf); err != nil {
		e.logStep(ctx, execution.ID, "SYSTEM", "ERROR", fmt.Sprintf("Graph validation failed: %v", err))
		_ = e.repo.UpdateStatus(ctx, execution.ID, domain.ExecutionStatusFailed, nil)
		return
	}

	// 3. Keep track of processed nodes for Cycle Detection
	processedCount := 0
	totalNodes := len(wf.Nodes)

	// 4. Build Graph (Adjacency List & In-Degree Map)
	adj := make(map[string][]string)     // nodeID -> [neighborIDs]
	parents := make(map[string][]string) // nodeID -> [parentIDs] (for context lookup)
	inDegree := make(map[string]int)     // nodeID -> count of incoming edges
	nodeMap := make(map[string]domain.WorkflowNode)

	for _, node := range wf.Nodes {
		nodeMap[node.ID] = node
		inDegree[node.ID] = 0 // Initialize
	}

	for _, edge := range wf.Edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		parents[edge.TargetNodeID] = append(parents[edge.TargetNodeID], edge.SourceNodeID)
		inDegree[edge.TargetNodeID]++
	}

	// 5. Find Start Nodes (In-Degree 0)
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// 6. Kahn's Algorithm Execution Loop
	nodeOutputs := make(map[string]string)
	finalStatus := domain.ExecutionStatusCompleted

	for len(queue) > 0 {
		// Check for context cancellation/timeout
		select {
		case <-ctx.Done():
			log.Printf("Execution cancelled or timed out: %v", ctx.Err())
			finalStatus = domain.ExecutionStatusFailed
			goto Finish
		default:
		}

		currentID := queue[0]
		queue = queue[1:]
		processedCount++

		// Prepare Inputs from Parents
		inputs := make(map[string]string)
		for _, parentID := range parents[currentID] {
			if out, exists := nodeOutputs[parentID]; exists {
				inputs[parentID] = out
			}
		}

		// Execute Node
		output, err := e.processNode(ctx, execution.ID, nodeMap[currentID], inputs)
		if err != nil {
			log.Printf("Error processing node %s: %v", currentID, err)
			finalStatus = domain.ExecutionStatusFailed
			e.logStep(ctx, execution.ID, currentID, "ERROR", fmt.Sprintf("Node execution failed: %v", err))
			// Failure Containment: Stop scheduling downstream nodes
			goto Finish
		}

		// Store Output
		nodeOutputs[currentID] = output

		// Handle Neighbors (Decrement In-Degree)
		for _, neighborID := range adj[currentID] {
			inDegree[neighborID]--
			if inDegree[neighborID] == 0 {
				queue = append(queue, neighborID)
			}
		}
	}

	// 7. Post-Execution Cycle Check
	if processedCount < totalNodes {
		err := fmt.Errorf("cycle detected or unreachable nodes: processed %d vs total %d", processedCount, totalNodes)
		e.logStep(ctx, execution.ID, "SYSTEM", "ERROR", err.Error())
		finalStatus = domain.ExecutionStatusFailed
	}

Finish:
	// 8. Update Final Status
	if err := e.repo.UpdateStatus(ctx, execution.ID, finalStatus, nil); err != nil {
		log.Printf("Failed to update execution final status: %v", err)
	}
}

// validateGraph checks for basic topology validity
func (e *Engine) validateGraph(wf *domain.Workflow) error {
	nodeIDs := make(map[string]bool)
	for _, node := range wf.Nodes {
		nodeIDs[node.ID] = true
	}

	for _, edge := range wf.Edges {
		if !nodeIDs[edge.SourceNodeID] {
			return fmt.Errorf("edge source %s does not exist", edge.SourceNodeID)
		}
		if !nodeIDs[edge.TargetNodeID] {
			return fmt.Errorf("edge target %s does not exist", edge.TargetNodeID)
		}
		if edge.SourceNodeID == edge.TargetNodeID {
			return fmt.Errorf("self-loop detected on node %s", edge.SourceNodeID)
		}
	}
	return nil
}

func (e *Engine) processNode(ctx context.Context, executionID string, node domain.WorkflowNode, inputs map[string]string) (string, error) {
	// Simulate processing time
	time.Sleep(500 * time.Millisecond)

	label := "Unknown"
	if node.Label != nil {
		label = *node.Label
	}

	// Log the step
	msg := fmt.Sprintf("Executing Node: %s (Type: %s)", label, node.NodeType)
	e.logStep(ctx, executionID, node.ID, "INFO", msg)

	// Placeholder for actual node logic (Switch case on node.NodeType)
	switch node.NodeType {
	case "start":
		return "Workflow Started", nil

	case "debug":
		// Debug node might echo inputs
		return fmt.Sprintf("Debug Echo: %v", inputs), nil

	case "llm":
		// Call AI via Agent SDK
		e.logStep(ctx, executionID, node.ID, "INFO", "Initializing Smart Agent...")

		// Unmarshal Configuration
		var configMap map[string]interface{}
		// Init safe
		configMap = make(map[string]interface{})

		if len(node.Configuration) > 0 {
			if err := json.Unmarshal(node.Configuration, &configMap); err != nil {
				e.logStep(ctx, executionID, node.ID, "WARN", "Config parse error, using defaults")
			}
		}

		// 1. Create Ephemeral Agent
		systemPrompt := "You are a helpful workflow assistant."
		if val, ok := configMap["system_prompt"]; ok {
			systemPrompt = fmt.Sprintf("%v", val)
		}

		worker, err := e.agentFactory.CreateAgent(executionID, systemPrompt)
		if err != nil {
			return "", fmt.Errorf("agent creation failed: %w", err)
		}

		// 2. Construct Prompt
		userPrompt := "Hello AI"
		if val, ok := configMap["prompt"]; ok {
			userPrompt = fmt.Sprintf("%v", val)
		}

		// 3. Deterministic Context Injection (Expert Fix)
		var contextBuilder strings.Builder
		if len(inputs) > 0 {
			contextBuilder.WriteString("CONTEXT FROM PREVIOUS STEPS:\n")

			// A. Collect Keys
			keys := make([]string, 0, len(inputs))
			for k := range inputs {
				keys = append(keys, k)
			}
			// B. Sort Keys (Deterministic Order)
			sort.Strings(keys)

			// C. IterateSorted
			for _, sourceID := range keys {
				output := inputs[sourceID]
				contextBuilder.WriteString(fmt.Sprintf("- Node %s: %s\n", sourceID, output))
			}
			contextBuilder.WriteString("\nUSER INSTRUCTION:\n")
		}
		contextBuilder.WriteString(userPrompt)
		finalPrompt := contextBuilder.String()

		// 4. Execution (Smart Run with Memory)
		resp, err := worker.Run(ctx, finalPrompt)
		if err != nil {
			return "", fmt.Errorf("agent execution failed: %w", err)
		}

		e.logStep(ctx, executionID, node.ID, "INFO", fmt.Sprintf("Agent Response: %s", resp))
		return resp, nil
	}

	return "", nil
}

func (e *Engine) logStep(ctx context.Context, executionID, nodeID, level, message string) {
	err := e.repo.AddLog(ctx, &domain.ExecutionLog{
		ExecutionID: executionID,
		NodeID:      &nodeID,
		Level:       level,
		Message:     message,
	})
	if err != nil {
		// Minimal fallback logging if DB logging fails
		log.Printf("[LOG FAILURE] ExecID=%s NodeID=%s Msg=%s Err=%v", executionID, nodeID, message, err)
	}
}
