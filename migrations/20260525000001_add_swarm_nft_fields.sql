-- +goose Up
-- +goose StatementBegin
ALTER TABLE swarm_tasks
    ADD COLUMN IF NOT EXISTS nft_token_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS ipfs_cid VARCHAR(255),
    ADD COLUMN IF NOT EXISTS nft_tx_hash VARCHAR(255);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE swarm_tasks
    DROP COLUMN IF EXISTS nft_token_id,
    DROP COLUMN IF EXISTS ipfs_cid,
    DROP COLUMN IF EXISTS nft_tx_hash;
-- +goose StatementEnd
