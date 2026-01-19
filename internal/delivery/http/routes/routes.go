package routes

import (
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/handler"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(
	router *gin.Engine,
	healthHandler *handler.HealthHandler,
	userHandler *handler.UserHandler,
) {
	// Health check
	router.GET("/health", healthHandler.Check)

	// API v1
	v1 := router.Group("/api/v1")
	{
		v1.GET("/ping", healthHandler.Ping)

		// Users
		users := v1.Group("/users")
		{
			users.GET("", userHandler.List)
			users.GET("/:id", userHandler.GetByID)
			users.GET("/email/:email", userHandler.GetByEmail)
		}
	}
}
