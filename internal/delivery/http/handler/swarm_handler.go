package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/swarm"
	"github.com/gin-gonic/gin"
)

type SwarmHandler struct {
	swarmUsecase *swarm.SwarmUsecase
	redis        cache.Cache
}

func NewSwarmHandler(swarmUsecase *swarm.SwarmUsecase, redis cache.Cache) *SwarmHandler {
	return &SwarmHandler{
		swarmUsecase: swarmUsecase,
		redis:        redis,
	}
}

type TriggerRequest struct {
	DocumentID string                   `json:"document_id" binding:"required"`
	Items      []map[string]interface{} `json:"items" binding:"required"`
}

func (h *SwarmHandler) Trigger(c *gin.Context) {
	var req TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	tenantIDStr := middleware.MustGetTenantIDFromContext(c)
	user := middleware.MustGetUserFromContext(c)
	userIDStr := user.ID.String()

	task, err := h.swarmUsecase.TriggerSwarm(c.Request.Context(), req.DocumentID, req.Items, tenantIDStr, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Swarm review triggered successfully",
		"task_id": task.ID,
		"status":  task.Status,
	})
}

func (h *SwarmHandler) Callback(c *gin.Context) {
	var payload domain.SwarmCallback
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if err := h.swarmUsecase.HandleCallback(c.Request.Context(), payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Callback processed"})
}

func (h *SwarmHandler) StreamEvents(c *gin.Context) {
	// Upgrade connection to SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Allow CORS specifically for SSE if needed, but handled by global CORS

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming unsupported"})
		return
	}

	// For hackathon, we'll subscribe to the Redis PubSub directly in the handler.
	redisCache, ok := h.redis.(*cache.RedisCache)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis not configured"})
		return
	}

	pubsub := redisCache.GetClient().Subscribe(c.Request.Context(), "swarm:events")
	defer pubsub.Close()

	// Wait for events
	ch := pubsub.Channel()

	// Handle client disconnection
	notify := c.Request.Context().Done()

	for {
		select {
		case <-notify:
			// Client disconnected
			return
		case msg := <-ch:
			// Ensure payload is a valid JSON string without double escaping
			var payload interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				// if not JSON, just send the raw string
				c.Writer.Write([]byte("data: " + msg.Payload + "\n\n"))
			} else {
				jsonStr, _ := json.Marshal(payload)
				c.Writer.Write([]byte("data: " + string(jsonStr) + "\n\n"))
			}
			flusher.Flush()
		}
	}
}

func (h *SwarmHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	task, err := h.swarmUsecase.GetSwarmTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   task,
	})
}

// List godoc
// @Summary      List swarm consensus tasks
// @Description  Returns a paginated list of swarm consensus tasks for the tenant.
// @Tags         swarm
// @Produce      json
// @Param        limit   query  int  false  "Limit"
// @Param        offset  query  int  false  "Offset"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /api/v1/swarm/tasks [get]
func (h *SwarmHandler) List(c *gin.Context) {
	tenantID := middleware.MustGetTenantIDFromContext(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	tasks, total, err := h.swarmUsecase.ListSwarmTasks(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   tasks,
		"total":  total,
	})
}

