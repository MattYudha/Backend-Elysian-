package middleware

import (
	"net/http"
	"strings"

	"encoding/json"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func AuthMiddleware(jwtSvc *auth.JWTService, userRepo repository.UserRepository, roleRepo repository.RoleRepository, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		token := parts[1]

		claims, err := jwtSvc.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		user, err := userRepo.FindByID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not found",
			})
			c.Abort()
			return
		}

		// IsActive check removed - field no longer exists in enterprise schema
		// If needed, check tenant_users status instead

		tenantID := c.GetHeader("X-Tenant-ID")

		cacheKey := "auth:rbac:" + tenantID + ":" + user.ID.String()
		var roles []*domain.Role

		// Try Redis First
		cachedRoles, err := redisClient.Get(c.Request.Context(), cacheKey).Result()
		if err == nil && cachedRoles != "" {
			json.Unmarshal([]byte(cachedRoles), &roles)
		} else {
			// Cache Miss
			roles, err = roleRepo.GetUserRoles(c.Request.Context(), tenantID, user.ID.String())
			if err != nil {
				roles = []*domain.Role{}
			} else {
				// Set Cache (Synchronous write to ensure immediate availability)
				rolesBytes, _ := json.Marshal(roles)
				redisClient.Set(c.Request.Context(), cacheKey, rolesBytes, 30*time.Minute)
			}
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)
		c.Set("user_roles", roles)

		c.Next()
	}
}

func OptionalAuth(jwtSvc *auth.JWTService, userRepo repository.UserRepository, roleRepo repository.RoleRepository, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token := parts[1]
		claims, err := jwtSvc.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		user, err := userRepo.FindByID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.Next()
			return
		}

		tenantID := c.GetHeader("X-Tenant-ID")

		cacheKey := "auth:rbac:" + tenantID + ":" + user.ID.String()
		var roles []*domain.Role
		cachedRoles, err := redisClient.Get(c.Request.Context(), cacheKey).Result()
		if err == nil && cachedRoles != "" {
			json.Unmarshal([]byte(cachedRoles), &roles)
		} else {
			roles, _ = roleRepo.GetUserRoles(c.Request.Context(), tenantID, user.ID.String())
			rolesBytes, _ := json.Marshal(roles)
			redisClient.Set(c.Request.Context(), cacheKey, rolesBytes, 30*time.Minute)
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("user_roles", roles)
		c.Next()
	}
}

func GetUserFromContext(c *gin.Context) (*domain.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	u, ok := user.(*domain.User)
	return u, ok
}

func MustGetUserFromContext(c *gin.Context) *domain.User {
	user, exists := GetUserFromContext(c)
	if !exists {
		panic("user not found in context - did you forget AuthMiddleware?")
	}
	return user
}

func GetUserRolesFromContext(c *gin.Context) ([]*domain.Role, bool) {
	roles, exists := c.Get("user_roles")
	if !exists {
		return nil, false
	}

	r, ok := roles.([]*domain.Role)
	return r, ok
}

// MustGetTenantIDFromContext reads X-Tenant-ID from the request header.
// Returns an empty string if the header is absent (not fatal for optional-tenant routes).
func MustGetTenantIDFromContext(c *gin.Context) string {
	return c.GetHeader("X-Tenant-ID")
}
