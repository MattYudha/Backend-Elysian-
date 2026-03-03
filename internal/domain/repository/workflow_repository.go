package repository

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
)

type WorkflowRepository interface {
	Create(ctx context.Context, workflow *domain.Workflow) error
	FindByID(ctx context.Context, id string) (*domain.Workflow, error)
	List(ctx context.Context, userID string, limit, offset int) ([]*domain.Workflow, int64, error)
	Update(ctx context.Context, workflow *domain.Workflow) error
	Delete(ctx context.Context, id string) error
	UpdateGraph(ctx context.Context, workflowID string, configuration []byte) error

	GetVersionByID(ctx context.Context, versionID string) (*domain.WorkflowVersion, error)
	CreatePipeline(ctx context.Context, pipeline *domain.Pipeline) error
	UpdatePipeline(ctx context.Context, pipeline *domain.Pipeline) error
}
