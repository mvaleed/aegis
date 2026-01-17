-- 001_initial_schema.up.sql
-- Initial database schema for user-service

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- User types enum
CREATE TYPE user_type AS ENUM ('admin', 'customer', 'partner');

-- User status enum
CREATE TYPE user_status AS ENUM ('pending', 'active', 'inactive', 'suspended');

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    username VARCHAR(50) NOT NULL,
    full_name VARCHAR(200) NOT NULL,
    user_type user_type NOT NULL DEFAULT 'customer',
    status user_status NOT NULL DEFAULT 'pending',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_username_unique UNIQUE (username)
);

-- Index for email lookup (case-insensitive)
CREATE INDEX idx_users_email ON users (LOWER(email));

-- Index for username lookup
CREATE INDEX idx_users_username ON users (username);

-- Index for listing non-deleted users by status
CREATE INDEX idx_users_status ON users (status) WHERE deleted_at IS NULL;

-- Index for soft-deleted users
CREATE INDEX idx_users_deleted ON users (deleted_at) WHERE deleted_at IS NOT NULL;

-- Permissions table (resource:action model)
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT permissions_resource_action_unique UNIQUE (resource, action)
);

-- Index for permission lookup
CREATE INDEX idx_permissions_resource ON permissions (resource);

-- Roles table
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT roles_name_unique UNIQUE (name)
);

-- Many-to-many: roles to permissions
CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (role_id, permission_id)
);

-- Index for looking up permissions by role
CREATE INDEX idx_role_permissions_role ON role_permissions (role_id);

-- Index for checking which roles have a permission
CREATE INDEX idx_role_permissions_permission ON role_permissions (permission_id);

-- Many-to-many: users to roles
CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (user_id, role_id)
);

-- Index for looking up roles by user
CREATE INDEX idx_user_roles_user ON user_roles (user_id);

-- Index for listing users by role
CREATE INDEX idx_user_roles_role ON user_roles (role_id);

-- Refresh tokens table
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    ip_address VARCHAR(45),
    user_agent TEXT,
    replaced_by_id UUID REFERENCES refresh_tokens(id),
    
    CONSTRAINT refresh_tokens_hash_unique UNIQUE (token_hash)
);

-- Index for token lookup by hash
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens (token_hash);

-- Index for user's active tokens
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id) WHERE revoked_at IS NULL;

-- Index for cleanup of expired tokens
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens (expires_at);

-- Insert default roles
INSERT INTO roles (id, name, description) VALUES
    (uuid_generate_v4(), 'admin', 'Full system administrator'),
    (uuid_generate_v4(), 'user', 'Standard user role'),
    (uuid_generate_v4(), 'moderator', 'Content moderator');

-- Insert default permissions
INSERT INTO permissions (id, resource, action, description) VALUES
    (uuid_generate_v4(), 'users', 'read', 'View user information'),
    (uuid_generate_v4(), 'users', 'write', 'Create and update users'),
    (uuid_generate_v4(), 'users', 'delete', 'Delete users'),
    (uuid_generate_v4(), 'users', 'admin', 'Full user administration'),
    (uuid_generate_v4(), 'roles', 'read', 'View roles'),
    (uuid_generate_v4(), 'roles', 'write', 'Create and update roles'),
    (uuid_generate_v4(), 'roles', 'delete', 'Delete roles'),
    (uuid_generate_v4(), 'roles', 'assign', 'Assign roles to users'),
    (uuid_generate_v4(), '*', '*', 'Super admin - all permissions');

-- Assign all permissions to admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'admin';

-- Assign basic read permissions to user role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'user' AND p.resource = 'users' AND p.action = 'read';

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update updated_at on users table
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Trigger to auto-update updated_at on roles table
CREATE TRIGGER update_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
