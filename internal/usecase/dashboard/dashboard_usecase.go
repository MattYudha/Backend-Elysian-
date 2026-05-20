package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLogResponse struct {
	ID           string    `json:"id"`
	ActorName    string    `json:"actor_name"`
	ActorEmail   string    `json:"actor_email"`
	ActorAvatar  string    `json:"actor_avatar"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ContextIP    string    `json:"context_ip"`
	CreatedAt    time.Time `json:"created_at"`
}

type PriorityQueueItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
}

type DashboardUseCase interface {
	GetStats(ctx context.Context, tenantID string) (map[string]interface{}, error)
	GetChartData(ctx context.Context, tenantID string) (map[string]interface{}, error)
	GetActivityFeed(ctx context.Context, tenantID string, limit int) ([]map[string]interface{}, error)
	GetAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLogResponse, error)
	GetPriorityQueue(ctx context.Context, tenantID string) ([]PriorityQueueItem, error)
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

	// ── Document count ─────────────────────────────────────────────────────────
	var docCount int64
	if err := uc.db.WithContext(ctx).Model(&domain.Document{}).Where("tenant_id = ?", tid).Count(&docCount).Error; err != nil {
		return nil, err
	}

	// ── Workflow count ─────────────────────────────────────────────────────────
	var wfCount int64
	if err := uc.db.WithContext(ctx).Model(&domain.Workflow{}).Where("tenant_id = ?", tid).Count(&wfCount).Error; err != nil {
		return nil, err
	}

	// ── Token usage (last 30 days) ─────────────────────────────────────────────
	var tokenCount struct {
		Total int64
	}
	_ = uc.db.WithContext(ctx).Raw(
		"SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total FROM token_usage_ledgers WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '30 days'",
		tid,
	).Scan(&tokenCount)

	// ── Pipeline execution metrics (last 30 days) ──────────────────────────────
	// Used for dynamic health score computation
	var pipe struct {
		Total  int64   `gorm:"column:total"`
		Failed int64   `gorm:"column:failed"`
		AvgMs  float64 `gorm:"column:avg_ms"`
		P95Ms  float64 `gorm:"column:p95_ms"`
	}
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*)                                                               AS total,
			COUNT(CASE WHEN status = 'failed' THEN 1 END)                         AS failed,
			COALESCE(AVG(execution_time_ms), 0)                                   AS avg_ms,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY execution_time_ms), 0) AS p95_ms
		FROM pipelines
		WHERE tenant_id = ? AND started_at >= NOW() - INTERVAL '30 days'
	`, tid).Scan(&pipe)

	// ── Tenant plan ────────────────────────────────────────────────────────────
	var tenantInfo struct {
		PlanTier string `gorm:"column:plan_tier"`
	}
	_ = uc.db.WithContext(ctx).Table("tenants").
		Select("plan_tier").
		Where("id = ?", tid).
		Scan(&tenantInfo)

	budgetLimit := 50.0
	switch tenantInfo.PlanTier {
	case "enterprise":
		budgetLimit = 5000.0
	case "pro":
		budgetLimit = 500.0
	}

	// ── Dynamic Health Score Formula (0–100) ───────────────────────────────────
	//
	//  Requires at least 1 pipeline execution in last 30 days.
	//  If no executions → score = nil → frontend shows empty state.
	//
	//  Component 1 – Success Rate penalty  (0–50 pts deducted)
	//    penalty = (1 - successRate) * 50
	//
	//  Component 2 – Error Rate penalty    (0–30 pts deducted, non-linear cap)
	//    penalty = min(errorRate * 200, 30)
	//
	//  Component 3 – Latency (p95) penalty (0–20 pts deducted)
	//    < 200ms → 0   | 200–500ms → 10 | 500–1000ms → 15 | > 1000ms → 20

	hasPipelineData := pipe.Total > 0

	latencyStatus := "operational"
	apiGwStatus := "operational"
	var healthScorePtr *int

	if hasPipelineData {
		score := 100.0

		// Component 1 — success rate
		successRate := float64(pipe.Total-pipe.Failed) / float64(pipe.Total)
		score -= (1.0 - successRate) * 50.0

		// Component 2 — error rate (non-linear, max 30 pts)
		errorRate := float64(pipe.Failed) / float64(pipe.Total)
		errPenalty := errorRate * 200.0
		if errPenalty > 30.0 {
			errPenalty = 30.0
		}
		score -= errPenalty

		// Component 3 — latency
		switch {
		case pipe.P95Ms > 1000:
			score -= 20.0
			latencyStatus = "degraded"
		case pipe.P95Ms > 500:
			score -= 15.0
			latencyStatus = "degraded"
		case pipe.P95Ms > 200:
			score -= 10.0
		}

		// API gateway degraded if error rate > 20%
		if errorRate > 0.20 {
			apiGwStatus = "degraded"
		}

		// Clamp and round
		if score < 0 {
			score = 0
		}
		rounded := int(score + 0.5)
		healthScorePtr = &rounded
	}

	result := map[string]interface{}{
		"total_documents":   docCount,
		"active_workflows":  wfCount,
		"tokens_used":       tokenCount.Total,
		"average_latency":   pipe.AvgMs,
		"budget_limit":      budgetLimit,
		"has_pipeline_data": hasPipelineData,
		"pipeline_total":    pipe.Total,
		"pipeline_failed":   pipe.Failed,
		"latency_p95_ms":    pipe.P95Ms,
		"systems": map[string]string{
			"api_gateway": apiGwStatus,
			"vector_db":   "operational",
			"latency":     latencyStatus,
		},
	}

	if healthScorePtr != nil {
		result["health_score"] = *healthScorePtr
	} else {
		result["health_score"] = nil
	}

	return result, nil
}


