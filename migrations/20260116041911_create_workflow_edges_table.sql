-- +goose Up
-- +goose StatementBegin
CREATE TABLE workflow_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL,
    edge_key VARCHAR(100) NOT NULL,
    source_node_id UUID NOT NULL,
    target_node_id UUID NOT NULL,
    source_handle VARCHAR(100),
    target_handle VARCHAR(100),
    configuration JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_workflow_edges_workflow FOREIGN KEY (workflow_id)
        REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT fk_workflow_edges_source FOREIGN KEY (source_node_id)
        REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    CONSTRAINT fk_workflow_edges_target FOREIGN KEY (target_node_id)
        REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    CONSTRAINT uq_workflow_edges_key UNIQUE(workflow_id, edge_key)
);

-- Indexes
CREATE INDEX idx_workflow_edges_workflow_id ON workflow_edges(workflow_id);
CREATE INDEX idx_workflow_edges_source ON workflow_edges(source_node_id);
CREATE INDEX idx_workflow_edges_target ON workflow_edges(target_node_id);

-- Trigger
CREATE TRIGGER update_workflow_edges_updated_at
    BEFORE UPDATE ON workflow_edges
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_workflow_edges_updated_at ON workflow_edges;
DROP TABLE IF EXISTS workflow_edges;
-- +goose StatementEnd
