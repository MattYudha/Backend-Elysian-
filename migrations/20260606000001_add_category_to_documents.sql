-- +goose Up
ALTER TABLE "documents" ADD COLUMN IF NOT EXISTS "category" VARCHAR(50) DEFAULT 'general';

-- +goose Down
ALTER TABLE "documents" DROP COLUMN IF EXISTS "category";
