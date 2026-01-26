package auth

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
)

type AuthUseCase interface {
	Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req LoginRequest) (*AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error)
}

type RegisterRequest struct {
    Email    string
    Password string
    Name     string
}

type LoginRequest struct {
    Email    string
    Password string
}

type AuthResponse struct {
    AccessToken  string
    RefreshToken string
    User         *domain.User
}

type authUseCase struct {
	userRepo postgres.UserRepository
	passwordSvc *PasswordService
	jwtSvc *JWTService
	cache cache.Cache
	keyBuilder *cache.CacheKeyBuilder
}

func NewAuthUseCase(
    repo postgres.UserRepository,
    ps *PasswordService,
    js *JWTService,
    c cache.Cache,
    kb *cache.CacheKeyBuilder,
) AuthUseCase {
    return &authUseCase{
        userRepo:    repo,
        passwordSvc: ps,
        jwtSvc:      js,
        cache:       c,
        keyBuilder:  kb,
    }
}

func (uc *authUseCase) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	_, err := mail.ParseAddress(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email format: %w", err)
	}

	exists, err := uc.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("email already registered")
	}

	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	hashedPass, err := uc.passwordSvc.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email: req.Email,
		Name: req.Name,
		PasswordHash: hashedPass,
		IsActive: true,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	accessToken, err := uc.jwtSvc.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	refreshToken, err := uc.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	refreshKey := uc.keyBuilder.RefreshToken(refreshToken)
	if err := uc.cache.Set(ctx, refreshKey, user.ID, 7 * time.Hour * 24); err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken: accessToken,
		RefreshToken: refreshToken,
		User: user,
	}, nil
}

func (uc *authUseCase) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	user, err := uc.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if err := uc.passwordSvc.ComparePassword(user.PasswordHash, req.Password); err != nil {
		return nil, err
	}

	accessToken, err := uc.jwtSvc.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	refreshToken, err := uc.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	refreshKey := uc.keyBuilder.RefreshToken(refreshToken)
	if err := uc.cache.Set(ctx, refreshKey, user.ID, 7 * time.Hour * 24); err != nil {
		return nil, err
	}

	now := time.Now()
	user.LastLoginAt = &now
	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken: accessToken,
		RefreshToken: refreshToken,
		User: user,
	}, nil
}

func (uc *authUseCase) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	refreshKey := uc.keyBuilder.RefreshToken(refreshToken)
	userID, err := uc.cache.Get(ctx, refreshKey)
	if err != nil {
		return nil, err
	}

	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	newAccessToken, err := uc.jwtSvc.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := uc.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	if err := uc.cache.Delete(ctx, refreshKey); err != nil {
		return nil, err
	}

	newRefreshKey := uc.keyBuilder.RefreshToken(newRefreshToken)
	if err := uc.cache.Set(ctx, newRefreshKey, user.ID, 7 * time.Hour * 24); err != nil {
		return nil, err
	}

	user.PasswordHash = ""

	return &AuthResponse{
		AccessToken: newAccessToken,
		RefreshToken: newRefreshToken,
		User: user,
	}, nil
}


