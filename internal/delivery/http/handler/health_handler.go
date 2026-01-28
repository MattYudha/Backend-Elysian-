package handler

import (
	"net/http"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	cfg   *config.Config
	db    *gorm.DB
	cache cache.Cache
}

func NewHealthHandler(cfg *config.Config, db *gorm.DB, cache cache.Cache) *HealthHandler {
	return &HealthHandler{
		cfg:   cfg,
		db:    db,
		cache: cache,
	}
}

// Request and Response structs

type ErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

type PingResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Status      string                 `json:"status"`
	Environment string                 `json:"environment"`
	Timestamp   int64                  `json:"timestamp"`
	Database    DatabaseHealthResponse `json:"database"`
	Cache       CacheHealthResponse    `json:"cache"`
}

type DatabaseHealthResponse struct {
	Healthy bool                   `json:"healthy"`
	Stats   map[string]interface{} `json:"stats"`
}

type CacheHealthResponse struct {
	Healthy bool                   `json:"healthy"`
	Stats   map[string]interface{} `json:"stats"`
}

// Check godoc
// @Summary      Health Check
// @Description  Check the health of the application (database and cache)
// @Tags         health
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Failure      503  {object}  HealthResponse
// @Router       /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	dbHealthy := true
	if err := database.HealthCheck(h.db); err != nil {
		dbHealthy = false
	}

	cacheHealthy := true
	if err := h.cache.Ping(c.Request.Context()); err != nil {
		cacheHealthy = false
	}

	status := "ok"
	httpStatus := http.StatusOK
	if !dbHealthy || !cacheHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	dbStats, _ := database.GetStats(h.db)

	cacheStats, _ := h.cache.(*cache.RedisCache).GetStats(c.Request.Context())

	c.JSON(httpStatus, HealthResponse{
		Status:      status,
		Environment: h.cfg.Server.Environment,
		Timestamp:   time.Now().Unix(),
		Database: DatabaseHealthResponse{
			Healthy: dbHealthy,
			Stats:   dbStats,
		},
		Cache: CacheHealthResponse{
			Healthy: cacheHealthy,
			Stats:   cacheStats,
		},
	})
}

// Ping godoc
// @Summary      Ping
// @Description  Simple ping endpoint
// @Tags         health
// @Produce      json
// @Success      200  {object}  PingResponse
// @Router       /api/v1/ping [get]
func (h *HealthHandler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, PingResponse{
		Message: "pong",
	})
}
