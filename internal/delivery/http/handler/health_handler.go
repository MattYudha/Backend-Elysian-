package handler

import (
	"net/http"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewHealthHandler(cfg *config.Config, db *gorm.DB) *HealthHandler {
	return &HealthHandler{
		cfg: cfg,
		db:  db,
	}
}

func (h *HealthHandler) Check(c *gin.Context) {
	dbHealthy := true
	if err := database.HealthCheck(h.db); err != nil {
		dbHealthy = false
	}

	status := "ok"
	httpStatus := http.StatusOK
	if !dbHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	dbStats, _ := database.GetStats(h.db)

	c.JSON(httpStatus, gin.H{
		"status":      status,
		"environment": h.cfg.Server.Environment,
		"timestamp":   time.Now().Unix(),
		"database": gin.H{
			"healthy": dbHealthy,
			"stats":   dbStats,
		},
	})
}

func (h *HealthHandler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
