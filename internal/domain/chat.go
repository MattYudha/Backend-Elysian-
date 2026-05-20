package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ChatSession struct {
	ID        uuid.UUID     `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID  uuid.UUID     `gorm:"type:uuid;not null;index" json:"tenant_id"`
	UserID    uuid.UUID     `gorm:"type:uuid;not null" json:"user_id"`
	Title     string        `gorm:"type:varchar(255)" json:"title"`
	CreatedAt time.Time     `json:"created_at"`
	Messages  []ChatMessage `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
}

type ChatMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	SessionID      uuid.UUID `gorm:"type:uuid;not null;index" json:"session_id"`
	SenderRole     string    `gorm:"type:varchar(50);not null" json:"sender_role"`
	MessageContent string    `gorm:"type:text;not null" json:"message_content"`
	TokensUsed     int       `gorm:"default:0" json:"tokens_used"`
	CreatedAt      time.Time `gorm:"primaryKey" json:"created_at"`
}

type ChatRepository interface {
	CreateSession(ctx context.Context, session *ChatSession) error
	GetSession(ctx context.Context, tenantID, id string) (*ChatSession, error)
	ListSessions(ctx context.Context, tenantID, userID string) ([]*ChatSession, error)
	DeleteSession(ctx context.Context, tenantID, id string) error

	CreateMessage(ctx context.Context, msg *ChatMessage) error
	ListMessages(ctx context.Context, sessionID string) ([]*ChatMessage, error)
}
