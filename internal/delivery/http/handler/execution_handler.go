package handler

import (
	"net/http"
	"strconv"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
	"github.com/gin-gonic/gin"
)

type ExecutionHandler struct {
	engine *engine.Engine
	repo   domain.ExecutionRepository
	wfRepo domain.WorkflowRepository
}

func NewExecutionHandler(engine *engine.Engine, repo domain.ExecutionRepository, wfRepo domain.WorkflowRepository) *ExecutionHandler {
	return &ExecutionHandler{
		engine: engine,
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
		WorkflowID: workflow.ID,
		UserID:     workflow.UserID, // Or current user if different
		Status:     domain.ExecutionStatusPending,
	}

	if err := h.repo.Create(c.Request.Context(), execution); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create execution record"})
		return
	}

	// 3. Start Async Engine
	// Pass a copy or ensure thread safety if modifying workflow (here we just read)
	h.engine.StartAsync(execution, workflow)

	// 4. Return Immediately
	c.JSON(http.StatusAccepted, gin.H{
		"status":       "pending",
		"execution_id": execution.ID,
		"message":      "Workflow execution started",
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
