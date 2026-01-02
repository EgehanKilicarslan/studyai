-- +goose Up
-- Migration: Add dynamic organization roles and make groups support standalone mode
-- This migration:
-- 1. Creates organization_roles table for dynamic org-level roles
-- 2. Migrates organization_members from string role to role_id
-- 3. Makes groups.organization_id nullable for standalone groups

-- Step 1: Create organization_roles table
CREATE TABLE IF NOT EXISTS organization_roles (
    id SERIAL PRIMARY KEY,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    permissions TEXT[] DEFAULT '{}',
    is_system_role BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX idx_organization_roles_organization_id ON organization_roles(organization_id);
CREATE INDEX idx_organization_roles_deleted_at ON organization_roles(deleted_at);

-- Step 2: Create default system roles for existing organizations
INSERT INTO organization_roles (organization_id, name, permissions, is_system_role)
SELECT 
    id AS organization_id,
    'owner' AS name,
    ARRAY['org:*', 'group:*', 'member:*', 'role:*', 'document:*'] AS permissions,
    TRUE AS is_system_role
FROM organizations
WHERE NOT EXISTS (
    SELECT 1 FROM organization_roles 
    WHERE organization_roles.organization_id = organizations.id 
    AND organization_roles.name = 'owner'
);

INSERT INTO organization_roles (organization_id, name, permissions, is_system_role)
SELECT 
    id AS organization_id,
    'admin' AS name,
    ARRAY['group:create', 'group:read', 'group:update', 'group:delete', 'member:invite', 'member:read', 'document:*'] AS permissions,
    TRUE AS is_system_role
FROM organizations
WHERE NOT EXISTS (
    SELECT 1 FROM organization_roles 
    WHERE organization_roles.organization_id = organizations.id 
    AND organization_roles.name = 'admin'
);

INSERT INTO organization_roles (organization_id, name, permissions, is_system_role)
SELECT 
    id AS organization_id,
    'member' AS name,
    ARRAY['group:read', 'member:read', 'document:read', 'document:create'] AS permissions,
    TRUE AS is_system_role
FROM organizations
WHERE NOT EXISTS (
    SELECT 1 FROM organization_roles 
    WHERE organization_roles.organization_id = organizations.id 
    AND organization_roles.name = 'member'
);

-- Step 3: Add role_id column to organization_members (temporarily nullable)
ALTER TABLE organization_members ADD COLUMN IF NOT EXISTS role_id INTEGER;

-- Step 4: Migrate existing members to use role_id based on their string role
UPDATE organization_members om
SET role_id = (
    SELECT or_table.id 
    FROM organization_roles or_table
    WHERE or_table.organization_id = om.organization_id 
    AND or_table.name = om.role
    LIMIT 1
)
WHERE om.role_id IS NULL;

-- Step 5: Make role_id NOT NULL and add foreign key
ALTER TABLE organization_members ALTER COLUMN role_id SET NOT NULL;
ALTER TABLE organization_members ADD CONSTRAINT fk_organization_members_role 
    FOREIGN KEY (role_id) REFERENCES organization_roles(id) ON DELETE RESTRICT;

CREATE INDEX idx_organization_members_role_id ON organization_members(role_id);

-- Step 6: Drop the old role column
ALTER TABLE organization_members DROP COLUMN IF EXISTS role;

-- Step 7: Make groups.organization_id nullable for standalone groups
ALTER TABLE groups ALTER COLUMN organization_id DROP NOT NULL;

-- Add comment to explain nullable organization_id
COMMENT ON COLUMN groups.organization_id IS 'NULL for standalone groups, foreign key to organizations for organizational groups';

-- +goose Down
-- Rollback: Reverse all changes
ALTER TABLE groups ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE organization_members ADD COLUMN role VARCHAR(50);
UPDATE organization_members om SET role = (SELECT name FROM organization_roles WHERE id = om.role_id);
ALTER TABLE organization_members DROP COLUMN role_id;
DROP TABLE organization_roles;
