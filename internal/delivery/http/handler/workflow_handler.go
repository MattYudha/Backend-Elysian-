package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/dto"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/workflow"
	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	useCase workflow.WorkflowUseCase
}

func NewWorkflowHandler(useCase workflow.WorkflowUseCase) *WorkflowHandler {
	return &WorkflowHandler{useCase: useCase}
}

// List Workflows
func (h *WorkflowHandler) List(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	workflows, total, err := h.useCase.List(c.Request.Context(), user.ID, limit, offset)
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
	user := middleware.MustGetUserFromContext(c)
	var req dto.SaveWorkflowRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	wf, err := h.useCase.Create(c.Request.Context(), user.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": wf})
}

// Get Workflow Details
func (h *WorkflowHandler) Get(c *gin.Context) {
	id := c.Param("id")

	wf, err := h.useCase.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Workflow not found"})
		return
	}

	// Prepare Graph DTO for Frontend
	nodes := make([]dto.ReactFlowNodeDTO, 0)
	edges := make([]dto.ReactFlowEdgeDTO, 0)

	for _, n := range wf.Nodes {
		var data map[string]interface{}
		_ = json.Unmarshal(n.Configuration, &data)

		nodes = append(nodes, dto.ReactFlowNodeDTO{
			ID:   n.ID,
			Type: n.NodeType,
			Position: dto.ReactFlowPosition{
				X: n.PositionX,
				Y: n.PositionY,
			},
			Data: data,
		})
	}

	for _, e := range wf.Edges {
		var data map[string]interface{}
		_ = json.Unmarshal(e.Configuration, &data)

		edges = append(edges, dto.ReactFlowEdgeDTO{
			ID:           e.EdgeKey, // ReactFlow expects its own ID key back
			Source:       e.SourceNodeID,
			Target:       e.TargetNodeID,
			SourceHandle: e.SourceHandle,
			TargetHandle: e.TargetHandle,
			Type:         e.Type,
			Animated:     e.Animated,
			Data:         data,
		})
	}

	response := dto.WorkflowResponse{
		ID:          wf.ID,
		Name:        wf.Name,
		Description: *wf.Description,
		Status:      string(wf.Status),
		Graph: dto.ReactFlowGraphDTO{
			Nodes: nodes,
			Edges: edges,
		},
		CreatedAt: wf.CreatedAt.Format(time.RFC3339),
		UpdatedAt: wf.UpdatedAt.Format(time.RFC3339),
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
