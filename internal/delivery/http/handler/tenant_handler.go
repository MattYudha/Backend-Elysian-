package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantHandler struct {
	db *gorm.DB
}

func NewTenantHandler(db *gorm.DB) *TenantHandler {
	return &TenantHandler{db: db}
}

type TenantTheme struct {
	PrimaryColor string `json:"primaryColor"`
	DarkMode     bool   `json:"darkMode"`
}

type TenantJSONResponse struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Slug        string       `json:"slug"`
	Logo        string       `json:"logo,omitempty"`
	Theme       *TenantTheme `json:"theme,omitempty"`
	Features    []string     `json:"features"`
	PlanTier    string       `json:"plan_tier"`
	Status      string       `json:"status"`
	HealthScore int          `json:"health_score"`
}

func ToTenantJSON(t domain.Tenant) TenantJSONResponse {
	slug := strings.ToLower(strings.ReplaceAll(t.Name, " ", "-"))
	return TenantJSONResponse{
		ID:          t.ID.String(),
		Name:        t.Name,
		Slug:        slug,
		Features:    []string{"workflows", "agents", "documents", "chat"},
		PlanTier:    t.PlanTier,
		Status:      t.Status,
		HealthScore: t.HealthScore,
		Theme: &TenantTheme{
			PrimaryColor: "#0284c7",
			DarkMode:     true,
		},
	}
}

func (h *TenantHandler) ListMyTenants(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)

	var tenants []domain.Tenant
	err := h.db.Table("tenants").
		Joins("JOIN tenant_users ON tenants.id = tenant_users.tenant_id").
		Where("tenant_users.user_id = ?", user.ID).
		Find(&tenants).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var data []TenantJSONResponse
	for _, t := range tenants {
		data = append(data, ToTenantJSON(t))
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data,
	})
}

func (h *TenantHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}

	var tenant domain.Tenant
	if err := h.db.First(&tenant, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   ToTenantJSON(tenant),
	})
}

type CreateTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *TenantHandler) CreateTenant(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)

	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := domain.Tenant{
		ID:           uuid.New(),
		Name:         req.Name,
		PlanTier:     "free",
		Status:       "active",
		HealthScore:  100,
		BillingCycle: "monthly",
		CreatedAt:    time.Now(),
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		// 1. Create tenant
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}

		// 2. Find or create user role for this tenant (admin role)
		var adminRole domain.Role
		err := tx.Where("tenant_id = ? AND name = ?", tenant.ID, "admin").First(&adminRole).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				adminRole = domain.Role{
					ID:       uuid.New(),
					TenantID: &tenant.ID,
					Name:     "admin",
				}
				if err := tx.Create(&adminRole).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// 3. Associate user with tenant
		tenantUser := domain.TenantUser{
			TenantID: tenant.ID,
			UserID:   user.ID,
			RoleID:   adminRole.ID,
			JoinedAt: time.Now(),
		}
		if err := tx.Create(&tenantUser).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data":   ToTenantJSON(tenant),
	})
}

// GetMembers returns the list of members in a given tenant.
func (h *TenantHandler) GetMembers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}

	type MemberRow struct {
		UserID    string `gorm:"column:user_id"`
		FullName  string `gorm:"column:full_name"`
		Email     string `gorm:"column:email"`
		AvatarURL string `gorm:"column:avatar_url"`
		RoleName  string `gorm:"column:role_name"`
	}

	var rows []MemberRow
	err = h.db.
		Table("tenant_users tu").
		Select("tu.user_id, u.full_name, u.email, u.avatar_url, r.name as role_name").
		Joins("JOIN users u ON u.id = tu.user_id").
		Joins("LEFT JOIN roles r ON r.id = tu.role_id").
		Where("tu.tenant_id = ?", id).
		Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type MemberJSON struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Avatar string `json:"avatar,omitempty"`
		Role   string `json:"role"`
	}

	var members []MemberJSON
	for _, r := range rows {
		members = append(members, MemberJSON{
			ID:     r.UserID,
			Name:   r.FullName,
			Email:  r.Email,
			Avatar: r.AvatarURL,
			Role:   r.RoleName,
		})
	}
	if members == nil {
		members = []MemberJSON{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   members,
	})
}
