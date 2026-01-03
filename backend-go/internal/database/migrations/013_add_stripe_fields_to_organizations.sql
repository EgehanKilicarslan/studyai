-- +goose Up
-- +goose StatementBegin

-- Add Stripe billing fields to organizations table that were missing from the original migration
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS subscription_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMP;

-- Add index for stripe_customer_id for lookups
CREATE INDEX IF NOT EXISTS idx_organizations_stripe_customer_id ON organizations(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

-- Add index for billing_status for filtering organizations by status
CREATE INDEX IF NOT EXISTS idx_organizations_billing_status ON organizations(billing_status);

-- Add constraint to ensure plan_tier is valid
ALTER TABLE organizations
    ADD CONSTRAINT chk_organizations_plan_tier_valid CHECK (plan_tier IN ('FREE', 'PRO', 'ENTERPRISE'));

-- Add constraint to ensure billing_status is valid
ALTER TABLE organizations
    ADD CONSTRAINT chk_organizations_billing_status_valid CHECK (billing_status IN ('active', 'past_due', 'canceled', 'trialing', 'suspended'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations DROP CONSTRAINT IF EXISTS chk_organizations_billing_status_valid;
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS chk_organizations_plan_tier_valid;
DROP INDEX IF EXISTS idx_organizations_billing_status;
DROP INDEX IF EXISTS idx_organizations_stripe_customer_id;
ALTER TABLE organizations
    DROP COLUMN IF EXISTS current_period_end,
    DROP COLUMN IF EXISTS subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id;

-- +goose StatementEnd
