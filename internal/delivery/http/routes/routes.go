package routes

import (
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/handler"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(
	router *gin.Engine,
	healthHandler *handler.HealthHandler,
	userHandler *handler.UserHandler,
	authHandler *handler.AuthHandler,
	workflowHandler *handler.WorkflowHandler,
	executionHandler *handler.ExecutionHandler,
	documentHandler *handler.DocumentHandler,
	ragSearchHandler *handler.RAGSearchHandler,
	authMiddleware gin.HandlerFunc,
) {
	// Swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check
	router.GET("/health", healthHandler.Check)

	// Prometheus Metrics Exporter
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API Root Group
	api := router.Group("/api")
	{
		// API v1
		v1 := api.Group("/v1")
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

			// Workflows
			workflows := v1.Group("/workflows")
			workflows.Use(authMiddleware)
			{
				workflows.GET("", workflowHandler.List)
				workflows.POST("", workflowHandler.Create)
				workflows.GET("/:id", workflowHandler.Get)
				workflows.PATCH("/:id", workflowHandler.Update)          // Metadata update
				workflows.PUT("/:id/graph", workflowHandler.UpdateGraph) // Canonical Graph update
				workflows.DELETE("/:id", workflowHandler.Delete)

				// Execution Trigger (legacy)
				workflows.POST("/:id/execute", executionHandler.Execute)

				// DAG Pipeline Execute (New Engine)
				workflows.POST("/versions/:versionId/execute", workflowHandler.ExecutePipeline)
			}

			// Executions (Global or specific)
			executions := v1.Group("/executions")
			executions.Use(authMiddleware)
			{
				executions.GET("/:id", executionHandler.Get)
				executions.GET("", executionHandler.List)
			}

			// Documents (Knowledge Base RAG)
			docs := v1.Group("/documents")
			docs.Use(authMiddleware)
			{
				docs.GET("/presign", documentHandler.Presign)        // Step 1: Get upload URL
				docs.POST("/confirm", documentHandler.ConfirmUpload) // Step 2: Confirm upload
				docs.GET("", documentHandler.List)                   // List all docs
				docs.POST("/search", ragSearchHandler.Search)        // Hybrid RAG search (HNSW+FTS+RRF)
			}
		}
	}
}
