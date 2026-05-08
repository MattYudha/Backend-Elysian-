package domain

import (
	"time"

	"gorm.io/datatypes"
)

type SwarmTask struct {
	ID         string         `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	DocumentID string         `json:"document_id" gorm:"type:uuid;not null"`
	Status     string         `json:"status" gorm:"type:varchar(50);not null;default:'PENDING'"`
	Summary    string         `json:"summary" gorm:"type:text"`
	Results    datatypes.JSON `json:"results" gorm:"type:jsonb"`
	CreatedAt  time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

type SwarmPayload struct {
	TaskID       string                   `json:"task_id"`
	DocumentID   string                   `json:"document_id"`
	DocumentType string                   `json:"document_type"`
	Items        []map[string]interface{} `json:"items"`
	WebhookURL   string                   `json:"webhook_url"`
}

type SwarmCallback struct {
	TaskID  string                   `json:"task_id"`
	Status  string                   `json:"status"`
	Summary string                   `json:"summary"`
	Results []map[string]interface{} `json:"results"`
}
