package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DashboardUseCase interface {
	GetStats(ctx context.Context, tenantID string) (map[string]interface{}, error)
	GetChartData(ctx context.Context, tenantID string) (map[string]interface{}, error)
	GetActivityFeed(ctx context.Context, tenantID string, limit int) ([]map[string]interface{}, error)
}

type dashboardUseCase struct {
	db *gorm.DB
}

func NewDashboardUseCase(db *gorm.DB) DashboardUseCase {
	return &dashboardUseCase{db: db}
}

func (uc *dashboardUseCase) GetStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}

	var docCount int64
	if err := uc.db.WithContext(ctx).Model(&domain.Document{}).Where("tenant_id = ?", tid).Count(&docCount).Error; err != nil {
		return nil, err
	}

	var wfCount int64
	if err := uc.db.WithContext(ctx).Model(&domain.Workflow{}).Where("tenant_id = ?", tid).Count(&wfCount).Error; err != nil {
		return nil, err
	}

	var tokenCount struct {
		Total int64
	}
	_ = uc.db.WithContext(ctx).Raw(
		"SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total FROM token_usage_ledgers WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '30 days'",
		tid,
	).Scan(&tokenCount)

	var avgLatency struct {
		Avg float64
	}
	_ = uc.db.WithContext(ctx).Raw(
		"SELECT COALESCE(AVG(execution_time_ms), 0) as avg FROM pipelines WHERE tenant_id = ? AND started_at >= NOW() - INTERVAL '30 days'",
		tid,
	).Scan(&avgLatency)

	return map[string]interface{}{
		"total_documents":  docCount,
		"active_workflows": wfCount,
		"tokens_used":      tokenCount.Total,
		"average_latency":  avgLatency.Avg,
	}, nil
}

func (uc *dashboardUseCase) GetChartData(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}

	type CostPoint struct {
		Date string  `json:"date"`
		Cost float64 `json:"cost"`
	}
	var usagePoints []CostPoint
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as date, SUM(cost) as cost 
		FROM token_usage_ledgers 
		WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY TO_CHAR(created_at, 'YYYY-MM-DD')
		ORDER BY date ASC
	`, tid).Scan(&usagePoints)

	type LatencyPoint struct {
		Date    string  `json:"date"`
		Latency float64 `json:"latency"`
	}
	var latencyPoints []LatencyPoint
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(started_at, 'YYYY-MM-DD') as date, AVG(execution_time_ms) as latency 
		FROM pipelines 
		WHERE tenant_id = ? AND started_at >= NOW() - INTERVAL '7 days'
		GROUP BY TO_CHAR(started_at, 'YYYY-MM-DD')
		ORDER BY date ASC
	`, tid).Scan(&latencyPoints)

	return map[string]interface{}{
		"usage_costs": usagePoints,
		"latency":     latencyPoints,
	}, nil
}

func (uc *dashboardUseCase) GetActivityFeed(ctx context.Context, tenantID string, limit int) ([]map[string]interface{}, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}

	type AuditLog struct {
		ID           string    `json:"id"`
		Action       string    `json:"action"`
		ResourceType string    `json:"resource_type"`
		CreatedAt    time.Time `json:"created_at"`
	}
	var logs []AuditLog

	_ = uc.db.WithContext(ctx).Raw(`
		SELECT id, action, resource_type, created_at
		FROM enterprise_audit_logs 
		WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '30 days'
		ORDER BY created_at DESC 
		LIMIT ?
	`, tid, limit).Scan(&logs)

	feed := []map[string]interface{}{}
	for _, log := range logs {
		feed = append(feed, map[string]interface{}{
			"id":          log.ID,
			"type":        log.ResourceType,
			"action":      log.Action,
			"description": fmt.Sprintf("%s on %s", log.Action, log.ResourceType),
			"timestamp":   log.CreatedAt.Format(time.RFC3339),
		})
	}

	return feed, nil
}
