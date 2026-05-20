package workflow

import (
	"context"
	"encoding/json"
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
	Create(ctx context.Context, tenantID string, req dto.SaveWorkflowRequest) (*domain.Workflow, error)
	GetByID(ctx context.Context, id string) (*domain.Workflow, error)
	List(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Workflow, int64, error)
	Update(ctx context.Context, id string, req dto.SaveWorkflowRequest) (*domain.Workflow, error)
	Delete(ctx context.Context, id string) error
	UpdateGraph(ctx context.Context, id string, req dto.SaveWorkflowGraphRequest) error
	GetLatestVersion(ctx context.Context, workflowID string) (*domain.WorkflowVersion, error)
	PublishWorkflow(ctx context.Context, id string) error
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

func (uc *workflowUseCase) Create(ctx context.Context, tenantID string, req dto.SaveWorkflowRequest) (*domain.Workflow, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	workflow := &domain.Workflow{
		TenantID: tid,
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

func (uc *workflowUseCase) List(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Workflow, int64, error) {
	return uc.repo.List(ctx, tenantID, limit, offset)
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

// UpdateGraph saves the draft graph configuration.
// NOTE: DAG cycle validation is intentionally NOT performed here.
// Draft workflows are allowed to have incomplete/cyclic connections.
// Validation runs only at Publish time via PublishWorkflow.
func (uc *workflowUseCase) UpdateGraph(ctx context.Context, id string, req dto.SaveWorkflowGraphRequest) error {
	configBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow graph config: %w", err)
	}
	return uc.repo.UpdateGraph(ctx, id, configBytes)
}

func (uc *workflowUseCase) GetLatestVersion(ctx context.Context, workflowID string) (*domain.WorkflowVersion, error) {
	return uc.repo.GetLatestVersion(ctx, workflowID)
}

// PublishWorkflow validates the DAG and locks the current draft into an immutable published version.
// A NEW version row is created regardless of the current status, incrementing version_number.
func (uc *workflowUseCase) PublishWorkflow(ctx context.Context, id string) error {
	// 1. Fetch latest draft version
	latestVersion, err := uc.repo.GetLatestVersion(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to fetch latest version: %w", err)
	}
	if latestVersion == nil {
		return fmt.Errorf("no draft version found to publish")
	}

	// 2. Validate DAG — only here do we enforce no cycles
	graph, err := engine.ParseWorkflow(latestVersion.Configuration)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}
	if _, err := engine.TopologicalSort(graph); err != nil {
		return fmt.Errorf("workflow contains a cycle and cannot be published: %w", err)
	}

	// 3. Update workflow status to published
	wf, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}
	wf.Status = "published"
	return uc.repo.Update(ctx, wf)
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

	// Update Workflow status to processing
	if wf, err := uc.repo.FindByID(ctx, version.WorkflowID.String()); err == nil && wf != nil {
		wf.Status = "processing"
		_ = uc.repo.Update(ctx, wf)
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
		if wf, err := uc.repo.FindByID(ctx, version.WorkflowID.String()); err == nil && wf != nil {
			wf.Status = "failed"
			_ = uc.repo.Update(ctx, wf)
		}
		return nil, fmt.Errorf("eksekusi pipeline gagal: %w", err)
	}

	pipeline.Status = "success"
	uc.repo.UpdatePipeline(ctx, &pipeline)
	if wf, err := uc.repo.FindByID(ctx, version.WorkflowID.String()); err == nil && wf != nil {
		wf.Status = "completed"
		_ = uc.repo.Update(ctx, wf)
	}

	return execCtx, nil
}
