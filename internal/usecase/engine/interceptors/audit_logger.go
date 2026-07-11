package interceptors

import (
	"context"
	"encoding/json"
	"log"
	"net"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
	"github.com/google/uuid"
)

// NewForensicAuditInterceptor intercepts node execution to log telemetry to the enterprise_audit_logs DB.
func NewForensicAuditInterceptor(auditRepo domain.AuditRepository) engine.Interceptor {
	return func(node engine.Node, ctx *engine.ExecutionContext, next func() error) error {
		// Extract mandatory identity from context with defensive checks
		tenantID := uuid.Nil
		if val, ok := ctx.Get("tenant_id"); ok {
			if s, ok := val.(string); ok {
				if id, err := uuid.Parse(s); err == nil {
					tenantID = id
				}
			}
		}

		userID := uuid.Nil
		if val, ok := ctx.Get("user_id"); ok {
			if s, ok := val.(string); ok {
				if id, err := uuid.Parse(s); err == nil {
					userID = id
				}
			}
		}

		// Attempt to extract IP if available
		ip := "0.0.0.0"
		if val, ok := ctx.Get("client_ip"); ok {
			if s, ok := val.(string); ok && net.ParseIP(s) != nil {
				ip = s
			}
		}

		// Execute next in chain (the actual handler or another interceptor)
		err := next()

		// Build Evidence JSON including output status
		evidenceMap := map[string]interface{}{
			"node_id":   node.ID,
			"node_type": node.Type,
			"status":    "success",
			"data":      node.Data, // Log the node configurations used
		}
		if err != nil {
			evidenceMap["status"] = "failed"
			evidenceMap["error"] = err.Error()
		}

		evidenceBytes, _ := json.Marshal(evidenceMap)

		// Parse the target resourceID (which is the node ID string disguised/hashed, or we keep it nil and use action string)
		// We'll leave ResourceID zero-value and dump info into evidence since node.ID is just a string alias.
		var resID uuid.UUID

		audit := &domain.AuditLog{
			TenantID:     tenantID,
			ActorID:      userID,
			Action:       "NODE_EXECUTION",
			ResourceType: "workflow_node",
			ResourceID:   resID,
			ContextIP:    ip,
			Evidence:     json.RawMessage(evidenceBytes),
		}

		// Asynchronously write to PostgreSQL to keep engine lightning fast
		go func() {
			if auditErr := auditRepo.Create(context.Background(), audit); auditErr != nil {
				// We do not fail the node if audit fails, but we raise a massive critical alert.
				log.Printf("[CRITICAL] Forensic Audit Interceptor failed to write to DB: %v", auditErr)
			}
		}()

		return err // Return the actual node error
	}
}
