-- +goose Up
-- +goose StatementBegin

-- Add plan and billing fields to users table for individual user billing
-- These fields enable users to have their own subscription plans
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS plan_tier VARCHAR(50) NOT NULL DEFAULT 'FREE',
    ADD COLUMN IF NOT EXISTS billing_status VARCHAR(50) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS subscription_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMP,
    ADD COLUMN IF NOT EXISTS used_storage_bytes BIGINT NOT NULL DEFAULT 0;

-- Add index for plan_tier for filtering users by tier
CREATE INDEX IF NOT EXISTS idx_users_plan_tier ON users(plan_tier);

-- Add index for billing_status for filtering users by status
CREATE INDEX IF NOT EXISTS idx_users_billing_status ON users(billing_status);

-- Add index for stripe_customer_id for lookups
CREATE INDEX IF NOT EXISTS idx_users_stripe_customer_id ON users(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

-- Add constraint to ensure used_storage_bytes is non-negative
ALTER TABLE users
    ADD CONSTRAINT chk_users_used_storage_non_negative CHECK (used_storage_bytes >= 0);

-- Add constraint to ensure plan_tier is valid
ALTER TABLE users
    ADD CONSTRAINT chk_users_plan_tier_valid CHECK (plan_tier IN ('FREE', 'PRO', 'ENTERPRISE'));

-- Add constraint to ensure billing_status is valid
ALTER TABLE users
    ADD CONSTRAINT chk_users_billing_status_valid CHECK (billing_status IN ('active', 'past_due', 'canceled', 'trialing'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_billing_status_valid;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_plan_tier_valid;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_used_storage_non_negative;
DROP INDEX IF EXISTS idx_users_stripe_customer_id;
DROP INDEX IF EXISTS idx_users_billing_status;
DROP INDEX IF EXISTS idx_users_plan_tier;
ALTER TABLE users
    DROP COLUMN IF EXISTS used_storage_bytes,
    DROP COLUMN IF EXISTS current_period_end,
    DROP COLUMN IF EXISTS subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS billing_status,
    DROP COLUMN IF EXISTS plan_tier;

-- +goose StatementEnd
