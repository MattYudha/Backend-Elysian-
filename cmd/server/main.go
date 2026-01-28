package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Elysian-Rebirth/backend-go/docs"
	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/handler"
	"github.com/Elysian-Rebirth/backend-go/internal/delivery/http/routes"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	postgresRepo "github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
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
	roleRepo := postgresRepo.NewRoleRepository(db)
	_ = roleRepo

	log.Printf("Repositories initialized")

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.Security.CORSAllowedOrigins,
		AllowMethods:     cfg.Security.CORSAllowedMethods,
		AllowHeaders:     cfg.Security.CORSAllowedHeaders,
		AllowCredentials: cfg.Security.CORSAllowCredentials,
		MaxAge:           12 * time.Hour,
	}))

	passwordSvc := auth.NewPasswordService()
	jwtSvc := auth.NewJWTService(cfg.JWT)
	cacheKeyBuilder := cache.NewCacheKeyBuilder("elysian")

	authUseCase := auth.NewAuthUseCase(userRepo, passwordSvc, jwtSvc, redisCache, cacheKeyBuilder)

	healthHandler := handler.NewHealthHandler(cfg, db, redisCache)
	userHandler := handler.NewUserHandler(userRepo)
	authHandler := handler.NewAuthHandler(authUseCase, cfg.IsProduction())

	authMiddleware := middleware.AuthMiddleware(jwtSvc, userRepo, roleRepo)

	routes.SetupRoutes(router, healthHandler, userHandler, authHandler, authMiddleware)

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
