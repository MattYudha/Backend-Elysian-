-- +goose Up
-- +goose StatementBegin
CREATE TABLE workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL,
    node_key VARCHAR(100) NOT NULL,
    node_type VARCHAR(100) NOT NULL,
    label VARCHAR(255),
    position_x REAL,
    position_y REAL,
    configuration JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_workflow_nodes_workflow FOREIGN KEY (workflow_id)
        REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT uq_workflow_nodes_key UNIQUE(workflow_id, node_key)
);

-- Indexes
CREATE INDEX idx_workflow_nodes_workflow_id ON workflow_nodes(workflow_id);
CREATE INDEX idx_workflow_nodes_type ON workflow_nodes(node_type);

-- Trigger
CREATE TRIGGER update_workflow_nodes_updated_at
    BEFORE UPDATE ON workflow_nodes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_workflow_nodes_updated_at ON workflow_nodes;
DROP TABLE IF EXISTS workflow_nodes;
-- +goose StatementEnd
