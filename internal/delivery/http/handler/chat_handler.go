package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/minimax"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ChatHandler struct {
	chatRepo       domain.ChatRepository
	docRepo        domain.DocumentRepository
	geminiAPIKey   string
	opencodeAPIKey string
}

func NewChatHandler(chatRepo domain.ChatRepository, docRepo domain.DocumentRepository, geminiAPIKey string, opencodeAPIKey string) *ChatHandler {
	return &ChatHandler{
		chatRepo:       chatRepo,
		docRepo:        docRepo,
		geminiAPIKey:   geminiAPIKey,
		opencodeAPIKey: opencodeAPIKey,
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
	activeKey := h.opencodeAPIKey
	if activeKey == "" {
		activeKey = h.geminiAPIKey
	}

	if activeKey != "" {
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

	// 3. Query Opencode/LiteLLM LLM
	var modelResponse string
	if activeKey != "" {
		minimaxClient := minimax.NewClient(activeKey)
		
		// Get previous message history for conversational memory
		history, _ := h.chatRepo.ListMessages(c.Request.Context(), sessionID)
		
		var mmMessages []minimax.ChatMessage
		
		// Build system instruction or context injection
		systemInstruction := "You are Elysian AI Assistant. Use the provided knowledge base references if applicable."

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
				mmMessages = append(mmMessages, minimax.ChatMessage{
					Role:    "assistant",
					Content: history[i].MessageContent,
				})
			} else {
				mmMessages = append(mmMessages, minimax.ChatMessage{
					Role:    "user",
					Content: history[i].MessageContent,
				})
			}
		}

		// Add current message and RAG context
		currentPrompt := req.Message
		if contextText != "" {
			currentPrompt = currentPrompt + "\n" + contextText
		}
		mmMessages = append(mmMessages, minimax.ChatMessage{
			Role:    "user",
			Content: currentPrompt,
		})

		resp, usage, err := minimaxClient.GenerateContent(c.Request.Context(), systemInstruction, mmMessages)
		if err != nil {
			modelResponse = "Failed to generate AI response: " + err.Error()
		} else {
			modelResponse = resp
			// Log token usage to database time-series ledger
			_ = h.chatRepo.LogTokenUsage(c.Request.Context(), session.TenantID, "MiniMax-M2.5", usage.PromptTokens, usage.CompletionTokens)
		}
	} else {
		modelResponse = "AI Service is not configured (missing Opencode API Key)."
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
	activeKey := h.opencodeAPIKey
	if activeKey == "" {
		activeKey = h.geminiAPIKey
	}
	minimaxClient := minimax.NewClient(activeKey)
	return minimaxClient.EmbedQuery(ctx, query)
}
