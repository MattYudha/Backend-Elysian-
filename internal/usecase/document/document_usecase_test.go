package document_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	documentUseCase "github.com/Elysian-Rebirth/backend-go/internal/usecase/document"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// MockDocumentRepository implements domain.DocumentRepository for unit tests
type MockDocumentRepository struct {
	CreateFunc       func(ctx context.Context, doc *domain.Document) error
	UpdateStatusFunc func(ctx context.Context, id uuid.UUID, status string, metadata map[string]interface{}) error
	StoreChunksFunc  func(ctx context.Context, chunks []domain.DocumentChunk) error
	FindByTenantFunc func(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error)
	FindByIDFunc     func(ctx context.Context, id string) (*domain.Document, error)
	HybridSearchFunc func(ctx context.Context, params domain.HybridSearchParams) ([]domain.HybridSearchResult, error)
}

func (m *MockDocumentRepository) Create(ctx context.Context, doc *domain.Document) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, doc)
	}
	return nil
}

func (m *MockDocumentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, metadata map[string]interface{}) error {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, id, status, metadata)
	}
	return nil
}

func (m *MockDocumentRepository) StoreChunks(ctx context.Context, chunks []domain.DocumentChunk) error {
	if m.StoreChunksFunc != nil {
		return m.StoreChunksFunc(ctx, chunks)
	}
	return nil
}

func (m *MockDocumentRepository) FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error) {
	if m.FindByTenantFunc != nil {
		return m.FindByTenantFunc(ctx, tenantID, limit, offset)
	}
	return nil, 0, nil
}

func (m *MockDocumentRepository) FindByID(ctx context.Context, id string) (*domain.Document, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *MockDocumentRepository) HybridSearch(ctx context.Context, params domain.HybridSearchParams) ([]domain.HybridSearchResult, error) {
	if m.HybridSearchFunc != nil {
		return m.HybridSearchFunc(ctx, params)
	}
	return nil, nil
}

// MockTaskQueue implements mq.TaskQueue for unit tests
type MockTaskQueue struct {
	EnqueueTaskFunc func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
	CloseFunc       func() error
}

func (m *MockTaskQueue) EnqueueTask(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if m.EnqueueTaskFunc != nil {
		return m.EnqueueTaskFunc(task, opts...)
	}
	return nil, nil
}

func (m *MockTaskQueue) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestDocumentUseCase_Approve(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()

	t.Run("Success Approval", func(t *testing.T) {
		repo := &MockDocumentRepository{
			FindByIDFunc: func(ctx context.Context, id string) (*domain.Document, error) {
				if id == docID.String() {
					return &domain.Document{
						ID:       docID,
						TenantID: tenantID,
						Status:   "pending_qa",
						Category: "pojk",
					}, nil
				}
				return nil, errors.New("not found")
			},
			UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status string, metadata map[string]interface{}) error {
				if id != docID || status != "processing" {
					t.Errorf("unexpected status update: id=%v, status=%s", id, status)
				}
				return nil
			},
		}

		taskEnqueued := false
		mqClient := &MockTaskQueue{
			EnqueueTaskFunc: func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
				if task.Type() == "rag:embed_document" {
					taskEnqueued = true
				}
				return nil, nil
			},
		}

		uc := documentUseCase.NewDocumentUsecase(repo, nil, mqClient)
		err := uc.Approve(context.Background(), tenantID, docID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !taskEnqueued {
			t.Error("expected embedding task to be enqueued")
		}
	})

	t.Run("Document Not Found", func(t *testing.T) {
		repo := &MockDocumentRepository{
			FindByIDFunc: func(ctx context.Context, id string) (*domain.Document, error) {
				return nil, errors.New("db record not found")
			},
		}
		uc := documentUseCase.NewDocumentUsecase(repo, nil, nil)
		err := uc.Approve(context.Background(), tenantID, docID)
		if err == nil || !strings.Contains(err.Error(), "document not found") {
			t.Fatalf("expected document not found error, got %v", err)
		}
	})

	t.Run("Unauthorized Tenant Access", func(t *testing.T) {
		wrongTenantID := uuid.New()
		repo := &MockDocumentRepository{
			FindByIDFunc: func(ctx context.Context, id string) (*domain.Document, error) {
				return &domain.Document{
					ID:       docID,
					TenantID: wrongTenantID,
					Status:   "pending_qa",
				}, nil
			},
		}
		uc := documentUseCase.NewDocumentUsecase(repo, nil, nil)
		err := uc.Approve(context.Background(), tenantID, docID)
		if err == nil || !strings.Contains(err.Error(), "unauthorized") {
			t.Fatalf("expected unauthorized error, got %v", err)
		}
	})

	t.Run("Invalid Initial Status", func(t *testing.T) {
		repo := &MockDocumentRepository{
			FindByIDFunc: func(ctx context.Context, id string) (*domain.Document, error) {
				return &domain.Document{
					ID:       docID,
					TenantID: tenantID,
					Status:   "ready",
				}, nil
			},
		}
		uc := documentUseCase.NewDocumentUsecase(repo, nil, nil)
		err := uc.Approve(context.Background(), tenantID, docID)
		if err == nil || !strings.Contains(err.Error(), "cannot approve document") {
			t.Fatalf("expected cannot approve error, got %v", err)
		}
	})
}
