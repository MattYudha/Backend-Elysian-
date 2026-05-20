package handler

import (
	"net/http"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AgentHandler struct {
	repo domain.AgentRepository
}

func NewAgentHandler(repo domain.AgentRepository) *AgentHandler {
	return &AgentHandler{repo: repo}
}

type CreateAgentRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ModelUsed   string `json:"model_used" binding:"required"`
	Status      string `json:"status"`
}

func (h *AgentHandler) CreateAgent(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	status := req.Status
	if status == "" {
		status = "active"
	}

	agent := &domain.Agent{
		ID:          uuid.New(),
		TenantID:    tid,
		Name:        req.Name,
		Description: req.Description,
		ModelUsed:   req.ModelUsed,
		Status:      status,
	}

	if err := h.repo.Create(c.Request.Context(), agent); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": agent})
}

func (h *AgentHandler) ListAgents(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	agents, err := h.repo.List(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": agents})
}

func (h *AgentHandler) GetAgent(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	id := c.Param("id")

	agent, err := h.repo.FindByID(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": agent})
}

func (h *AgentHandler) UpdateAgent(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	id := c.Param("id")

	agent, err := h.repo.FindByID(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	agent.Name = req.Name
	agent.Description = req.Description
	agent.ModelUsed = req.ModelUsed
	if req.Status != "" {
		agent.Status = req.Status
	}

	if err := h.repo.Update(c.Request.Context(), agent); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": agent})
}

func (h *AgentHandler) DeleteAgent(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	id := c.Param("id")

	if err := h.repo.Delete(c.Request.Context(), tenantID, id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Agent deleted"})
}

type CreateSkillRequest struct {
	Name              string `json:"name" binding:"required"`
	ConfigurationJSON string `json:"configuration_json"`
}

func (h *AgentHandler) CreateSkill(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	agentID := c.Param("id")

	// Verify ownership
	_, err := h.repo.FindByID(c.Request.Context(), tenantID, agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
		return
	}

	var req CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	aid, _ := uuid.Parse(agentID)
	configJSON := req.ConfigurationJSON
	if configJSON == "" {
		configJSON = "{}"
	}

	skill := &domain.Skill{
		ID:                uuid.New(),
		AgentID:           aid,
		Name:              req.Name,
		ConfigurationJSON: []byte(configJSON),
	}

	if err := h.repo.CreateSkill(c.Request.Context(), skill); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": skill})
}

func (h *AgentHandler) DeleteSkill(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	agentID := c.Param("id")
	skillID := c.Param("skillId")

	// Verify ownership
	_, err := h.repo.FindByID(c.Request.Context(), tenantID, agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
		return
	}

	if err := h.repo.DeleteSkill(c.Request.Context(), agentID, skillID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Skill deleted"})
}
