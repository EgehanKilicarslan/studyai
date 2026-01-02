-- +goose Up
-- +goose StatementBegin
-- Make organization_id nullable to support user-level chat sessions without organization context
ALTER TABLE chat_sessions 
    ALTER COLUMN organization_id DROP NOT NULL,
    DROP CONSTRAINT IF EXISTS chat_sessions_organization_id_fkey;

-- Re-add foreign key without NOT NULL requirement
ALTER TABLE chat_sessions 
    ADD CONSTRAINT chat_sessions_organization_id_fkey 
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL;

COMMENT ON COLUMN chat_sessions.organization_id IS 'Optional organization context for the chat session. NULL for user-level sessions.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert: make organization_id NOT NULL again
-- First, delete sessions without organization (or assign a default)
DELETE FROM chat_sessions WHERE organization_id IS NULL;

ALTER TABLE chat_sessions 
    DROP CONSTRAINT IF EXISTS chat_sessions_organization_id_fkey;

ALTER TABLE chat_sessions 
    ALTER COLUMN organization_id SET NOT NULL,
    ADD CONSTRAINT chat_sessions_organization_id_fkey 
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
-- +goose StatementEnd
