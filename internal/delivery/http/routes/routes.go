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
	swarmHandler *handler.SwarmHandler,
	dashboardHandler *handler.DashboardHandler,
	chatHandler *handler.ChatHandler,
	agentHandler *handler.AgentHandler,
	tenantHandler *handler.TenantHandler,
	dataTypeHandler *handler.DataTypeHandler,
	blockchainHandler *handler.BlockchainHandler,
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
					protected.GET("/me", userHandler.GetMe)
					protected.PUT("/me", userHandler.UpdateMe)
					protected.DELETE("/me", userHandler.DeleteMe)
					protected.PUT("/me/password", userHandler.UpdatePassword)
					protected.GET("/me/preferences", userHandler.GetPreferences)
					protected.PUT("/me/preferences", userHandler.UpdatePreferences)

					// Admin only routes
					admin := protected.Group("")
					admin.Use(middleware.RequireRole("admin"))
					{
						admin.GET("", userHandler.List)
					}
				}
			}

			// Tenants
			tenants := v1.Group("/tenants")
			tenants.Use(authMiddleware)
			{
				tenants.GET("", tenantHandler.ListMyTenants)
				tenants.POST("", tenantHandler.CreateTenant)
				tenants.GET("/:id", tenantHandler.GetByID)
				tenants.GET("/:id/members", tenantHandler.GetMembers)
				tenants.PUT("/:id", tenantHandler.UpdateTenant)
				tenants.PUT("/:id/members/:userId", tenantHandler.UpdateMemberRole)
			}

			// Data Types (Strict Multi-Tenancy Enforced)
			dataTypes := v1.Group("/data-types")
			dataTypes.Use(authMiddleware, middleware.TenantMiddleware())
			{
				dataTypes.GET("", dataTypeHandler.List)
				dataTypes.POST("", dataTypeHandler.Create)
				dataTypes.DELETE("/:id", dataTypeHandler.Delete)
			}

			// Workflows (Strict Multi-Tenancy Enforced)
			workflows := v1.Group("/workflows")
			workflows.Use(authMiddleware, middleware.TenantMiddleware())
			{
				workflows.GET("", workflowHandler.List)
				workflows.POST("", workflowHandler.Create)
				workflows.GET("/:id", workflowHandler.Get)
				workflows.PATCH("/:id", workflowHandler.Update)          // Metadata update
				workflows.PUT("/:id/graph", workflowHandler.UpdateGraph) // Canonical Graph update
				workflows.DELETE("/:id", workflowHandler.Delete)
				workflows.POST("/:id/publish", workflowHandler.Publish)

				// Execution Trigger
				workflows.POST("/:id/execute", executionHandler.Execute)

				// DAG Pipeline Execute
				workflows.POST("/versions/:versionId/execute", workflowHandler.ExecutePipeline)
			}

			// Executions (Strict Multi-Tenancy Enforced)
			executions := v1.Group("/executions")
			executions.Use(authMiddleware, middleware.TenantMiddleware())
			{
				executions.GET("/:id", executionHandler.Get)
				executions.GET("", executionHandler.List)
			}

			// Documents (Strict Multi-Tenancy Enforced)
			docs := v1.Group("/documents")
			docs.Use(authMiddleware, middleware.TenantMiddleware())
			{
				docs.GET("/presign", documentHandler.Presign)
				docs.POST("/confirm", documentHandler.ConfirmUpload)
				docs.GET("", documentHandler.List)
				docs.POST("/search", ragSearchHandler.Search)
				docs.POST("/:id/approve", documentHandler.Approve)
			}

			// Swarm (Strict Multi-Tenancy Enforced for Trigger)
			swarm := v1.Group("/swarm")
			{
				swarm.POST("/callback", swarmHandler.Callback)
				swarm.GET("/events", swarmHandler.StreamEvents)

				protectedSwarm := swarm.Group("")
				protectedSwarm.Use(authMiddleware, middleware.TenantMiddleware())
				{
					protectedSwarm.POST("/upload", swarmHandler.Trigger)
					protectedSwarm.GET("/tasks/:id", swarmHandler.GetByID)
				}
			}

			// Blockchain Verification (Strict Multi-Tenancy Enforced)
			blockchain := v1.Group("/blockchain")
			blockchain.Use(authMiddleware, middleware.TenantMiddleware())
			{
				blockchain.GET("/status/:task_id", blockchainHandler.GetStatus)
				blockchain.GET("/verify/:task_id", blockchainHandler.Verify)
			}

			// Dashboard (Strict Multi-Tenancy Enforced)
			dashboard := v1.Group("/dashboard")
			dashboard.Use(authMiddleware, middleware.TenantMiddleware())
			{
				dashboard.GET("/stats", dashboardHandler.GetStats)
				dashboard.GET("/charts", dashboardHandler.GetChartData)
				dashboard.GET("/audit-logs", dashboardHandler.GetAuditLogs)
				dashboard.GET("/priority-queue", dashboardHandler.GetPriorityQueue)
			}

			// Activity Feed (Strict Multi-Tenancy Enforced)
			v1.GET("/activity", authMiddleware, middleware.TenantMiddleware(), dashboardHandler.GetActivityFeed)

			// Chat (Strict Multi-Tenancy Enforced)
			chat := v1.Group("/chat")
			chat.Use(authMiddleware, middleware.TenantMiddleware())
			{
				chat.POST("/sessions", chatHandler.CreateSession)
				chat.GET("/sessions", chatHandler.ListSessions)
				chat.DELETE("/sessions/:id", chatHandler.DeleteSession)
				chat.GET("/sessions/:id/messages", chatHandler.GetMessages)
				chat.POST("/sessions/:id/messages", chatHandler.SendMessage)
			}

			// Agent (Strict Multi-Tenancy Enforced)
			agents := v1.Group("/agents")
			agents.Use(authMiddleware, middleware.TenantMiddleware())
			{
				agents.GET("", agentHandler.ListAgents)
				agents.POST("", agentHandler.CreateAgent)
				agents.GET("/:id", agentHandler.GetAgent)
				agents.PUT("/:id", agentHandler.UpdateAgent)
				agents.DELETE("/:id", agentHandler.DeleteAgent)
				agents.POST("/:id/skills", agentHandler.CreateSkill)
				agents.DELETE("/:id/skills/:skillId", agentHandler.DeleteSkill)
			}
		}
	}
}
