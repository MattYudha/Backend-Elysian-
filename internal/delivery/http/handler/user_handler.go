package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/Elysian-Rebirth/backend-go/internal/middleware"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo repository.UserRepository
}

func NewUserHandler(userRepo repository.UserRepository) *UserHandler {
	return &UserHandler{
		userRepo: userRepo,
	}
}

// Request and Response structs
type UpdateUserRequest struct {
	Name      string  `json:"name" validate:"min=2,max=100"`
	AvatarURL *string `json:"avatar_url"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type UserListResponse struct {
	Data []*domain.User `json:"data"`
	Meta Meta           `json:"meta"`
}

type Meta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type UpdateUserResponse struct {
	Message string       `json:"message"`
	User    UserResponse `json:"user"`
}

// GetByID godoc
// @Summary      Get user by ID
// @Description  Get user details by ID
// @Tags         users
// @Produce      json
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  domain.User
// @Failure      404  {object}  ErrorResponse
// @Router       /api/v1/users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	user, err := h.userRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// List godoc
// @Summary      List users
// @Description  Get list of users
// @Tags         users
// @Produce      json
// @Param        limit   query     int     false  "Limit"
// @Param        offset  query     int     false  "Offset"
// @Success      200     {object}  UserListResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /api/v1/users [get]
func (h *UserHandler) List(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	users, total, err := h.userRepo.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, UserListResponse{
		Data: users,
		Meta: Meta{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
}

// GetByEmail godoc
// @Summary      Get user by email
// @Description  Get user details by email
// @Tags         users
// @Produce      json
// @Param        email path      string  true  "User Email"
// @Success      200   {object}  domain.User
// @Failure      404   {object}  ErrorResponse
// @Router       /api/v1/users/email/{email} [get]
func (h *UserHandler) GetByEmail(c *gin.Context) {
	email := c.Param("email")

	user, err := h.userRepo.FindByEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetMe godoc
// @Summary      Get current user
// @Description  Get details of currently logged in user
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  UserResponse
// @Router       /api/v1/users/me [get]
func (h *UserHandler) GetMe(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)

	c.JSON(http.StatusOK, UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
	})
}

// UpdateMe godoc
// @Summary      Update current user
// @Description  Update details of currently logged in user
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body UpdateUserRequest true "Update Request"
// @Success      200  {object}  UpdateUserResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/users/me [put]
func (h *UserHandler) UpdateMe(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)

	var req UpdateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, UpdateUserResponse{
		Message: "Profile updated successfully",
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			AvatarURL: user.AvatarURL,
		},
	})
}

// DeleteMe godoc
// @Summary      Delete current user
// @Description  Delete currently logged in user account
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  SuccessResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/users/me [delete]
func (h *UserHandler) DeleteMe(c *gin.Context) {
	user := middleware.MustGetUserFromContext(c)

	if err := h.userRepo.Delete(c.Request.Context(), user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete account"})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Account deleted successfully",
	})
}
