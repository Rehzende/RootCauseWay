-- Roles and Permissions (RBAC)
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    UNIQUE(resource, action)
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Update users table for SSO
ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_provider VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_subject VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(500);
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;

-- User-Role mapping (replaces simple role column)
CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- SSO/OIDC Providers config
CREATE TABLE sso_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider_type VARCHAR(30) NOT NULL CHECK (provider_type IN ('oidc', 'saml', 'google', 'github', 'azure_ad', 'okta')),
    client_id VARCHAR(500) NOT NULL,
    client_secret VARCHAR(500),
    issuer_url VARCHAR(500),
    authorization_url VARCHAR(500),
    token_url VARCHAR(500),
    userinfo_url VARCHAR(500),
    scopes VARCHAR(500) DEFAULT 'openid profile email',
    auto_provision_users BOOLEAN DEFAULT true,
    default_role_id UUID REFERENCES roles(id),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- API Keys (for programmatic access)
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(10) NOT NULL,
    role_id UUID REFERENCES roles(id),
    scopes JSONB DEFAULT '[]',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit Log
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB DEFAULT '{}',
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    request_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sessions (for SSO token management)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    refresh_token_hash VARCHAR(255),
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed system permissions
INSERT INTO permissions (id, resource, action, description) VALUES
    (uuid_generate_v4(), 'incidents', 'read', 'View incidents'),
    (uuid_generate_v4(), 'incidents', 'write', 'Create and update incidents'),
    (uuid_generate_v4(), 'incidents', 'delete', 'Delete incidents'),
    (uuid_generate_v4(), 'software', 'read', 'View software catalog'),
    (uuid_generate_v4(), 'software', 'write', 'Manage software catalog'),
    (uuid_generate_v4(), 'agents', 'read', 'View agents'),
    (uuid_generate_v4(), 'agents', 'write', 'Manage agents'),
    (uuid_generate_v4(), 'skills', 'read', 'View skills'),
    (uuid_generate_v4(), 'skills', 'write', 'Manage skills'),
    (uuid_generate_v4(), 'webhooks', 'read', 'View webhooks'),
    (uuid_generate_v4(), 'webhooks', 'write', 'Manage webhooks'),
    (uuid_generate_v4(), 'credentials', 'read', 'View credentials'),
    (uuid_generate_v4(), 'credentials', 'write', 'Manage credentials'),
    (uuid_generate_v4(), 'runbooks', 'read', 'View runbooks'),
    (uuid_generate_v4(), 'runbooks', 'write', 'Manage runbooks'),
    (uuid_generate_v4(), 'runbooks', 'execute', 'Execute runbooks'),
    (uuid_generate_v4(), 'notifications', 'read', 'View notification config'),
    (uuid_generate_v4(), 'notifications', 'write', 'Manage notifications'),
    (uuid_generate_v4(), 'analytics', 'read', 'View analytics'),
    (uuid_generate_v4(), 'settings', 'read', 'View settings'),
    (uuid_generate_v4(), 'settings', 'write', 'Manage settings'),
    (uuid_generate_v4(), 'users', 'read', 'View users'),
    (uuid_generate_v4(), 'users', 'write', 'Manage users'),
    (uuid_generate_v4(), 'audit', 'read', 'View audit log')
ON CONFLICT (resource, action) DO NOTHING;

CREATE INDEX idx_roles_org ON roles(org_id);
CREATE INDEX idx_user_roles_user ON user_roles(user_id);
CREATE INDEX idx_user_roles_role ON user_roles(role_id);
CREATE INDEX idx_sso_providers_org ON sso_providers(org_id);
CREATE INDEX idx_api_keys_org ON api_keys(org_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_audit_log_org ON audit_log(org_id);
CREATE INDEX idx_audit_log_user ON audit_log(user_id);
CREATE INDEX idx_audit_log_action ON audit_log(org_id, action);
CREATE INDEX idx_audit_log_resource ON audit_log(resource_type, resource_id);
CREATE INDEX idx_audit_log_created ON audit_log(org_id, created_at);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
