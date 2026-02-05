package workflow

import (
	"context"
	"encoding/json"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/dto"
	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/datatypes"
)

type WorkflowUseCase interface {
	Create(ctx context.Context, userID string, req dto.SaveWorkflowRequest) (*domain.Workflow, error)
	GetByID(ctx context.Context, id string) (*domain.Workflow, error)
	List(ctx context.Context, userID string, limit, offset int) ([]*domain.Workflow, int64, error)
	Update(ctx context.Context, id string, req dto.SaveWorkflowRequest) (*domain.Workflow, error)
	Delete(ctx context.Context, id string) error
	UpdateGraph(ctx context.Context, id string, req dto.SaveWorkflowGraphRequest) error
}

type workflowUseCase struct {
	repo domain.WorkflowRepository
}

func NewWorkflowUseCase(repo domain.WorkflowRepository) *workflowUseCase {
	return &workflowUseCase{repo: repo}
}

func (uc *workflowUseCase) Create(ctx context.Context, userID string, req dto.SaveWorkflowRequest) (*domain.Workflow, error) {
	workflow := &domain.Workflow{
		UserID:      userID,
		Name:        req.Name,
		Description: &req.Description,
		IsPublic:    req.IsPublic,
		Status:      domain.WorkflowStatusDraft,
	}

	if err := uc.repo.Create(ctx, workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (uc *workflowUseCase) GetByID(ctx context.Context, id string) (*domain.Workflow, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *workflowUseCase) List(ctx context.Context, userID string, limit, offset int) ([]*domain.Workflow, int64, error) {
	return uc.repo.List(ctx, userID, limit, offset)
}

func (uc *workflowUseCase) Update(ctx context.Context, id string, req dto.SaveWorkflowRequest) (*domain.Workflow, error) {
	workflow, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	workflow.Name = req.Name
	workflow.Description = &req.Description
	workflow.IsPublic = req.IsPublic

	if err := uc.repo.Update(ctx, workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (uc *workflowUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *workflowUseCase) UpdateGraph(ctx context.Context, id string, req dto.SaveWorkflowGraphRequest) error {
	var nodes []domain.WorkflowNode
	var edges []domain.WorkflowEdge

	// 1. Convert DTO Nodes to Domain
	for _, n := range req.Nodes {
		// Extract label from data if exists
		label := ""
		if l, ok := n.Data["label"].(string); ok {
			label = l
		}

		// Serialize Data to JSON for DB
		configJSON, err := json.Marshal(n.Data)
		if err != nil {
			configJSON = []byte("{}")
		}

		nodes = append(nodes, domain.WorkflowNode{
			ID:            n.ID,
			WorkflowID:    id,
			NodeKey:       n.ID,
			NodeType:      n.Type,
			Label:         &label,
			PositionX:     n.Position.X,
			PositionY:     n.Position.Y,
			Configuration: datatypes.JSON(configJSON),
		})
	}

	// 2. Convert DTO Edges to Domain
	for _, e := range req.Edges {
		configJSON, err := json.Marshal(e.Data)
		if err != nil {
			configJSON = []byte("{}")
		}

		edges = append(edges, domain.WorkflowEdge{
			ID:            e.ID,
			WorkflowID:    id,
			EdgeKey:       e.ID,
			SourceNodeID:  e.Source,
			TargetNodeID:  e.Target,
			SourceHandle:  e.SourceHandle,
			TargetHandle:  e.TargetHandle,
			Type:          e.Type,
			Animated:      e.Animated,
			Configuration: datatypes.JSON(configJSON),
		})
	}

	// 3. Call Repo Transaction
	return uc.repo.UpdateGraph(ctx, id, nodes, edges)
}
