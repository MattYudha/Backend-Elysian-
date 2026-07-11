package handler

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
		log.Printf("ERROR in Presign (tenantID: %s, user: %s): %v\n", tenantIDStr, user.ID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
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
// @Summary      Confirm S3 upload and trigger vectorization
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
		log.Printf("ERROR in ConfirmUpload (tenantID: %s, user: %s): %v\n", tenantIDStr, user.ID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
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
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100 // Prevent memory exhaustion / DoS
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	docs, total, err := h.usecase.ListDocuments(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		log.Printf("ERROR in List (tenantID: %s): %v\n", tenantIDStr, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
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
		log.Printf("ERROR in Approve (tenantID: %s, docID: %s): %v\n", tenantIDStr, docIDStr, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
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
		log.Printf("ERROR in Delete (tenantID: %s, docID: %s): %v\n", tenantIDStr, docIDStr, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
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
	user := middleware.MustGetUserFromContext(c)
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	var docID uuid.UUID
	if docID, err = uuid.Parse(docIDStr); err != nil {
		// Non-UUID (e.g. "draft-1"), resolve to deterministic draft UUID
		docID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("draft-"+tenantIDStr+"-"+docIDStr))
	}

	var req struct {
		ExtractedText string `json:"extracted_text"`
		Title         string `json:"title"`
		Status        string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	err = h.usecase.UpdateText(c.Request.Context(), tenantID, user.ID, docID, req.ExtractedText, req.Title, req.Status)
	if err != nil {
		log.Printf("ERROR in UpdateText (tenantID: %s, user: %s, docID: %s): %v\n", tenantIDStr, user.ID, docIDStr, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Document text updated successfully",
	})
}

// GetRaw godoc
// @Summary      Get raw staging document text
// @Description  Returns the full raw text and its SHA-256 hash from MongoDB staging area
// @Tags         knowledge
// @Produce      json
// @Param        id   path  string  true  "Document ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/documents/{id}/raw [get]
func (h *DocumentHandler) GetRaw(c *gin.Context) {
	docIDStr := c.Param("id")
	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}

	var docID uuid.UUID
	if docID, err = uuid.Parse(docIDStr); err != nil {
		// Non-UUID (e.g. "draft-1"), resolve to deterministic draft UUID
		docID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("draft-"+tenantIDStr+"-"+docIDStr))
	}

	text, hash, err := h.usecase.GetRaw(c.Request.Context(), tenantID, docID)
	if err != nil {
		log.Printf("ERROR in GetRaw (tenantID: %s, docID: %s): %v\n", tenantIDStr, docIDStr, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "An internal server error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       docID.String(),
		"raw_text": text,
		"hash":     hash,
	})
}

// MockUpload handles PUT request directly from the browser for local testing when S3 is mocked
func (h *DocumentHandler) MockUpload(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key query parameter is required"})
		return
	}

	// For security in local dev, make sure key does not escape directory (prevent path traversal)
	cleanKey := filepath.Clean(key)
	if filepath.IsAbs(cleanKey) || strings.Contains(cleanKey, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key path"})
		return
	}

	// Create parent directory
	mockFilePath := filepath.Join("tmp", "s3_mock", cleanKey)
	err := os.MkdirAll(filepath.Dir(mockFilePath), 0755)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create local mock directory: " + err.Error()})
		return
	}

	// Save request body to file
	outFile, err := os.Create(mockFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create mock file: " + err.Error()})
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write mock file content: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Mock upload successful",
		"key":     key,
	})
}
