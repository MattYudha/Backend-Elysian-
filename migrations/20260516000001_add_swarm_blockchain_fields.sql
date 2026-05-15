-- +goose Up
-- +goose StatementBegin
ALTER TABLE swarm_tasks
    ADD COLUMN IF NOT EXISTS rationale_hash VARCHAR(128),
    ADD COLUMN IF NOT EXISTS consensus_hash VARCHAR(128),
    ADD COLUMN IF NOT EXISTS blockchain_tx VARCHAR(128),
    ADD COLUMN IF NOT EXISTS blockchain_net VARCHAR(50),
    ADD COLUMN IF NOT EXISTS blockchain_stat VARCHAR(50) DEFAULT 'PENDING_COMMIT';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE swarm_tasks
    DROP COLUMN IF EXISTS rationale_hash,
    DROP COLUMN IF EXISTS consensus_hash,
    DROP COLUMN IF EXISTS blockchain_tx,
    DROP COLUMN IF EXISTS blockchain_net,
    DROP COLUMN IF EXISTS blockchain_stat;
-- +goose StatementEnd
