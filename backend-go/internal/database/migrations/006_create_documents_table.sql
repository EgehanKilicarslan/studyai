-- +goose Up
-- +goose StatementBegin

-- Document status enum type
CREATE TYPE document_status AS ENUM ('PENDING', 'PROCESSING', 'COMPLETED', 'ERROR');

-- Documents table (Go is the source of truth)
-- Supports three scoping modes:
-- - User-scoped (private): organization_id=NULL, group_id=NULL
-- - Organization-scoped: organization_id set, group_id=NULL
-- - Group-scoped: organization_id set, group_id set
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id INTEGER,
    group_id INTEGER,
    owner_id INTEGER NOT NULL,
    name VARCHAR(512) NOT NULL,
    file_path VARCHAR(1024) NOT NULL,
    file_hash VARCHAR(64),
    file_size BIGINT DEFAULT 0,
    content_type VARCHAR(255) DEFAULT 'application/octet-stream',
    status document_status NOT NULL DEFAULT 'PENDING',
    chunks_count INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_documents_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_documents_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL,
    CONSTRAINT fk_documents_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_documents_organization_id ON documents(organization_id);
CREATE INDEX IF NOT EXISTS idx_documents_group_id ON documents(group_id);
CREATE INDEX IF NOT EXISTS idx_documents_owner_id ON documents(owner_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_file_hash ON documents(file_hash);
CREATE INDEX IF NOT EXISTS idx_documents_deleted_at ON documents(deleted_at);

COMMENT ON COLUMN documents.organization_id IS 'NULL for user-scoped documents, foreign key for organization/group-scoped documents';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS documents;
DROP TYPE IF EXISTS document_status;
-- +goose StatementEnd
