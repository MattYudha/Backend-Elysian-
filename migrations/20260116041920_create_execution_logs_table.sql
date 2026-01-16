-- +goose Up
-- +goose StatementBegin
CREATE TABLE execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL,
    node_id UUID,
    log_level VARCHAR(20) DEFAULT 'info' NOT NULL,
    message TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_execution_logs_execution FOREIGN KEY (execution_id)
        REFERENCES executions(id) ON DELETE CASCADE,
    CONSTRAINT fk_execution_logs_node FOREIGN KEY (node_id)
        REFERENCES workflow_nodes(id) ON DELETE SET NULL,
    CONSTRAINT chk_execution_logs_level CHECK (log_level IN ('debug', 'info', 'warn', 'error'))
);

-- Indexes
CREATE INDEX idx_execution_logs_execution_id ON execution_logs(execution_id);
CREATE INDEX idx_execution_logs_created_at ON execution_logs(created_at);
CREATE INDEX idx_execution_logs_level ON execution_logs(log_level);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS execution_logs;
-- +goose StatementEnd
