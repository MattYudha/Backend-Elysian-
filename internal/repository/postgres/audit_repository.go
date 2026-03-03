package postgres

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/gorm"
)

type auditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) domain.AuditRepository {
	return &auditRepository{db: db}
}

// Create stores a new audit log record. By using gorm on the partitioned table 'enterprise_audit_logs',
// postgres handles the routing to the appropriate monthly partition table automatically.
func (r *auditRepository) Create(ctx context.Context, audit *domain.AuditLog) error {
	return r.db.WithContext(ctx).Table("enterprise_audit_logs").Create(audit).Error
}
