-- Observability Data Sources (Datadog, Prometheus, Loki, Tempo, Grafana, etc.)
CREATE TABLE observability_sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN (
        'datadog', 'prometheus', 'loki', 'tempo', 'grafana',
        'elasticsearch', 'splunk', 'cloudwatch', 'azure_monitor',
        'gcp_monitoring', 'newrelic', 'dynatrace', 'jaeger', 'zipkin', 'custom'
    )),
    base_url VARCHAR(500) NOT NULL,
    auth_type VARCHAR(20) NOT NULL DEFAULT 'api_key' CHECK (auth_type IN ('api_key', 'bearer', 'basic', 'oauth2', 'none')),
    auth_config JSONB NOT NULL DEFAULT '{}',
    -- What this source provides
    capabilities JSONB DEFAULT '["metrics"]',
    -- Link to software catalog entries this source monitors
    monitored_software_ids JSONB DEFAULT '[]',
    -- Connection settings
    timeout_seconds INTEGER DEFAULT 30,
    verify_ssl BOOLEAN DEFAULT true,
    custom_headers JSONB DEFAULT '{}',
    -- Health
    enabled BOOLEAN NOT NULL DEFAULT true,
    health_status VARCHAR(20) DEFAULT 'unknown' CHECK (health_status IN ('healthy', 'unhealthy', 'unknown')),
    last_health_check TIMESTAMPTZ,
    -- Metadata
    description TEXT,
    environment VARCHAR(50),
    region VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Snapshot Configs (what to capture from each source when alert arrives)
CREATE TABLE snapshot_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_id UUID NOT NULL REFERENCES observability_sources(id) ON DELETE CASCADE,
    software_id UUID REFERENCES software_catalog(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    snapshot_type VARCHAR(30) NOT NULL CHECK (snapshot_type IN ('metrics', 'logs', 'traces', 'dashboard', 'alerts', 'custom')),
    -- Query config
    query_template TEXT NOT NULL,
    time_range_seconds INTEGER DEFAULT 1800,
    parameters JSONB DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_observability_sources_org ON observability_sources(org_id);
CREATE INDEX idx_observability_sources_type ON observability_sources(source_type);
CREATE INDEX idx_snapshot_configs_source ON snapshot_configs(source_id);
CREATE INDEX idx_snapshot_configs_software ON snapshot_configs(software_id);
