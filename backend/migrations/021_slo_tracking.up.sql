-- SLO (Service Level Objective) / error budget tracking per software service.
--
-- A slo_definition declares a target (e.g. 99.9% availability) measured over
-- a rolling window (measurement_window_days). Actual attainment and error
-- budget burn are computed on read from the existing `incidents` table (see
-- backend/internal/services/slo_service.go) -- this migration only stores
-- the target definitions, not pre-aggregated metrics.
CREATE TABLE slo_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    software_id UUID NOT NULL REFERENCES software_catalog(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slo_type VARCHAR(20) NOT NULL CHECK (slo_type IN ('availability', 'latency', 'error_rate')),
    target_percentage NUMERIC(6,3) NOT NULL CHECK (target_percentage > 0 AND target_percentage <= 100),
    measurement_window_days INTEGER NOT NULL DEFAULT 30 CHECK (measurement_window_days > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_slo_definitions_org ON slo_definitions(org_id);
CREATE INDEX idx_slo_definitions_software ON slo_definitions(software_id);
