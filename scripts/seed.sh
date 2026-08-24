#!/bin/bash
set -e

DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-rootcauseway}
DB_NAME=${DB_NAME:-rootcauseway}

export PGPASSWORD=${DB_PASS:-rootcauseway_dev_password}

psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" <<'SQL'
-- Seed org
INSERT INTO organizations (id, name, slug) VALUES
  ('00000000-0000-0000-0000-000000000001', 'RootCauseway Demo', 'rootcauseway-demo')
ON CONFLICT (slug) DO NOTHING;

-- Seed admin user (password: admin123!)
INSERT INTO users (id, org_id, name, email, password_hash, role) VALUES
  ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'Admin', 'admin@rootcauseway.local', '$2a$10$placeholder_hash_replace_on_first_run', 'admin')
ON CONFLICT (org_id, email) DO NOTHING;

-- Seed software catalog
INSERT INTO software_catalog (id, org_id, name, slug, description, status) VALUES
  ('00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000001', 'API Gateway', 'api-gateway', 'Main API gateway service', 'active'),
  ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000001', 'Payment Service', 'payment-service', 'Payment processing service', 'active'),
  ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000001', 'User Service', 'user-service', 'User management service', 'active')
ON CONFLICT (org_id, slug) DO NOTHING;

-- Seed default agents
INSERT INTO agents (id, org_id, name, type, description, config) VALUES
  ('00000000-0000-0000-0000-000000001000', '00000000-0000-0000-0000-000000000001', 'Triage Agent', 'triage', 'Classifies and prioritizes incoming alerts', '{"model": "claude-sonnet-4-6", "temperature": 0.3}'),
  ('00000000-0000-0000-0000-000000001001', '00000000-0000-0000-0000-000000000001', 'Evidence Analyst', 'evidence_analysis', 'Collects and correlates evidence from multiple sources', '{"model": "claude-sonnet-4-6", "temperature": 0.2}'),
  ('00000000-0000-0000-0000-000000001002', '00000000-0000-0000-0000-000000000001', 'Hypothesis Generator', 'hypothesis', 'Generates root cause hypotheses from evidence', '{"model": "claude-sonnet-4-6", "temperature": 0.5}')
ON CONFLICT DO NOTHING;

SELECT 'Seed completed successfully' AS status;
SQL
