-- RootCauseway Database Schema Contract
-- All services must agree on this schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Organizations (multi-tenant)
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'operator', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, email)
);

-- Software Catalog
CREATE TABLE software_catalog (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    owner_id UUID REFERENCES users(id),
    repository_url VARCHAR(500),
    tags JSONB DEFAULT '[]',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

-- Agent Registry
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(30) NOT NULL CHECK (type IN ('triage', 'evidence_analysis', 'hypothesis', 'debug', 'custom')),
    description TEXT,
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Webhooks
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    source VARCHAR(30) NOT NULL CHECK (source IN ('datadog', 'prometheus_alertmanager', 'grafana', 'otel', 'custom')),
    software_id UUID NOT NULL REFERENCES software_catalog(id) ON DELETE CASCADE,
    endpoint_token VARCHAR(64) NOT NULL UNIQUE,
    secret VARCHAR(255),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Incidents
CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    software_id UUID NOT NULL REFERENCES software_catalog(id),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity VARCHAR(10) NOT NULL DEFAULT 'medium' CHECK (severity IN ('critical', 'high', 'medium', 'low')),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'investigating', 'mitigated', 'resolved', 'closed')),
    assignee_id UUID REFERENCES users(id),
    source_alert_id VARCHAR(255),
    root_cause TEXT,
    mitigation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

-- Incident Timeline Events
CREATE TABLE incident_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL CHECK (type IN ('alert_received', 'triage_started', 'triage_completed', 'evidence_collected', 'hypothesis_generated', 'status_changed', 'comment', 'agent_action')),
    actor VARCHAR(255),
    data JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Incident Evidence
CREATE TABLE incident_evidence (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('log', 'metric', 'trace', 'snapshot', 'agent_output', 'manual')),
    title VARCHAR(500) NOT NULL,
    content JSONB NOT NULL,
    source VARCHAR(255),
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Alert Snapshots
CREATE TABLE alert_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    software_id UUID NOT NULL REFERENCES software_catalog(id),
    raw_payload JSONB NOT NULL,
    normalized JSONB NOT NULL,
    snapshots JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_users_org ON users(org_id);
CREATE INDEX idx_software_org ON software_catalog(org_id);
CREATE INDEX idx_agents_org ON agents(org_id);
CREATE INDEX idx_webhooks_org ON webhooks(org_id);
CREATE INDEX idx_webhooks_token ON webhooks(endpoint_token);
CREATE INDEX idx_incidents_org ON incidents(org_id);
CREATE INDEX idx_incidents_status ON incidents(org_id, status);
CREATE INDEX idx_incidents_software ON incidents(software_id);
CREATE INDEX idx_incident_events_incident ON incident_events(incident_id);
CREATE INDEX idx_incident_evidence_incident ON incident_evidence(incident_id);
CREATE INDEX idx_alert_snapshots_incident ON alert_snapshots(incident_id);
