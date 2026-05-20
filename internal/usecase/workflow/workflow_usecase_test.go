package workflow_test

import (
	"context"
	"testing"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/dto"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/workflow"
)

// MockWorkflowRepo implements repository.WorkflowRepository
type MockWorkflowRepo struct {
	repository.WorkflowRepository
	updateGraphCalled bool
	savedConfig       []byte
}

func (m *MockWorkflowRepo) UpdateGraph(ctx context.Context, id string, configuration []byte) error {
	m.updateGraphCalled = true
	m.savedConfig = configuration
	return nil
}

func TestWorkflowUseCase_UpdateGraph_Success(t *testing.T) {
	mockRepo := &MockWorkflowRepo{}
	usecase := workflow.NewWorkflowUseCase(mockRepo, nil, nil, "")

	req := dto.SaveWorkflowGraphRequest{
		Nodes: []dto.ReactFlowNodeDTO{
			{ID: "node_1", Type: "llm_agent"},
			{ID: "node_2", Type: "llm_agent"},
		},
		Edges: []dto.ReactFlowEdgeDTO{
			{ID: "edge_1", Source: "node_1", Target: "node_2"},
		},
	}

	err := usecase.UpdateGraph(context.Background(), "wf_123", req)
	if err != nil {
		t.Fatalf("Expected UpdateGraph to succeed on valid DAG, got error: %v", err)
	}

	if !mockRepo.updateGraphCalled {
		t.Error("Expected repo.UpdateGraph to be called on success")
	}
}

func TestWorkflowUseCase_UpdateGraph_CycleError(t *testing.T) {
	mockRepo := &MockWorkflowRepo{}
	usecase := workflow.NewWorkflowUseCase(mockRepo, nil, nil, "")

	req := dto.SaveWorkflowGraphRequest{
		Nodes: []dto.ReactFlowNodeDTO{
			{ID: "node_1", Type: "llm_agent"},
			{ID: "node_2", Type: "llm_agent"},
		},
		Edges: []dto.ReactFlowEdgeDTO{
			{ID: "edge_1", Source: "node_1", Target: "node_2"},
			{ID: "edge_2", Source: "node_2", Target: "node_1"}, // Cycle
		},
	}

	err := usecase.UpdateGraph(context.Background(), "wf_123", req)
	if err == nil {
		t.Fatal("Expected UpdateGraph to fail on cyclic graph, got nil error")
	}

	if mockRepo.updateGraphCalled {
		t.Error("Expected repo.UpdateGraph to NOT be called on cyclic validation error")
	}
}
