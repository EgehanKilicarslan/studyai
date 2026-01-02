-- +goose Up
-- +goose StatementBegin

-- Add plan and billing fields to organizations table
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS plan_tier VARCHAR(50) NOT NULL DEFAULT 'FREE',
    ADD COLUMN IF NOT EXISTS billing_status VARCHAR(50) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS used_storage_bytes BIGINT NOT NULL DEFAULT 0;

-- Add index for plan_tier for filtering by tier
CREATE INDEX IF NOT EXISTS idx_organizations_plan_tier ON organizations(plan_tier);

-- Add constraint to ensure used_storage_bytes is non-negative
ALTER TABLE organizations
    ADD CONSTRAINT chk_used_storage_non_negative CHECK (used_storage_bytes >= 0);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations DROP CONSTRAINT IF EXISTS chk_used_storage_non_negative;
DROP INDEX IF EXISTS idx_organizations_plan_tier;
ALTER TABLE organizations
    DROP COLUMN IF EXISTS used_storage_bytes,
    DROP COLUMN IF EXISTS billing_status,
    DROP COLUMN IF EXISTS plan_tier;

-- +goose StatementEnd
