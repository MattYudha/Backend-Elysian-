package handler

import (
	"net/http"
	"strings"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authUseCase  auth.AuthUseCase
	validate     *validator.Validate
	isProduction bool
	db           *gorm.DB
}

func NewAuthHandler(authUseCase auth.AuthUseCase, isProduction bool, db ...*gorm.DB) *AuthHandler {
	var database *gorm.DB
	if len(db) > 0 {
		database = db[0]
	}
	return &AuthHandler{
		authUseCase:  authUseCase,
		validate:     validator.New(),
		isProduction: isProduction,
		db:           database,
	}
}

// getUserRole fetches the highest role name for a user across all their tenants
func (h *AuthHandler) getUserRole(userID string) string {
	if h.db == nil {
		return "viewer"
	}
	type RoleResult struct {
		Name string
	}
	var result RoleResult
	// Priority: admin > manager > viewer
	err := h.db.Raw(`
		SELECT r.name FROM roles r
		JOIN tenant_users tu ON tu.role_id = r.id
		WHERE tu.user_id = ?
		ORDER BY CASE r.name
			WHEN 'admin' THEN 1
			WHEN 'super_admin' THEN 1
			WHEN 'manager' THEN 2
			ELSE 3
		END
		LIMIT 1
	`, userID).Scan(&result).Error
	if err != nil || result.Name == "" {
		return "viewer"
	}
	return result.Name
}

// Request and Response structs
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResponse struct {
	Message      string       `json:"message"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	User         *domain.User `json:"user,omitempty"`
}

// Register godoc
// @Summary      Register a new user
// @Description  Register a new user with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body auth.RegisterRequest true "Register Request"
// @Success      201  {object}  AuthResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse
// @Router       /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req auth.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body: " + err.Error()})
		return
	}

	res, err := h.authUseCase.Register(c.Request.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "Email already registered"})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	h.setRefreshTokenCookie(c, res.RefreshToken)

	role := h.getUserRole(res.User.ID.String())

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"access_token": res.AccessToken,
			"user": gin.H{
				"id":         res.User.ID,
				"email":      res.User.Email,
				"full_name":  res.User.FullName,
				"avatar_url": res.User.AvatarURL,
				"role":       role,
			},
		},
	})
}

// Login godoc
// @Summary      Login
// @Description  Login with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body auth.LoginRequest true "Login Request"
// @Success      200  {object}  AuthResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	res, err := h.authUseCase.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Invalid email or password"})
		return
	}

	h.setRefreshTokenCookie(c, res.RefreshToken)

	role := h.getUserRole(res.User.ID.String())

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"access_token": res.AccessToken,
			"user": gin.H{
				"id":         res.User.ID,
				"email":      res.User.Email,
				"full_name":  res.User.FullName,
				"avatar_url": res.User.AvatarURL,
				"role":       role,
			},
		},
	})
}

// RefreshToken godoc
// @Summary      Refresh Access Token
// @Description  Refresh access token using refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RefreshTokenRequest false "Refresh Token Request"
// @Success      200  {object}  AuthResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var refreshToken string

	cookieToken, err := c.Cookie("refresh_token")
	if err == nil && cookieToken != "" {
		refreshToken = cookieToken
	} else {
		var req RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Refresh token is required"})
		return
	}

	res, err := h.authUseCase.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		// Clear the stale cookie so the browser stops retrying with it
		c.SetCookie("refresh_token", "", -1, "/", "", h.isProduction, true)
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Invalid or expired refresh token"})
		return
	}

	if cookieToken != "" {
		h.setRefreshTokenCookie(c, res.RefreshToken)
	}

	role := h.getUserRole(res.User.ID.String())

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"access_token": res.AccessToken,
			"user": gin.H{
				"id":         res.User.ID,
				"email":      res.User.Email,
				"full_name":  res.User.FullName,
				"avatar_url": res.User.AvatarURL,
				"role":       role,
			},
		},
	})
}

// Logout godoc
// @Summary      Logout
// @Description  Logout user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LogoutRequest false "Logout Request"
// @Success      200  {object}  SuccessResponse
// @Router       /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie("refresh_token")
	if refreshToken == "" {
		var req LogoutRequest
		c.ShouldBindJSON(&req)
		refreshToken = req.RefreshToken
	}

	if refreshToken != "" {
		h.authUseCase.Logout(c.Request.Context(), refreshToken)
	}

	c.SetCookie("refresh_token", "", -1, "/", "", h.isProduction, true)

	c.JSON(http.StatusOK, SuccessResponse{Message: "Logged out successfully"})
}

func (h *AuthHandler) setRefreshTokenCookie(c *gin.Context, token string) {
	c.SetCookie(
		"refresh_token",
		token,
		7*24*60*60,
		"/",
		"",
		h.isProduction,
		true,
	)
}
