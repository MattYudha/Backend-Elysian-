package routes

import (
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/handler"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(
	router *gin.Engine,
	healthHandler *handler.HealthHandler,
	userHandler *handler.UserHandler,
	authHandler *handler.AuthHandler,
	authMiddleware gin.HandlerFunc,
) {
	// Swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check
	router.GET("/health", healthHandler.Check)

	// API v1
	v1 := router.Group("/api/v1")
	{
		v1.GET("/ping", healthHandler.Ping)

		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authHandler.Logout)
		}

		// Users
		users := v1.Group("/users")
		{
			users.GET("/:id", userHandler.GetByID)
			users.GET("/email/:email", userHandler.GetByEmail)

			protected := users.Group("")
			protected.Use(authMiddleware) // Apply auth middleware
			{
				protected.GET("/me", userHandler.GetMe)       // Get current user
				protected.PUT("/me", userHandler.UpdateMe)    // Update current user
				protected.DELETE("/me", userHandler.DeleteMe) // Delete current user

				// Admin only routes
				admin := protected.Group("")
				admin.Use(middleware.RequireRole("admin"))
				{
					admin.GET("", userHandler.List)
				}
			}
		}
	}
}
