#!/bin/bash
# Seed the marketplace_agents table with the 4 built-in agents
# Usage: DB_URL=postgres://user:pass@host:5432/rootcauseway ./scripts/seed-marketplace.sh

set -euo pipefail

DB_URL="${DB_URL:-postgres://rootcauseway:rootcauseway@localhost:5432/rootcauseway?sslmode=disable}"

echo "Seeding marketplace agents..."

psql "$DB_URL" <<'SQL'
INSERT INTO marketplace_agents (id, name, slug, description, long_description, author, author_url, version, category, icon_url, docker_image, agent_card, skills, required_credentials, config_schema, readme, downloads, rating, verified, published, created_at, updated_at)
VALUES
(
  'a0000000-0000-0000-0000-000000000001',
  'Triage Agent',
  'triage-agent',
  'Automated incident triage, severity classification, and routing',
  'The Triage Agent performs initial incident assessment by analyzing alert data, classifying severity (P1-P4), identifying affected services, and routing to the appropriate team. Uses LLM-powered analysis combined with historical incident patterns.',
  'RootCauseway',
  'https://github.com/Rehzende/RootCauseway',
  '0.1.0',
  'core',
  '',
  'ghcr.io/rootcauseway/triage-agent',
  '{"name":"triage-agent","description":"Incident triage and classification","url":"http://triage-agent:8090","version":"0.1.0","capabilities":{"streaming":false,"pushNotifications":false},"defaultInputModes":["text"],"defaultOutputModes":["text"]}',
  '[{"id":"triage","name":"Triage","description":"Classify and route incidents based on alert data"}]',
  '[]',
  '{"type":"object","properties":{"model":{"type":"string","default":"gpt-4o-mini","description":"LLM model to use"}}}',
  '# Triage Agent\n\nAutomated incident triage using AI-powered classification.',
  0, 5.0, true, true, NOW(), NOW()
),
(
  'a0000000-0000-0000-0000-000000000002',
  'Evidence Agent',
  'evidence-agent',
  'Automated evidence collection from logs, metrics, and traces',
  'The Evidence Agent gathers relevant evidence for incident investigation. It queries log aggregators, metrics systems, and distributed tracing platforms to build a comprehensive evidence timeline. Supports Elasticsearch, Prometheus, Jaeger, and more.',
  'RootCauseway',
  'https://github.com/Rehzende/RootCauseway',
  '0.1.0',
  'core',
  '',
  'ghcr.io/rootcauseway/evidence-agent',
  '{"name":"evidence-agent","description":"Evidence collection and correlation","url":"http://evidence-agent:8091","version":"0.1.0","capabilities":{"streaming":false,"pushNotifications":false},"defaultInputModes":["text"],"defaultOutputModes":["text"]}',
  '[{"id":"collect-evidence","name":"Collect Evidence","description":"Gather logs, metrics, and traces related to an incident"}]',
  '[{"type":"prometheus","description":"Metrics endpoint"},{"type":"elasticsearch","description":"Log aggregator"}]',
  '{"type":"object","properties":{"model":{"type":"string","default":"gpt-4o-mini"},"lookback_minutes":{"type":"integer","default":60}}}',
  '# Evidence Agent\n\nAutomated evidence collection from observability platforms.',
  0, 5.0, true, true, NOW(), NOW()
),
(
  'a0000000-0000-0000-0000-000000000003',
  'RCA Agent',
  'rca-agent',
  'Root cause analysis using causal reasoning and evidence correlation',
  'The RCA Agent analyzes collected evidence to determine the root cause of incidents. Uses causal reasoning, fault tree analysis, and pattern matching against the knowledge base to produce structured root cause reports with confidence scores.',
  'RootCauseway',
  'https://github.com/Rehzende/RootCauseway',
  '0.1.0',
  'core',
  '',
  'ghcr.io/rootcauseway/rca-agent',
  '{"name":"rca-agent","description":"Root cause analysis","url":"http://rca-agent:8092","version":"0.1.0","capabilities":{"streaming":false,"pushNotifications":false},"defaultInputModes":["text"],"defaultOutputModes":["text"]}',
  '[{"id":"analyze","name":"Root Cause Analysis","description":"Determine root cause from evidence using causal reasoning"}]',
  '[]',
  '{"type":"object","properties":{"model":{"type":"string","default":"gpt-4o"},"confidence_threshold":{"type":"number","default":0.7}}}',
  '# RCA Agent\n\nAI-powered root cause analysis using causal reasoning.',
  0, 5.0, true, true, NOW(), NOW()
),
(
  'a0000000-0000-0000-0000-000000000004',
  'Postmortem Agent',
  'postmortem-agent',
  'Automated postmortem report generation with action items',
  'The Postmortem Agent generates comprehensive postmortem reports from incident data, including timeline reconstruction, impact assessment, root cause summary, and actionable follow-up items. Outputs in structured format compatible with common postmortem templates.',
  'RootCauseway',
  'https://github.com/Rehzende/RootCauseway',
  '0.1.0',
  'core',
  '',
  'ghcr.io/rootcauseway/postmortem-agent',
  '{"name":"postmortem-agent","description":"Postmortem report generation","url":"http://postmortem-agent:8093","version":"0.1.0","capabilities":{"streaming":false,"pushNotifications":false},"defaultInputModes":["text"],"defaultOutputModes":["text"]}',
  '[{"id":"generate-postmortem","name":"Generate Postmortem","description":"Create a structured postmortem report from incident data"}]',
  '[]',
  '{"type":"object","properties":{"model":{"type":"string","default":"gpt-4o"},"template":{"type":"string","default":"standard","enum":["standard","brief","detailed"]}}}',
  '# Postmortem Agent\n\nAutomated postmortem generation with actionable insights.',
  0, 5.0, true, true, NOW(), NOW()
)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  long_description = EXCLUDED.long_description,
  version = EXCLUDED.version,
  agent_card = EXCLUDED.agent_card,
  skills = EXCLUDED.skills,
  updated_at = NOW();

SQL

echo "Done. Seeded 4 marketplace agents."
