package handler

import (
	"net/http"
	"strconv"

	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/dashboard"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	useCase dashboard.DashboardUseCase
}

func NewDashboardHandler(useCase dashboard.DashboardUseCase) *DashboardHandler {
	return &DashboardHandler{useCase: useCase}
}

func (h *DashboardHandler) GetStats(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	stats, err := h.useCase.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": stats})
}

func (h *DashboardHandler) GetChartData(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	chartData, err := h.useCase.GetChartData(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": chartData})
}

func (h *DashboardHandler) GetActivityFeed(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	feed, err := h.useCase.GetActivityFeed(c.Request.Context(), tenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": feed})
}

func (h *DashboardHandler) GetAuditLogs(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	logs, err := h.useCase.GetAuditLogs(c.Request.Context(), tenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": logs})
}

func (h *DashboardHandler) GetPriorityQueue(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)

	items, err := h.useCase.GetPriorityQueue(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": items})
}
