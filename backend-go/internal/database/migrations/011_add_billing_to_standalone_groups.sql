-- +goose Up
-- +goose StatementBegin

-- Add plan and billing fields to groups table for standalone group billing
-- These fields are only relevant when organization_id IS NULL (standalone groups)
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS plan_tier VARCHAR(50) NOT NULL DEFAULT 'FREE',
    ADD COLUMN IF NOT EXISTS billing_status VARCHAR(50) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS subscription_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMP,
    ADD COLUMN IF NOT EXISTS used_storage_bytes BIGINT NOT NULL DEFAULT 0;

-- Add index for plan_tier for filtering standalone groups by tier
CREATE INDEX IF NOT EXISTS idx_groups_plan_tier ON groups(plan_tier);

-- Add index for stripe_customer_id for lookups
CREATE INDEX IF NOT EXISTS idx_groups_stripe_customer_id ON groups(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

-- Add constraint to ensure used_storage_bytes is non-negative
ALTER TABLE groups
    ADD CONSTRAINT chk_groups_used_storage_non_negative CHECK (used_storage_bytes >= 0);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE groups DROP CONSTRAINT IF EXISTS chk_groups_used_storage_non_negative;
DROP INDEX IF EXISTS idx_groups_stripe_customer_id;
DROP INDEX IF EXISTS idx_groups_plan_tier;
ALTER TABLE groups
    DROP COLUMN IF EXISTS used_storage_bytes,
    DROP COLUMN IF EXISTS current_period_end,
    DROP COLUMN IF EXISTS subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS billing_status,
    DROP COLUMN IF EXISTS plan_tier;

-- +goose StatementEnd
