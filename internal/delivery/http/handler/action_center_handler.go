package handler

import (
	"net/http"

	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/action_center"
	"github.com/gin-gonic/gin"
)

type ActionCenterHandler struct {
	useCase action_center.ActionCenterUseCase
}

func NewActionCenterHandler(useCase action_center.ActionCenterUseCase) *ActionCenterHandler {
	return &ActionCenterHandler{useCase: useCase}
}

func (h *ActionCenterHandler) List(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	status := c.Query("status") // pending, resolved, etc.

	items, err := h.useCase.ListActionItems(c.Request.Context(), tenantID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   items,
	})
}

type ResolveRequest struct {
	Justification string `json:"justification" binding:"required"`
}

func (h *ActionCenterHandler) Resolve(c *gin.Context) {
	itemID := c.Param("id")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Action item ID is required"})
		return
	}

	var req ResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request payload: justification is required"})
		return
	}

	user := middleware.MustGetUserFromContext(c)
	userIDStr := user.ID.String()

	err := h.useCase.ResolveActionItem(c.Request.Context(), itemID, userIDStr, req.Justification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Action item resolved successfully",
	})
}
