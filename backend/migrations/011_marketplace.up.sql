-- Agent Marketplace
CREATE TABLE marketplace_agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    long_description TEXT,
    author VARCHAR(255) NOT NULL,
    author_url VARCHAR(500),
    version VARCHAR(50) NOT NULL DEFAULT '0.1.0',
    category VARCHAR(50) NOT NULL CHECK (category IN ('triage', 'evidence', 'rca', 'postmortem', 'security', 'infrastructure', 'database', 'cloud', 'custom')),
    icon_url VARCHAR(500),
    docker_image VARCHAR(500),
    agent_card JSONB NOT NULL DEFAULT '{}',
    skills JSONB NOT NULL DEFAULT '[]',
    required_credentials JSONB DEFAULT '[]',
    config_schema JSONB DEFAULT '{}',
    readme TEXT,
    downloads INTEGER DEFAULT 0,
    rating NUMERIC(2,1) DEFAULT 0.0,
    verified BOOLEAN DEFAULT false,
    published BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Installed marketplace agents per org
CREATE TABLE installed_agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    marketplace_agent_id UUID NOT NULL REFERENCES marketplace_agents(id),
    a2a_agent_id UUID REFERENCES a2a_agents(id),
    config JSONB DEFAULT '{}',
    version VARCHAR(50),
    status VARCHAR(20) DEFAULT 'installed' CHECK (status IN ('installing', 'installed', 'updating', 'failed', 'uninstalled')),
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, marketplace_agent_id)
);

CREATE INDEX idx_marketplace_agents_category ON marketplace_agents(category);
CREATE INDEX idx_marketplace_agents_slug ON marketplace_agents(slug);
CREATE INDEX idx_installed_agents_org ON installed_agents(org_id);
