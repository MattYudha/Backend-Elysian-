package workflow_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/dto"
	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/workflow"
)

// MockWorkflowRepo implements repository.WorkflowRepository
type MockWorkflowRepo struct {
	repository.WorkflowRepository
	updateGraphCalled bool
	savedConfig       []byte
	latestVersion     *domain.WorkflowVersion
	workflow          *domain.Workflow
	updateCalled      bool
}

func (m *MockWorkflowRepo) UpdateGraph(ctx context.Context, id string, configuration []byte) error {
	m.updateGraphCalled = true
	m.savedConfig = configuration
	return nil
}

func (m *MockWorkflowRepo) GetLatestVersion(ctx context.Context, workflowID string) (*domain.WorkflowVersion, error) {
	return m.latestVersion, nil
}

func (m *MockWorkflowRepo) FindByID(ctx context.Context, id string) (*domain.Workflow, error) {
	return m.workflow, nil
}

func (m *MockWorkflowRepo) Update(ctx context.Context, workflow *domain.Workflow) error {
	m.updateCalled = true
	m.workflow = workflow
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

func TestWorkflowUseCase_PublishWorkflow_Success(t *testing.T) {
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
	configBytes, _ := json.Marshal(req)

	mockRepo.latestVersion = &domain.WorkflowVersion{
		Configuration: configBytes,
	}
	mockRepo.workflow = &domain.Workflow{
		Name:   "Test Workflow",
		Status: "draft",
	}

	err := usecase.PublishWorkflow(context.Background(), "wf_123")
	if err != nil {
		t.Fatalf("Expected PublishWorkflow to succeed on valid DAG, got error: %v", err)
	}

	if !mockRepo.updateCalled {
		t.Error("Expected repo.Update to be called on success")
	}

	if mockRepo.workflow.Status != "published" {
		t.Errorf("Expected workflow status to be 'published', got '%s'", mockRepo.workflow.Status)
	}
}

func TestWorkflowUseCase_PublishWorkflow_CycleError(t *testing.T) {
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
	configBytes, _ := json.Marshal(req)

	mockRepo.latestVersion = &domain.WorkflowVersion{
		Configuration: configBytes,
	}
	mockRepo.workflow = &domain.Workflow{
		Name:   "Test Workflow",
		Status: "draft",
	}

	err := usecase.PublishWorkflow(context.Background(), "wf_123")
	if err == nil {
		t.Fatal("Expected PublishWorkflow to fail on cyclic graph, got nil error")
	}

	if mockRepo.updateCalled {
		t.Error("Expected repo.Update to NOT be called on cyclic validation error")
	}
}
