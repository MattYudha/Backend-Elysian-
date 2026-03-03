INSERT INTO tenants (id, name, plan_tier, status) VALUES ('11111111-1111-1111-1111-111111111111', 'Test Tenant', 'enterprise', 'active') ON CONFLICT DO NOTHING;

INSERT INTO workflows (id, tenant_id, name, status) VALUES ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'Test DAG Workflow', 'active') ON CONFLICT DO NOTHING;

INSERT INTO workflow_versions (id, workflow_id, version_number, configuration) VALUES (
    '33333333-3333-3333-3333-333333333333',
    '22222222-2222-2222-2222-222222222222',
    1,
    '{"nodes":[{"id":"node_1","type":"llm_agent","data":{"prompt":"Summarize the following text"}},{"id":"node_2","type":"llm_agent","data":{"prompt":"Translate to Bahasa Indonesia"}}],"edges":[{"source":"node_1","target":"node_2"}]}'
) ON CONFLICT DO NOTHING;
