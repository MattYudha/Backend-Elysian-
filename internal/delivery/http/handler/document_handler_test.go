package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/handler"
	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MockDocumentUsecase is a mock of domain.DocumentUsecase
type MockDocumentUsecase struct {
	GetUploadURLFunc  func(ctx context.Context, tenantID, userID uuid.UUID, fileName string) (string, string, error)
	ConfirmUploadFunc func(ctx context.Context, tenantID, userID uuid.UUID, title, objectKey, category string) (*domain.Document, error)
	ListDocumentsFunc func(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error)
	ApproveFunc       func(ctx context.Context, tenantID, docID uuid.UUID) error
	DeleteFunc        func(ctx context.Context, tenantID, docID uuid.UUID) error
	UpdateTextFunc    func(ctx context.Context, tenantID, docID uuid.UUID, text string) error
}

func (m *MockDocumentUsecase) GetUploadURL(ctx context.Context, tenantID, userID uuid.UUID, fileName string) (string, string, error) {
	return m.GetUploadURLFunc(ctx, tenantID, userID, fileName)
}

func (m *MockDocumentUsecase) ConfirmUpload(ctx context.Context, tenantID, userID uuid.UUID, title, objectKey, category string) (*domain.Document, error) {
	return m.ConfirmUploadFunc(ctx, tenantID, userID, title, objectKey, category)
}

func (m *MockDocumentUsecase) ListDocuments(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Document, int64, error) {
	return m.ListDocumentsFunc(ctx, tenantID, limit, offset)
}

func (m *MockDocumentUsecase) Approve(ctx context.Context, tenantID, docID uuid.UUID) error {
	return m.ApproveFunc(ctx, tenantID, docID)
}

func (m *MockDocumentUsecase) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, tenantID, docID)
	}
	return nil
}

func (m *MockDocumentUsecase) UpdateText(ctx context.Context, tenantID, docID uuid.UUID, text string) error {
	if m.UpdateTextFunc != nil {
		return m.UpdateTextFunc(ctx, tenantID, docID, text)
	}
	return nil
}

func TestDocumentHandler_Approve(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tenantID := uuid.New()
	docID := uuid.New()

	t.Run("Success Approve Endpoint", func(t *testing.T) {
		uc := &MockDocumentUsecase{
			ApproveFunc: func(ctx context.Context, tID, dID uuid.UUID) error {
				if tID != tenantID || dID != docID {
					t.Errorf("unexpected parameters: tenantID=%v, docID=%v", tID, dID)
				}
				return nil
			},
		}

		h := handler.NewDocumentHandler(uc)
		router := gin.New()
		
		// Setup mock middleware to set tenant ID in context
		router.Use(func(c *gin.Context) {
			c.Set("tenant_id", tenantID)
			c.Next()
		})

		router.POST("/api/v1/documents/:id/approve", h.Approve)

		req := httptest.NewRequest("POST", "/api/v1/documents/"+docID.String()+"/approve", nil)
		req.Header.Set("X-Tenant-ID", tenantID.String())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("expected status 202 Accepted, got %d", w.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response body: %v", err)
		}

		if resp["status"] != "success" || resp["document_id"] != docID.String() {
			t.Errorf("unexpected response body: %v", resp)
		}
	})

	t.Run("Approve Error Response", func(t *testing.T) {
		uc := &MockDocumentUsecase{
			ApproveFunc: func(ctx context.Context, tID, dID uuid.UUID) error {
				return errors.New("approval logic failed")
			},
		}

		h := handler.NewDocumentHandler(uc)
		router := gin.New()
		
		router.Use(func(c *gin.Context) {
			c.Set("tenant_id", tenantID)
			c.Next()
		})

		router.POST("/api/v1/documents/:id/approve", h.Approve)

		req := httptest.NewRequest("POST", "/api/v1/documents/"+docID.String()+"/approve", nil)
		req.Header.Set("X-Tenant-ID", tenantID.String())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500 Internal Server Error, got %d", w.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response body: %v", err)
		}

		if resp["error"] != "approval logic failed" {
			t.Errorf("unexpected error message: %v", resp)
		}
	})
}
