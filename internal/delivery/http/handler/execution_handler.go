package handler

import (
	"net/http"
	"strconv"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
	"github.com/gin-gonic/gin"
)

type ExecutionHandler struct {
	engine *engine.WorkflowEngine
	repo   domain.ExecutionRepository
	wfRepo repository.WorkflowRepository
}

func NewExecutionHandler(wfEngine *engine.WorkflowEngine, repo domain.ExecutionRepository, wfRepo repository.WorkflowRepository) *ExecutionHandler {
	return &ExecutionHandler{
		engine: wfEngine,
		repo:   repo,
		wfRepo: wfRepo,
	}
}

// ExecuteWorkflow triggers a new execution
func (h *ExecutionHandler) Execute(c *gin.Context) {
	workflowID := c.Param("id")
	// userID := middleware.MustGetUserFromContext(c).ID // Assuming Auth middleware is used

	// 1. Fetch Workflow Graph
	workflow, err := h.wfRepo.FindByID(c.Request.Context(), workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	// 2. Create Pending Execution Record
	execution := &domain.Execution{
		WorkflowID: workflow.ID.String(),
		Status:     domain.ExecutionStatusPending,
	}

	if err := h.repo.Create(c.Request.Context(), execution); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create execution record"})
		return
	}

	// 3. Trigger - note: full async execution managed by the DAG engine via /versions/:versionId/execute
	// This legacy endpoint only creates the execution record and returns a pending status.

	// 4. Return Immediately
	c.JSON(http.StatusAccepted, gin.H{
		"status":       "pending",
		"execution_id": execution.ID,
		"message":      "Execution record created. Trigger via /workflows/versions/{versionId}/execute for DAG execution.",
	})
}

// GetExecution returns details and logs
func (h *ExecutionHandler) Get(c *gin.Context) {
	id := c.Param("id")

	execution, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Execution not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": execution})
}

// ListExecutions for a workflow
func (h *ExecutionHandler) List(c *gin.Context) {
	workflowID := c.Query("workflow_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	executions, total, err := h.repo.List(c.Request.Context(), workflowID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": executions,
		"meta": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}
