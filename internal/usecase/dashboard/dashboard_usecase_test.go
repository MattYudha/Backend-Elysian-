package dashboard_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/dashboard"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func SetupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock database: %v", err)
	}

	dialector := postgres.New(postgres.Config{
		Conn: db,
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open gorm DB: %v", err)
	}

	return gormDB, mock
}

func TestDashboardUseCase_GetStats(t *testing.T) {
	gormDB, mock := SetupMockDB(t)
	defer func() {
		mock.ExpectClose()
	}()

	uc := dashboard.NewDashboardUseCase(gormDB)
	tenantID := uuid.New()

	// 1. SELECT count(*) FROM "documents" WHERE tenant_id = $1
	mock.ExpectQuery(`SELECT count\(\*\) FROM "documents" WHERE tenant_id = \$1`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(120))

	// 2. SELECT count(*) FROM "workflows" WHERE tenant_id = $1
	mock.ExpectQuery(`SELECT count\(\*\) FROM "workflows" WHERE tenant_id = \$1`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	// 3. Token usage ledger sum
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(prompt_tokens \+ completion_tokens\), 0\) as total FROM token_usage_ledgers WHERE tenant_id = \$1`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(int64(50000)))

	// 4. Pipeline execution metrics
	mock.ExpectQuery(`(?i)SELECT.*FROM pipelines WHERE tenant_id = \$1`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"total", "failed", "avg_ms", "p95_ms"}).
			AddRow(int64(250), int64(3), float64(120.0), float64(150.0)))

	// 5. Select tenant plan_tier
	mock.ExpectQuery(`SELECT plan_tier FROM "tenants" WHERE id = \$1`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"plan_tier"}).AddRow("enterprise"))

	res, err := uc.GetStats(context.Background(), tenantID.String())
	if err != nil {
		t.Fatalf("Expected GetStats to succeed, got: %v", err)
	}

	if res["total_documents"] != int64(120) {
		t.Errorf("Expected total_documents = 120, got %v", res["total_documents"])
	}
	if res["health_score"] != 97 {
		t.Errorf("Expected health_score = 97, got %v", res["health_score"])
	}
	if res["budget_limit"] != 5000.0 { // enterprise limit
		t.Errorf("Expected budget_limit = 5000.0, got %v", res["budget_limit"])
	}
}

func TestDashboardUseCase_GetChartData(t *testing.T) {
	gormDB, mock := SetupMockDB(t)
	defer func() {
		mock.ExpectClose()
	}()

	uc := dashboard.NewDashboardUseCase(gormDB)
	tenantID := uuid.New()

	// Mock usage_costs query
	mock.ExpectQuery(`SELECT TO_CHAR\(created_at, 'YYYY-MM-DD'\) as date`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"date", "cost", "tokens"}).
			AddRow("2026-05-18", 12.5, int64(125000)).
			AddRow("2026-05-19", 15.0, int64(150000)))

	// Mock latency percentile query
	mock.ExpectQuery(`SELECT TO_CHAR\(started_at, 'YYYY-MM-DD'\) as date`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"date", "p95", "p99", "errors"}).
			AddRow("2026-05-18", 310.0, 450.0, int64(2)).
			AddRow("2026-05-19", 320.0, 460.0, int64(1)))

	res, err := uc.GetChartData(context.Background(), tenantID.String())
	if err != nil {
		t.Fatalf("Expected GetChartData to succeed, got: %v", err)
	}

	// inspect using JSON encoding/decoding
	bytes, _ := json.Marshal(res)
	var mapRes map[string]interface{}
	_ = json.Unmarshal(bytes, &mapRes)

	costs, ok := mapRes["usage_costs"].([]interface{})
	if !ok || len(costs) != 2 {
		t.Fatalf("Expected 2 cost entries, got %v", mapRes["usage_costs"])
	}
	cost1 := costs[0].(map[string]interface{})
	if cost1["cost"] != 12.5 {
		t.Errorf("Expected cost to be 12.5, got %v", cost1["cost"])
	}

	latency, ok := mapRes["latency"].([]interface{})
	if !ok || len(latency) != 2 {
		t.Fatalf("Expected 2 latency entries, got %v", mapRes["latency"])
	}
	lat1 := latency[0].(map[string]interface{})
	if lat1["p95"] != 310.0 {
		t.Errorf("Expected p95 to be 310.0, got %v", lat1["p95"])
	}
}

func TestDashboardUseCase_GetAuditLogs(t *testing.T) {
	gormDB, mock := SetupMockDB(t)
	defer func() {
		mock.ExpectClose()
	}()

	uc := dashboard.NewDashboardUseCase(gormDB)
	tenantID := uuid.New()

	now := time.Now()
	// Match raw audit log retrieval query
	mock.ExpectQuery(`SELECT l\.id, COALESCE\(u\.full_name, 'System'\) as actor_name`).
		WithArgs(tenantID, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_name", "actor_email", "actor_avatar", "action", "resource_type", "context_ip", "created_at"}).
			AddRow("log-1", "John Doe", "john@example.com", "avatar.png", "CREATE", "workflow", "127.0.0.1", now))

	res, err := uc.GetAuditLogs(context.Background(), tenantID.String(), 5)
	if err != nil {
		t.Fatalf("Expected GetAuditLogs to succeed, got: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(res))
	}
	if res[0].ActorName != "John Doe" {
		t.Errorf("Expected actor name 'John Doe', got '%s'", res[0].ActorName)
	}
}

func TestDashboardUseCase_GetPriorityQueue(t *testing.T) {
	gormDB, mock := SetupMockDB(t)
	defer func() {
		mock.ExpectClose()
	}()

	uc := dashboard.NewDashboardUseCase(gormDB)
	tenantID := uuid.New()

	// 1. Pipeline failures check
	mock.ExpectQuery(`SELECT id, name, started_at FROM pipelines WHERE tenant_id = \$1 AND status = 'failed'`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "started_at"}).
			AddRow(uuid.New(), "Execution-1", time.Now()))

	// 2. Token Limit Check
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(prompt_tokens \+ completion_tokens\), 0\) as total FROM token_usage_ledgers`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(int64(9500000))) // near 10M pro limit

	// 3. Plan Tier Check
	mock.ExpectQuery(`SELECT plan_tier FROM "tenants" WHERE id = \$1`).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"plan_tier"}).
			AddRow("pro"))

	res, err := uc.GetPriorityQueue(context.Background(), tenantID.String())
	if err != nil {
		t.Fatalf("Expected GetPriorityQueue to succeed, got: %v", err)
	}

	// Should contain pipeline failure warning + billing budget warning + scheduled maintenance warning
	if len(res) < 3 {
		t.Fatalf("Expected at least 3 items, got %d", len(res))
	}
	if res[0].Type != "pipeline" {
		t.Errorf("Expected first priority item type to be pipeline, got %s", res[0].Type)
	}
	if res[1].Type != "billing" {
		t.Errorf("Expected second priority item type to be billing, got %s", res[1].Type)
	}
}
