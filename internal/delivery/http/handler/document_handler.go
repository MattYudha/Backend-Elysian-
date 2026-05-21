package handler

import (
	"net/http"
	"strconv"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DocumentHandler struct {
	usecase domain.DocumentUsecase
}

func NewDocumentHandler(usecase domain.DocumentUsecase) *DocumentHandler {
	return &DocumentHandler{usecase: usecase}
}

// Presign godoc
// @Summary      Get Presigned URL for S3 upload
// @Description  Returns a 15-minute presigned S3 URL for direct browser upload (zero-memory transit)
// @Tags         knowledge
// @Param        filename  query  string  true  "Original filename"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/presign [get]
func (h *DocumentHandler) Presign(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "filename query param is required"})
		return
	}

	user := middleware.MustGetUserFromContext(c)
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	presignedURL, objectKey, err := h.usecase.GetUploadURL(c.Request.Context(), tenantID, user.ID, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate presigned URL: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"presigned_url": presignedURL,
		"object_key":    objectKey,
		"expires_in":    "15m",
	})
}

// ConfirmUpload godoc
// @Summary      Confirm S3 upload and trigger vectorization
// @Description  Called by frontend AFTER PUT to S3. Creates DB record and enqueues Asynq vectorization worker.
// @Tags         knowledge
// @Accept       json
// @Produce      json
// @Param        request body ConfirmUploadRequest true "Confirm Upload Request"
// @Success      202  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/confirm [post]
type ConfirmUploadRequest struct {
	Title     string `json:"title" binding:"required"`
	ObjectKey string `json:"object_key" binding:"required"`
	Category  string `json:"category" binding:"required"`
}

func (h *DocumentHandler) ConfirmUpload(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	var req ConfirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	doc, err := h.usecase.ConfirmUpload(c.Request.Context(), tenantID, user.ID, req.Title, req.ObjectKey, req.Category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":      "success",
		"document_id": doc.ID,
		"message":     "Document accepted for processing. Vectorization queued.",
	})
}

// List godoc
// @Summary      List documents for tenant
// @Tags         knowledge
// @Produce      json
// @Param        limit   query  int  false  "Limit"
// @Param        offset  query  int  false  "Offset"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/documents [get]
func (h *DocumentHandler) List(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	docs, total, err := h.usecase.ListDocuments(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": docs,
		"meta": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// Approve godoc
// @Summary      Approve parsed document to begin vectorization
// @Description  Transition status from pending_qa to processing and trigger chunk embedding task.
// @Tags         knowledge
// @Produce      json
// @Param        id   path  string  true  "Document ID"
// @Success      202  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/{id}/approve [post]
func (h *DocumentHandler) Approve(c *gin.Context) {
	docIDStr := c.Param("id")
	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid document ID path parameter"})
		return
	}

	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	err = h.usecase.Approve(c.Request.Context(), tenantID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":      "success",
		"document_id": docID.String(),
		"message":     "Document approved. Vectorization task enqueued.",
	})
}

// Delete godoc
// @Summary      Delete document
// @Description  Deletes a document record from the database.
// @Tags         knowledge
// @Produce      json
// @Param        id   path  string  true  "Document ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/{id} [delete]
func (h *DocumentHandler) Delete(c *gin.Context) {
	docIDStr := c.Param("id")
	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid document ID path parameter"})
		return
	}

	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	err = h.usecase.Delete(c.Request.Context(), tenantID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Document deleted successfully",
	})
}

// UpdateText godoc
// @Summary      Update document extracted text
// @Description  Updates the extracted plain text inside document's ai_analysis_json.
// @Tags         knowledge
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "Document ID"
// @Param        request body map[string]string true "Update text request"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/{id}/text [patch]
func (h *DocumentHandler) UpdateText(c *gin.Context) {
	docIDStr := c.Param("id")
	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid document ID path parameter"})
		return
	}

	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	var req struct {
		ExtractedText string `json:"extracted_text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	err = h.usecase.UpdateText(c.Request.Context(), tenantID, docID, req.ExtractedText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Document text updated successfully",
	})
}
