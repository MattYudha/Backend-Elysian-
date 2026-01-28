package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userRoles, exists := GetUserRolesFromContext(c)
		if !exists || len(userRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRoles {
				if strings.EqualFold(userRole.Name, requiredRole) {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":         "Insufficient permissions",
				"required_role": roles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequireAnyRole(roles ...string) gin.HandlerFunc {
	return RequireRole(roles...)
}

func RequireAllRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userRoles, exists := GetUserRolesFromContext(c)
		if !exists || len(userRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		userRoleMap := make(map[string]bool)
		for _, userRole := range userRoles {
			userRoleMap[strings.ToLower(userRole.Name)] = true
		}

		for _, requiredRole := range roles {
			if !userRoleMap[strings.ToLower(requiredRole)] {
				c.JSON(http.StatusForbidden, gin.H{
					"error":          "Insufficient permissions",
					"required_roles": roles,
					"missing_role":   requiredRole,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func RequirePermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userRoles, exists := GetUserRolesFromContext(c)
		if !exists || len(userRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		userPermissions := make(map[string]bool)
		for _, role := range userRoles {
			perms := role.GetPermissions()
			for _, perm := range perms {
				userPermissions[perm] = true
			}
		}

		hasAllPermissions := true
		missingPermissions := []string{}

		for _, requiredPerm := range permissions {
			if !userPermissions[requiredPerm] && !userPermissions["*"] {
				hasAllPermissions = false
				missingPermissions = append(missingPermissions, requiredPerm)
			}
		}

		if !hasAllPermissions {
			c.JSON(http.StatusForbidden, gin.H{
				"error":                "Insufficient permissions",
				"required_permissions": permissions,
				"missing_permissions":  missingPermissions,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequireOwnership(resourceType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		resourceID := c.Param("id")
		if resourceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Resource ID is required",
			})
			c.Abort()
			return
		}

		c.Set("resource_type", resourceType)
		c.Set("resource_id", resourceID)
		c.Set("ownership_checked", true)

		c.Next()
	}
}

func CheckOwnership(c *gin.Context, resourceUserID string) bool {
	user := MustGetUserFromContext(c)

	ownershipChecked, exists := c.Get("ownership_checked")
	if !exists || !ownershipChecked.(bool) {
		return false
	}

	return user.ID == resourceUserID
}

func MustCheckOwnership(c *gin.Context, resourceUserID string) {
	if !CheckOwnership(c, resourceUserID) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You don't have permission to access this resource",
		})
		c.Abort()
	}
}