func (uc *dashboardUseCase) GetChartData(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}

	type CostPoint struct {
		Date   string  `json:"date"`
		Cost   float64 `json:"cost"`
		Tokens int64   `json:"tokens"`
	}
	usagePoints := []CostPoint{}
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as date, 
		       COALESCE(SUM(cost), 0) as cost,
		       COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens
		FROM token_usage_ledgers 
		WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY TO_CHAR(created_at, 'YYYY-MM-DD')
		ORDER BY date ASC
	`, tid).Scan(&usagePoints)

	type LatencyPoint struct {
		Date    string  `json:"date"`
		P95     float64 `json:"p95"`
		P99     float64 `json:"p99"`
		Errors  int64   `json:"errors"`
	}
	latencyPoints := []LatencyPoint{}
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(started_at, 'YYYY-MM-DD') as date, 
		       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY execution_time_ms), 0) as p95,
		       COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY execution_time_ms), 0) as p99,
		       COUNT(CASE WHEN status = 'failed' THEN 1 END) as errors
		FROM pipelines 
		WHERE tenant_id = ? AND started_at >= NOW() - INTERVAL '7 days'
		GROUP BY TO_CHAR(started_at, 'YYYY-MM-DD')
		ORDER BY date ASC
	`, tid).Scan(&latencyPoints)

	if usagePoints == nil {
		usagePoints = []CostPoint{}
	}
	if latencyPoints == nil {
		latencyPoints = []LatencyPoint{}
	}

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

func (uc *dashboardUseCase) GetAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLogResponse, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}

	var results []AuditLogResponse
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT l.id, COALESCE(u.full_name, 'System') as actor_name, COALESCE(u.email, 'system@elysian.com') as actor_email, 
		       COALESCE(u.avatar_url, '') as actor_avatar, l.action, l.resource_type, l.context_ip, l.created_at
		FROM enterprise_audit_logs l
		LEFT JOIN users u ON l.actor_id = u.id
		WHERE l.tenant_id = ? AND l.created_at >= NOW() - INTERVAL '30 days'
		ORDER BY l.created_at DESC
		LIMIT ?
	`, tid, limit).Scan(&results)

	if results == nil {
		results = []AuditLogResponse{}
	}

	return results, nil
}

func (uc *dashboardUseCase) GetPriorityQueue(ctx context.Context, tenantID string) ([]PriorityQueueItem, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}

	var items []PriorityQueueItem

	// 1. Failed Pipelines (High Priority)
	var failedPipelines []struct {
		ID        uuid.UUID
		Name      string
		StartedAt time.Time
	}
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT id, name, started_at 
		FROM pipelines 
		WHERE tenant_id = ? AND status = 'failed' AND started_at >= NOW() - INTERVAL '7 days'
		ORDER BY started_at DESC 
		LIMIT 5
	`, tid).Scan(&failedPipelines)

	for _, fp := range failedPipelines {
		items = append(items, PriorityQueueItem{
			ID:          fp.ID.String(),
			Title:       fmt.Sprintf("Pipeline #%s Failed", fp.Name),
			Description: "Execution failed during runtime",
			Priority:    "high",
			Timestamp:   fp.StartedAt,
			Type:        "pipeline",
		})
	}

	// 2. Token Limit (Medium Priority)
	var tokenCount struct {
		Total int64
	}
	_ = uc.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) as total 
		FROM token_usage_ledgers 
		WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '30 days'
	`, tid).Scan(&tokenCount)

	var tenantInfo struct {
		PlanTier string `gorm:"column:plan_tier"`
	}
	_ = uc.db.WithContext(ctx).Table("tenants").
		Select("plan_tier").
		Where("id = ?", tid).
		Scan(&tenantInfo)

	var tokenLimit int64 = 1000000 // default Free
	if tenantInfo.PlanTier == "pro" {
		tokenLimit = 10000000
	} else if tenantInfo.PlanTier == "enterprise" {
		tokenLimit = 100000000
	}

	if tokenCount.Total >= tokenLimit {
		items = append(items, PriorityQueueItem{
			ID:          uuid.New().String(),
			Title:       "Token Limit Reached",
			Description: "Upgraded to Tier 2 automatically",
			Priority:    "medium",
			Timestamp:   time.Now(),
			Type:        "billing",
		})
	} else if float64(tokenCount.Total) >= float64(tokenLimit)*0.9 {
		items = append(items, PriorityQueueItem{
			ID:          uuid.New().String(),
			Title:       "Token Limit Nearing",
			Description: fmt.Sprintf("Using %d%% of monthly limit", int(float64(tokenCount.Total)/float64(tokenLimit)*100)),
			Priority:    "medium",
			Timestamp:   time.Now(),
			Type:        "billing",
		})
	}

	// 3. System maintenance update (Low Priority)
	items = append(items, PriorityQueueItem{
		ID:          uuid.New().String(),
		Title:       "System Update",
		Description: "Scheduled maintenance in 24h",
		Priority:    "low",
		Timestamp:   time.Now().Add(-4 * time.Hour),
		Type:        "system",
	})

	return items, nil
}
