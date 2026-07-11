-- +goose Up
-- +goose StatementBegin

-- 1. Hapus index redundan pada Primary Key user_profiles
DROP INDEX IF EXISTS idx_user_profiles_user_id;

-- 2. Index untuk Foreign Keys & Kolom Pencarian Kritis guna mencegah deadlock/full table scan pada CASCADE DELETE
CREATE INDEX IF NOT EXISTS idx_sso_identities_user_id ON sso_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_roles_tenant_id ON roles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_user_id ON tenant_users(user_id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_role_id ON tenant_users(role_id);
CREATE INDEX IF NOT EXISTS idx_documents_tenant_id ON documents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_documents_user_id ON documents(user_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_id ON workflows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_tenant_id ON pipelines(tenant_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_workflow_version_id ON pipelines(workflow_version_id);
CREATE INDEX IF NOT EXISTS idx_workstreams_pipeline_id ON workstreams(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_ledgers_tenant_id ON token_usage_ledgers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_ledgers_workflow_id ON token_usage_ledgers(workflow_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_tenant_id ON chat_sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_enterprise_audit_logs_tenant_id ON enterprise_audit_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_enterprise_audit_logs_actor_id ON enterprise_audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_action_items_tenant_id ON action_items(tenant_id);
CREATE INDEX IF NOT EXISTS idx_action_items_resolved_by ON action_items(resolved_by);
CREATE INDEX IF NOT EXISTS idx_tenant_feature_flags_feature_flag_id ON tenant_feature_flags(feature_flag_id);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skills_agent_id ON skills(agent_id);
CREATE INDEX IF NOT EXISTS idx_swarm_tasks_document_id ON swarm_tasks(document_id);
CREATE INDEX IF NOT EXISTS idx_data_types_tenant_id ON data_types(tenant_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Kembalikan index redundan ke user_profiles
CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id ON user_profiles(user_id);

-- Hapus seluruh index yang dibuat pada Up
DROP INDEX IF EXISTS idx_data_types_tenant_id;
DROP INDEX IF EXISTS idx_swarm_tasks_document_id;
DROP INDEX IF EXISTS idx_skills_agent_id;
DROP INDEX IF EXISTS idx_agents_tenant_id;
DROP INDEX IF EXISTS idx_tenant_feature_flags_feature_flag_id;
DROP INDEX IF EXISTS idx_action_items_resolved_by;
DROP INDEX IF EXISTS idx_action_items_tenant_id;
DROP INDEX IF EXISTS idx_enterprise_audit_logs_actor_id;
DROP INDEX IF EXISTS idx_enterprise_audit_logs_tenant_id;
DROP INDEX IF EXISTS idx_chat_messages_session_id;
DROP INDEX IF EXISTS idx_chat_sessions_user_id;
DROP INDEX IF EXISTS idx_chat_sessions_tenant_id;
DROP INDEX IF EXISTS idx_token_usage_ledgers_workflow_id;
DROP INDEX IF EXISTS idx_token_usage_ledgers_tenant_id;
DROP INDEX IF EXISTS idx_workstreams_pipeline_id;
DROP INDEX IF EXISTS idx_pipelines_workflow_version_id;
DROP INDEX IF EXISTS idx_pipelines_tenant_id;
DROP INDEX IF EXISTS idx_workflows_tenant_id;
DROP INDEX IF EXISTS idx_documents_status;
DROP INDEX IF EXISTS idx_documents_user_id;
DROP INDEX IF EXISTS idx_documents_tenant_id;
DROP INDEX IF EXISTS idx_tenant_users_role_id;
DROP INDEX IF EXISTS idx_tenant_users_user_id;
DROP INDEX IF EXISTS idx_roles_tenant_id;
DROP INDEX IF EXISTS idx_sso_identities_user_id;

-- +goose StatementEnd
