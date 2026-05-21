package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

type ChatHandler struct {
	chatRepo     domain.ChatRepository
	docRepo      domain.DocumentRepository
	geminiAPIKey string
}

func NewChatHandler(chatRepo domain.ChatRepository, docRepo domain.DocumentRepository, geminiAPIKey string) *ChatHandler {
	return &ChatHandler{
		chatRepo:     chatRepo,
		docRepo:      docRepo,
		geminiAPIKey: geminiAPIKey,
	}
}

type CreateSessionRequest struct {
	Title string `json:"title"`
}

func (h *ChatHandler) CreateSession(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "User not authenticated"})
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	title := req.Title
	if title == "" {
		title = "New Chat Session"
	}

	tid, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid X-Tenant-ID header"})
		return
	}
	uid := userID.(uuid.UUID)

	session := &domain.ChatSession{
		ID:        uuid.New(),
		TenantID:  tid,
		UserID:    uid,
		Title:     title,
		CreatedAt: time.Now(),
	}

	if err := h.chatRepo.CreateSession(c.Request.Context(), session); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": session})
}

func (h *ChatHandler) ListSessions(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "User not authenticated"})
		return
	}

	uid := userID.(uuid.UUID).String()

	sessions, err := h.chatRepo.ListSessions(c.Request.Context(), tenantID, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": sessions})
}

func (h *ChatHandler) DeleteSession(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	sessionID := c.Param("id")

	if err := h.chatRepo.DeleteSession(c.Request.Context(), tenantID, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Session deleted"})
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	sessionID := c.Param("id")

	// Ensure session belongs to tenant
	_, err := h.chatRepo.GetSession(c.Request.Context(), tenantID, sessionID)
	if err != nil {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "Unauthorized or session not found"})
		return
	}

	messages, err := h.chatRepo.ListMessages(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": messages})
}

type SendMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	sessionID := c.Param("id")

	// Verify session ownership
	session, err := h.chatRepo.GetSession(c.Request.Context(), tenantID, sessionID)
	if err != nil {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "Unauthorized or session not found"})
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// 1. Save user message to database
	userMsg := &domain.ChatMessage{
		ID:             uuid.New(),
		SessionID:      session.ID,
		SenderRole:     "user",
		MessageContent: req.Message,
		CreatedAt:      time.Now(),
	}
	if err := h.chatRepo.CreateMessage(c.Request.Context(), userMsg); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save message: " + err.Error()})
		return
	}

	// 2. Perform RAG query enhancement if API Key is available
	var contextText string
	if h.geminiAPIKey != "" {
		embedding, err := h.getQueryEmbedding(c.Request.Context(), req.Message)
		if err == nil {
			results, err := h.docRepo.HybridSearch(c.Request.Context(), domain.HybridSearchParams{
				TenantID:       tenantID,
				QueryText:      req.Message,
				QueryEmbedding: embedding,
				TopK:           3,
				EfSearch:       50,
				RRFConstant:    60,
			})
			if err == nil && len(results) > 0 {
				contextText = "\nKnowledge base reference:\n"
				for _, res := range results {
					contextText += fmt.Sprintf("- From document '%s': %s\n", res.DocumentTitle, res.Content)
				}
			}
		}
	}

	// 3. Query Gemini LLM
	var modelResponse string
	if h.geminiAPIKey != "" {
		geminiClient, err := genai.NewClient(c.Request.Context(), option.WithAPIKey(h.geminiAPIKey))
		if err != nil {
			modelResponse = "Error connecting to AI engine: " + err.Error()
		} else {
			defer geminiClient.Close()
			model := geminiClient.GenerativeModel("gemini-2.5-flash")
			
			// Get previous message history for conversational memory
			history, _ := h.chatRepo.ListMessages(c.Request.Context(), sessionID)
			var parts []genai.Part
			
			// Build system instruction or context injection
			systemInstruction := "You are Elysian AI Assistant. Use the provided knowledge base references if applicable."
			model.SystemInstruction = &genai.Content{
				Parts: []genai.Part{genai.Text(systemInstruction)},
			}

			// Add conversation history
			// Limit history to last 10 messages to avoid context explosion
			startIdx := 0
			if len(history) > 10 {
				startIdx = len(history) - 10
			}
			for i := startIdx; i < len(history); i++ {
				// Don't duplicate the current user message which is already in database but not yet processed by LLM
				if history[i].ID == userMsg.ID {
					continue
				}
				role := history[i].SenderRole
				if role == "model" || role == "assistant" {
					parts = append(parts, genai.Text(fmt.Sprintf("Assistant: %s", history[i].MessageContent)))
				} else {
					parts = append(parts, genai.Text(fmt.Sprintf("User: %s", history[i].MessageContent)))
				}
			}

			// Add current message and RAG context
			currentPrompt := req.Message
			if contextText != "" {
				currentPrompt = currentPrompt + "\n" + contextText
			}
			parts = append(parts, genai.Text(fmt.Sprintf("User: %s", currentPrompt)))

			resp, err := model.GenerateContent(c.Request.Context(), parts...)
			if err != nil {
				modelResponse = "Failed to generate AI response: " + err.Error()
			} else {
				if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
					if txt, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
						modelResponse = string(txt)
					} else {
						modelResponse = "Unsupported format returned by AI engine."
					}
				} else {
					modelResponse = "No response generated by AI engine."
				}
			}
		}
	} else {
		modelResponse = "AI Service is not configured (missing Gemini API Key)."
	}

	// 4. Save model response to database
	modelMsg := &domain.ChatMessage{
		ID:             uuid.New(),
		SessionID:      session.ID,
		SenderRole:     "model",
		MessageContent: modelResponse,
		CreatedAt:      time.Now(),
	}
	if err := h.chatRepo.CreateMessage(c.Request.Context(), modelMsg); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save AI response: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": modelMsg})
}

func (h *ChatHandler) getQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(h.geminiAPIKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	em := client.EmbeddingModel("text-embedding-004")
	resp, err := em.EmbedContent(ctx, genai.Text(query))
	if err != nil {
		return nil, err
	}
	return resp.Embedding.Values, nil
}
