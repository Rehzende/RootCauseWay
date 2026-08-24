-- Skills Registry
CREATE TABLE skills (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL CHECK (category IN ('infrastructure', 'application', 'database', 'network', 'security', 'cloud', 'observability', 'custom')),
    -- Skill definition
    prompt_template TEXT,
    input_schema JSONB DEFAULT '{}',
    output_schema JSONB DEFAULT '{}',
    required_tools JSONB DEFAULT '[]',
    -- What resources this skill needs access to
    required_resource_types JSONB DEFAULT '[]',
    required_permissions JSONB DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

-- Agent <-> Skill mapping (many-to-many)
CREATE TABLE agent_skills (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES a2a_agents(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    priority INTEGER DEFAULT 0,
    config_overrides JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, skill_id)
);

-- Credential Providers (Vault, AWS STS, Azure MI, GCP WI, static)
CREATE TABLE credential_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider_type VARCHAR(30) NOT NULL CHECK (provider_type IN ('hashicorp_vault', 'aws_sts', 'azure_managed_identity', 'gcp_workload_identity', 'static', 'custom')),
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Resource Credentials (links software resources to credential providers)
CREATE TABLE resource_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    software_id UUID NOT NULL REFERENCES software_catalog(id) ON DELETE CASCADE,
    resource_name VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50) NOT NULL CHECK (resource_type IN ('kubernetes_cluster', 'database', 'cloud_account', 'api_endpoint', 'storage', 'message_queue', 'cache', 'custom')),
    provider_id UUID NOT NULL REFERENCES credential_providers(id),
    -- How to request credentials from the provider
    credential_path VARCHAR(500),
    default_ttl_seconds INTEGER DEFAULT 900,
    max_ttl_seconds INTEGER DEFAULT 3600,
    default_scope JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Access Policies (what agents/skills can access)
CREATE TABLE access_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    -- What this policy applies to (agent, skill, or agent_type)
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('agent', 'skill', 'agent_type')),
    target_id VARCHAR(255) NOT NULL,
    -- What resources this policy grants access to
    resource_type VARCHAR(50) NOT NULL,
    allowed_actions JSONB NOT NULL DEFAULT '["read"]',
    scope_restrictions JSONB DEFAULT '{}',
    max_ttl_seconds INTEGER DEFAULT 900,
    require_approval BOOLEAN DEFAULT false,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- JIT Credential Leases (audit trail of issued credentials)
CREATE TABLE credential_leases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id),
    agent_id UUID REFERENCES a2a_agents(id),
    skill_id UUID REFERENCES skills(id),
    resource_credential_id UUID NOT NULL REFERENCES resource_credentials(id),
    policy_id UUID REFERENCES access_policies(id),
    -- Lease details
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'revoked')),
    scope JSONB NOT NULL DEFAULT '{}',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoked_by VARCHAR(255),
    -- Audit
    request_reason TEXT,
    actions_performed JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_skills_org ON skills(org_id);
CREATE INDEX idx_skills_category ON skills(org_id, category);
CREATE INDEX idx_agent_skills_agent ON agent_skills(agent_id);
CREATE INDEX idx_agent_skills_skill ON agent_skills(skill_id);
CREATE INDEX idx_credential_providers_org ON credential_providers(org_id);
CREATE INDEX idx_resource_credentials_software ON resource_credentials(software_id);
CREATE INDEX idx_resource_credentials_provider ON resource_credentials(provider_id);
CREATE INDEX idx_access_policies_org ON access_policies(org_id);
CREATE INDEX idx_access_policies_target ON access_policies(target_type, target_id);
CREATE INDEX idx_credential_leases_incident ON credential_leases(incident_id);
CREATE INDEX idx_credential_leases_status ON credential_leases(status);
CREATE INDEX idx_credential_leases_expires ON credential_leases(expires_at);
