-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chat_sessions (
    id UUID PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_user_org_session ON chat_sessions(user_id, organization_id);
CREATE INDEX idx_chat_sessions_deleted_at ON chat_sessions(deleted_at);

CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_session_created ON chat_messages(session_id, created_at);
CREATE INDEX idx_chat_messages_deleted_at ON chat_messages(deleted_at);

COMMENT ON TABLE chat_sessions IS 'Stores chat sessions for the hybrid Redis/Postgres history system';
COMMENT ON TABLE chat_messages IS 'Stores individual messages within chat sessions';
COMMENT ON COLUMN chat_messages.role IS 'Message role: user or assistant';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_sessions;
-- +goose StatementEnd
