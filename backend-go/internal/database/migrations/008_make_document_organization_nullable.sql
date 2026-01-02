-- +goose Up
-- Migration: Make documents.organization_id nullable for user-scoped documents

ALTER TABLE documents ALTER COLUMN organization_id DROP NOT NULL;

COMMENT ON COLUMN documents.organization_id IS 'NULL for user-scoped documents, foreign key for organization/group-scoped documents';

-- +goose Down
-- Rollback: Make organization_id NOT NULL again (will fail if NULL values exist)

ALTER TABLE documents ALTER COLUMN organization_id SET NOT NULL;
