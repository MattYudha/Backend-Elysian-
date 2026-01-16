-- +goose Up
-- +goose StatementBegin
INSERT INTO roles (name, description, permissions) VALUES
('admin', 'Full system access', '["*"]'::jsonb),
('user', 'Standard user access', '["workflow:read", "workflow:write", "workflow:execute", "workflow:delete"]'::jsonb),
('viewer', 'Read-only access', '["workflow:read"]'::jsonb)
ON CONFLICT (name) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM roles WHERE name IN ('admin', 'user', 'viewer');
-- +goose StatementEnd
