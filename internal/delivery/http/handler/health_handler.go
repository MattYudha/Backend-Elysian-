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
	cfg *config.Config
	db  *gorm.DB
	cache cache.Cache
}

func NewHealthHandler(cfg *config.Config, db *gorm.DB, cache cache.Cache) *HealthHandler {
	return &HealthHandler{
		cfg: cfg,
		db:  db,
		cache: cache,
	}
}

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

	c.JSON(httpStatus, gin.H{
		"status":      status,
		"environment": h.cfg.Server.Environment,
		"timestamp":   time.Now().Unix(),
		"database": gin.H{
			"healthy": dbHealthy,
			"stats":   dbStats,
		},
		"cache": gin.H{
			"healthy": cacheHealthy,
			"stats": cacheStats,
		},
	})
}

func (h *HealthHandler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
