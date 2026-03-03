package workflow

import (
	"context"
	"fmt"

	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/dto"
	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine/handlers"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine/interceptors"
	"github.com/google/uuid"
)

type WorkflowUseCase interface {
	Create(ctx context.Context, userID string, req dto.SaveWorkflowRequest) (*domain.Workflow, error)
	GetByID(ctx context.Context, id string) (*domain.Workflow, error)
	List(ctx context.Context, userID string, limit, offset int) ([]*domain.Workflow, int64, error)
	Update(ctx context.Context, id string, req dto.SaveWorkflowRequest) (*domain.Workflow, error)
	Delete(ctx context.Context, id string) error
	UpdateGraph(ctx context.Context, id string, req dto.SaveWorkflowGraphRequest) error
	ExecutePipeline(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, versionID uuid.UUID) (*engine.ExecutionContext, error)
}

type workflowUseCase struct {
	repo         repository.WorkflowRepository
	docRepo      domain.DocumentRepository
	auditRepo    domain.AuditRepository
	geminiAPIKey string
}

func NewWorkflowUseCase(repo repository.WorkflowRepository, docRepo domain.DocumentRepository, auditRepo domain.AuditRepository, geminiAPIKey string) *workflowUseCase {
	return &workflowUseCase{
		repo:         repo,
		docRepo:      docRepo,
		auditRepo:    auditRepo,
		geminiAPIKey: geminiAPIKey,
	}
}

func (uc *workflowUseCase) Create(ctx context.Context, userID string, req dto.SaveWorkflowRequest) (*domain.Workflow, error) {
	workflow := &domain.Workflow{
		TenantID: uuid.Nil, // Must be correctly set by context later
		Name:     req.Name,
		Status:   "draft",
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

	if err := uc.repo.Update(ctx, workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (uc *workflowUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *workflowUseCase) UpdateGraph(ctx context.Context, id string, req dto.SaveWorkflowGraphRequest) error {
	// 3. Call Repo Transaction
	return uc.repo.UpdateGraph(ctx, id, []byte{})
}

func (uc *workflowUseCase) ExecutePipeline(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, versionID uuid.UUID) (*engine.ExecutionContext, error) {
	// 1. Ambil Workflow Version (JSONB) dari Database
	version, err := uc.repo.GetVersionByID(ctx, versionID.String())
	if err != nil {
		return nil, err
	}

	// 2. Parse JSON Configuration menjadi Graph
	graph, err := engine.ParseWorkflow(version.Configuration)
	if err != nil {
		return nil, fmt.Errorf("gagal mem-parsing skema workflow: %w", err)
	}

	// 3. Catat di Database bahwa Pipeline mulai berjalan
	pipeline := domain.Pipeline{
		TenantID:          tenantID,
		WorkflowVersionID: version.ID,
		Name:              fmt.Sprintf("Execution-%d", time.Now().Unix()),
		Status:            "running",
	}
	if err := uc.repo.CreatePipeline(ctx, &pipeline); err != nil {
		return nil, err
	}

	// 4. Inisialisasi Engine & Daftarkan Handlers + Interceptors
	workflowEngine := engine.NewWorkflowEngine()
	workflowEngine.Register("llm_agent", handlers.NewLLMAgentHandler())

	ragHandler, err := handlers.NewRAGRetrieverHandler(uc.docRepo, uc.geminiAPIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to init RAG retriever: %w", err)
	}
	workflowEngine.Register("rag_retriever", ragHandler)

	// Mount Telemetry and Forensic Audit Interceptors
	workflowEngine.Use(interceptors.NewTelemetryInterceptor())
	workflowEngine.Use(interceptors.NewForensicAuditInterceptor(uc.auditRepo))

	// 5. Eksekusi DAG
	execCtx, err := workflowEngine.Run(graph, map[string]interface{}{
		"tenant_id": tenantID.String(),
		"user_id":   userID.String(),
	})

	// 6. Finalisasi & Update Log Database (Wajib untuk Observabilitas)
	duration := time.Since(pipeline.StartedAt).Milliseconds()
	pipeline.ExecutionTimeMs = int(duration)
	now := time.Now()
	pipeline.CompletedAt = &now

	if err != nil {
		pipeline.Status = "failed"
		uc.repo.UpdatePipeline(ctx, &pipeline) // Simpan status gagal
		return nil, fmt.Errorf("eksekusi pipeline gagal: %w", err)
	}

	pipeline.Status = "success"
	uc.repo.UpdatePipeline(ctx, &pipeline)

	return execCtx, nil
}
