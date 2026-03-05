package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/Elysian-Rebirth/backend-go/docs"
	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/handler"
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/routes"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/agent"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/mq"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/parsing"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/storage"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	postgresRepo "github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
	documentUsecase "github.com/Elysian-Rebirth/backend-go/internal/usecase/document"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/rag"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/workflow"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// @title           Elysian Backend API
// @version         1.0.0
// @description     Elysian Backend API provides user authentication, management, and health check endpoints. Built with Go and Gin framework.
// @termsOfService  http://swagger.io/terms/

// @contact.name    API Support
// @contact.url     http://www.swagger.io/support
// @contact.email   support@swagger.io

// @license.name    Apache 2.0
// @license.url     http://www.apache.org/licenses/LICENSE-2.0.html

// @host            localhost:7777
// @BasePath        /

// @schemes         http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded")
	log.Printf("Environment: %s", cfg.Server.Environment)

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := database.HealthCheck(db); err != nil {
		log.Fatalf("Database health check failed: %v", err)
	}
	log.Printf("Database is healthy")

	redisCache, err := cache.NewRedisCache(cfg)
	if err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	log.Printf("Redis connectin established")

	userRepo := postgresRepo.NewUserRepository(db)
	roleRepo := postgresRepo.NewRoleRepository(db, redisCache.(*cache.RedisCache).GetClient())
	_ = roleRepo

	log.Printf("Repositories initialized")

	// Set GIN mode: respect GIN_MODE env var directly, or fall back to IsProduction
	if os.Getenv("GIN_MODE") == "release" || cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())

	// CORS: use AllowOriginFunc for maximum flexibility and reliability.
	// AllowOriginFunc + AllowCredentials: true is the most robust approach.
	// This is compiled into the binary and cannot be broken by Docker cache.
	configOrigins := cfg.Security.CORSAllowedOrigins
	router.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Always allow localhost for development
			if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
				return true
			}
			// Always allow all Vercel preview and production deployments
			if strings.HasSuffix(origin, ".vercel.app") {
				return true
			}
			// Allow config-based origins (from config.yml)
			for _, o := range configOrigins {
				if o == origin {
					return true
				}
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Requested-With", "Accept"},
		ExposeHeaders:    []string{"Content-Length", "Authorization"},
		AllowCredentials: true, // hardcoded — required for login session
		MaxAge:           12 * time.Hour,
	}))
	log.Printf("CORS configured: AllowOriginFunc=*.vercel.app + localhost + %d config origins, AllowCredentials=true", len(configOrigins))

	passwordSvc := auth.NewPasswordService()
	jwtSvc := auth.NewJWTService(cfg.JWT)
	cacheKeyBuilder := cache.NewCacheKeyBuilder("elysian")

	authUseCase := auth.NewAuthUseCase(userRepo, passwordSvc, jwtSvc, redisCache, cacheKeyBuilder)

	healthHandler := handler.NewHealthHandler(cfg, db, redisCache)
	userHandler := handler.NewUserHandler(userRepo)
	authHandler := handler.NewAuthHandler(authUseCase, cfg.IsProduction())

	// Workflow Components
	workflowRepo := postgresRepo.NewWorkflowRepository(db)
	docRepo := postgresRepo.NewDocumentRepository(db)
	auditRepo := postgresRepo.NewAuditRepository(db)
	workflowUseCase := workflow.NewWorkflowUseCase(workflowRepo, docRepo, auditRepo, cfg.AI.GeminiAPIKey)
	workflowHandler := handler.NewWorkflowHandler(workflowUseCase)

	// Infrastructure Components
	agentFactory, err := agent.NewAgentFactory(context.Background(), cfg.AI.GeminiAPIKey, cfg.Redis.Host+":"+cfg.Redis.Port)
	if err != nil {
		log.Printf("[WARN] Agent Factory initialization failed (no Gemini API Key?): %v — DAG engine will run in mock mode", err)
		agentFactory = nil
	} else {
		log.Printf("Agent Factory initialized")
	}

	// Execution Components
	executionRepo := postgresRepo.NewExecutionRepository(db)
	wfEngine := engine.NewWorkflowEngine()
	executionHandler := handler.NewExecutionHandler(wfEngine, executionRepo, workflowRepo)

	// S3 Storage + Document Components
	var documentHandler *handler.DocumentHandler
	asynqClient := mq.NewAsynqClient(cfg)

	s3Service, s3Err := storage.NewS3Service(&cfg.Storage)
	if s3Err != nil {
		log.Printf("[WARN] S3 not configured (%v) — document upload disabled", s3Err)
	} else {
		// Ensure the bucket exists on startup
		if err := s3Service.EnsureBucket(context.Background()); err != nil {
			log.Printf("[WARN] Could not ensure S3 bucket: %v", err)
		}
		// docRepo previously initialized above

		// DocumentUsecase orchestrates presign → DB record → Asynq enqueue
		docUsecase := documentUsecase.NewDocumentUsecase(docRepo, s3Service, asynqClient)
		documentHandler = handler.NewDocumentHandler(docUsecase)

		// Start Asynq Worker with Docling parser + Gemini key from config (never hardcoded)
		asynqWorker := mq.NewAsynqWorker(cfg)
		docParser := parsing.NewDocumentParser(cfg.AI.DoclingURL, cfg.AI.UnstructuredURL)
		docTaskHandler := rag.NewDocumentTaskHandler(docRepo, s3Service, docParser, cfg.AI.GeminiAPIKey)
		asynqWorker.RegisterHandler(rag.TypeProcessDocument, docTaskHandler.Handle)
		go func() {
			if err := asynqWorker.Start(); err != nil {
				log.Printf("[WARN] Asynq Worker failed to start: %v", err)
			}
		}()
		log.Printf("Asynq RAG Worker started (Gemini embedding enabled: %v)", cfg.AI.GeminiAPIKey != "")
	}

	authMiddleware := middleware.AuthMiddleware(jwtSvc, userRepo, roleRepo, redisCache.(*cache.RedisCache).GetClient())

	// RAG Search Handler — uses the already-initialized docRepo and Gemini key from config
	var ragSearchHandler *handler.RAGSearchHandler
	if cfg.AI.GeminiAPIKey != "" {
		ragSearchHandler = handler.NewRAGSearchHandler(postgresRepo.NewDocumentRepository(db), cfg.AI.GeminiAPIKey)
	}

	routes.SetupRoutes(router, healthHandler, userHandler, authHandler, workflowHandler, executionHandler, documentHandler, ragSearchHandler, authMiddleware)

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Printf("Server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulShutdownTimeout)
	defer cancel()

	if agentFactory != nil {
		if err := agentFactory.Close(); err != nil {
			log.Printf("Error closing AgentFactory: %v", err)
		} else {
			log.Printf("AgentFactory (GenAI) connections closed")
		}
	}

	if err := redisCache.Close(); err != nil {
		log.Printf("Error closing Redis: %v", err)
	} else {
		log.Printf("Redis connection closed")
	}

	if err := database.Close(db); err != nil {
		log.Printf("Error closing database: %v", err)
	} else {
		log.Println("Database closed")
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
