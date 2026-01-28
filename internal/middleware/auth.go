package middleware

import (
	"net/http"
	"strings"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(jwtSvc *auth.JWTService, userRepo repository.UserRepository, roleRepo repository.RoleRepository) gin.HandlerFunc {
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

		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Account is disabled",
			})
			c.Abort()
			return
		}

		roles, err := roleRepo.GetUserRoles(c.Request.Context(), user.ID)
		if err != nil {
			roles = []*domain.Role{}
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)
		c.Set("user_roles", roles)

		c.Next()
	}
}

func OptionalAuth(jwtSvc *auth.JWTService, userRepo repository.UserRepository, roleRepo repository.RoleRepository) gin.HandlerFunc {
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

		roles, _ := roleRepo.GetUserRoles(c.Request.Context(), user.ID)

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
