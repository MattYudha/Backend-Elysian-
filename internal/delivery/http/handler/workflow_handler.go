package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/dto"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WorkflowHandler struct {
	useCase workflow.WorkflowUseCase
}

func NewWorkflowHandler(useCase workflow.WorkflowUseCase) *WorkflowHandler {
	return &WorkflowHandler{useCase: useCase}
}

// List Workflows
func (h *WorkflowHandler) List(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	workflows, total, err := h.useCase.List(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": workflows,
		"meta": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// Create Workflow
func (h *WorkflowHandler) Create(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	var req dto.SaveWorkflowRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	wf, err := h.useCase.Create(c.Request.Context(), tenantID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": wf})
}

// Get Workflow Details (with latest graph)
func (h *WorkflowHandler) Get(c *gin.Context) {
	id := c.Param("id")

	wf, err := h.useCase.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Workflow not found"})
		return
	}

	// Fetch the latest version (draft or published) to get the graph
	latestVersion, err := h.useCase.GetLatestVersion(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch workflow version"})
		return
	}

	graph := dto.ReactFlowGraphDTO{
		Nodes: []dto.ReactFlowNodeDTO{},
		Edges: []dto.ReactFlowEdgeDTO{},
	}

	if latestVersion != nil && len(latestVersion.Configuration) > 0 {
		var savedGraph dto.SaveWorkflowGraphRequest
		if jsonErr := json.Unmarshal(latestVersion.Configuration, &savedGraph); jsonErr == nil {
			graph.Nodes = savedGraph.Nodes
			graph.Edges = savedGraph.Edges
		}
	}

	response := dto.WorkflowResponse{
		ID:          wf.ID.String(),
		Name:        wf.Name,
		Description: "",
		Status:      string(wf.Status),
		Graph:       graph,
		CreatedAt:   wf.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   wf.CreatedAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": response})
}

// Update Workflow Metadata (PATCH)
func (h *WorkflowHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req dto.SaveWorkflowRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	wf, err := h.useCase.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": wf})
}

// Update Workflow Graph (PUT)
func (h *WorkflowHandler) UpdateGraph(c *gin.Context) {
	id := c.Param("id")
	var req dto.SaveWorkflowGraphRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.useCase.UpdateGraph(c.Request.Context(), id, req); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Workflow graph saved successfully"})
}

// Delete Workflow
func (h *WorkflowHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.useCase.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Workflow deleted"})
}

// Execute Pipeline (POST /workflows/versions/:versionId/execute)
func (h *WorkflowHandler) ExecutePipeline(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)
	tenantID := middleware.MustGetTenantIDFromContext(c)

	_ = user

	versionIDStr := c.Param("versionId")
	versionID, err := uuid.Parse(versionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid version ID format"})
		return
	}

	tid, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid tenant ID format"})
		return
	}

	execCtx, err := h.useCase.ExecutePipeline(c.Request.Context(), tid, user.ID, versionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// For debugging/demo, return the full context payload
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "DAG Pipeline executed successfully",
		"context": execCtx.Payload,
	})
}

// PublishWorkflow validates the DAG and promotes the draft to a published (immutable) version.
func (h *WorkflowHandler) Publish(c *gin.Context) {
	id := c.Param("id")

	if err := h.useCase.PublishWorkflow(c.Request.Context(), id); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no draft") {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: errMsg})
			return
		}
		if strings.Contains(errMsg, "cycle") {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Workflow published successfully",
	})
}
