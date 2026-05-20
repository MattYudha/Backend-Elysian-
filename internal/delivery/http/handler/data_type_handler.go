package handler

import (
	"net/http"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DataTypeHandler struct {
	db *gorm.DB
}

func NewDataTypeHandler(db *gorm.DB) *DataTypeHandler {
	return &DataTypeHandler{db: db}
}

type CreateDataTypeRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func (h *DataTypeHandler) List(c *gin.Context) {
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	var dataTypes []domain.DataType
	// Return custom data types of the tenant AND system data types
	err = h.db.Where("tenant_id = ? OR is_system = ?", tenantID, true).
		Order("is_system DESC, created_at DESC").
		Find(&dataTypes).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// If empty, let's seed default system types dynamically so the user gets a great out-of-box experience!
	if len(dataTypes) == 0 {
		h.seedDefaultSystemTypes(tenantID)
		h.db.Where("tenant_id = ? OR is_system = ?", tenantID, true).
			Order("is_system DESC, created_at DESC").
			Find(&dataTypes)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   dataTypes,
	})
}

func (h *DataTypeHandler) seedDefaultSystemTypes(tenantID uuid.UUID) {
	defaults := []domain.DataType{
		{ID: uuid.New(), TenantID: tenantID, Name: "Task", Description: "Standard unit of work", FieldsCount: 12, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), TenantID: tenantID, Name: "Project", Description: "Container for tasks", FieldsCount: 8, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), TenantID: tenantID, Name: "Document", Description: "Rich text knowledge base", FieldsCount: 3, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, d := range defaults {
		h.db.Create(&d)
	}
}

func (h *DataTypeHandler) Create(c *gin.Context) {
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	var req CreateDataTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dataType := domain.DataType{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		FieldsCount: 0,
		IsSystem:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.Create(&dataType).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data":   dataType,
	})
}

func (h *DataTypeHandler) Delete(c *gin.Context) {
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data type ID"})
		return
	}

	var dataType domain.DataType
	if err := h.db.First(&dataType, "id = ? AND tenant_id = ? AND is_system = ?", id, tenantID, false).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Data type not found or cannot be deleted"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if err := h.db.Delete(&dataType).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data type deleted successfully",
	})
}
