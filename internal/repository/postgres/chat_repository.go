package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/gorm"
)

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) domain.ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) CreateSession(ctx context.Context, session *domain.ChatSession) error {
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("failed to create chat session: %w", err)
	}
	return nil
}

func (r *chatRepository) GetSession(ctx context.Context, tenantID, id string) (*domain.ChatSession, error) {
	var session domain.ChatSession
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&session).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("chat session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chat session: %w", err)
	}

	return &session, nil
}

func (r *chatRepository) ListSessions(ctx context.Context, tenantID, userID string) ([]*domain.ChatSession, error) {
	var sessions []*domain.ChatSession
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Find(&sessions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list chat sessions: %w", err)
	}

	return sessions, nil
}

func (r *chatRepository) DeleteSession(ctx context.Context, tenantID, id string) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&domain.ChatSession{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete chat session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat session not found")
	}
	return nil
}

func (r *chatRepository) CreateMessage(ctx context.Context, msg *domain.ChatMessage) error {
	if err := r.db.WithContext(ctx).Create(msg).Error; err != nil {
		return fmt.Errorf("failed to create chat message: %w", err)
	}
	return nil
}

func (r *chatRepository) ListMessages(ctx context.Context, sessionID string) ([]*domain.ChatMessage, error) {
	var messages []*domain.ChatMessage
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list chat messages: %w", err)
	}

	return messages, nil
}
