# RootCauseway — Root Cause Analysis Intelligence
## Product Feature Reference

> Complete inventory of all modules, features, endpoints, and capabilities.

---

## Table of Contents

1. [Platform Overview](#1-platform-overview)
2. [Alert Ingestion & Normalization](#2-alert-ingestion--normalization)
3. [Incident Management](#3-incident-management)
4. [AI Investigation Pipeline](#4-ai-investigation-pipeline)
5. [Agent Framework (A2A)](#5-agent-framework-a2a)
6. [Skills Registry](#6-skills-registry)
7. [Credentials & JIT Access](#7-credentials--jit-access)
8. [Runbooks](#8-runbooks)
9. [Knowledge Base & Feedback Loop](#9-knowledge-base--feedback-loop)
10. [Observability Sources](#10-observability-sources)
11. [Change Events](#11-change-events)
12. [Notifications & Escalation, War Room (Teams)](#12-notifications--escalation)
13. [Analytics](#13-analytics)
14. [Agent Marketplace](#14-agent-marketplace)
15. [Software Catalog](#15-software-catalog)
16. [Authentication & Authorization](#16-authentication--authorization)
17. [Real-Time & WebSocket](#17-real-time--websocket)
18. [Audit Log](#18-audit-log)
19. [CLI Tool](#19-cli-tool)
20. [Frontend Pages & UI](#20-frontend-pages--ui)
21. [API Reference Summary](#21-api-reference-summary)
22. [Infrastructure & Deployment](#22-infrastructure--deployment)

---

## 1. Platform Overview

RootCauseway is a multi-tenant AI-powered incident management platform that automates the full investigation lifecycle — from alert ingestion to blameless postmortem — using a pipeline of specialized AI agents.

### Architecture

| Layer | Technology | Port |
|---|---|---|
| Frontend | React + Vite + TailwindCSS | 3000 (dev) / 80 (prod) |
| Backend API | Go (Gin) | 8080 |
| Agent Orchestrator | Python (FastAPI) | 8081 |
| Triage Agent | Python (A2A microservice) | 8090 |
| Evidence Agent | Python (A2A microservice) | 8091 |
| RCA Agent | Python (A2A microservice) | 8092 |
| Postmortem Agent | Python (A2A microservice) | 8093 |
| Database | PostgreSQL 17 | 5432 |
| Message Broker | Redis | 6379 |

### Key Design Principles

- **Event-driven**: Redis pub/sub decouples ingestion from orchestration
- **Agent-to-Agent (A2A) Protocol**: Google A2A standard for inter-agent communication
- **Multi-tenant**: Full organization-level isolation
- **Contract-first**: OpenAPI + Redis event schemas defined in `/contracts/`
- **Hybrid hosting**: Managed (RootCauseway hosts agents) + BYOA (customer-hosted agents)

---

## 2. Alert Ingestion & Normalization

### Webhook Ingestion

Each software resource gets a unique ingest token. Alerts are posted to:

```
POST /api/v1/ingest/:token
```

**Supported sources:**
- Datadog
- Prometheus AlertManager
- Grafana
- OpenTelemetry Collector
- Custom (any JSON payload)

### Alert Normalization

Incoming payloads are normalized into a source-agnostic `NormalizedAlert` structure:

| Field | Description |
|---|---|
| `title` | Alert title |
| `description` | Full description |
| `severity` | critical / high / medium / low |
| `source` | Originating system |
| `labels` | Key-value tags |
| `annotations` | Extended metadata |
| `fingerprint` | Deduplication hash |
| `starts_at` | Alert start time |
| `software_id` | Linked software entry |

### Alert Quarantine

Alerts that cannot be matched to any registered software are placed in quarantine instead of being silently dropped.

- **GET** `/api/v1/quarantine` — list quarantined alerts
- **POST** `/api/v1/quarantine/:id/resolve` — resolve or dismiss

Dedicated **QuarantinePage** in the UI shows unmatched alerts with context for manual triage.

### Alert Snapshot

At ingestion time, the platform captures an `AlertSnapshot` — a point-in-time copy of the raw payload, preserved for forensic use during evidence analysis.

---

## 3. Incident Management

### Incident Lifecycle

```
open → investigating → mitigated → resolved → closed
```

### Incident Fields

| Field | Description |
|---|---|
| `title` | Incident title |
| `description` | Full description |
| `severity` | critical / high / medium / low |
| `status` | Lifecycle state |
| `assignee_id` | Responsible user |
| `software_id` | Affected software |
| `root_cause` | Summary root cause |
| `mitigation` | Mitigation steps taken |
| `resolution` | Final resolution |
| `resolved_at` | Resolution timestamp |

### Incident Events (Timeline)

Every action on an incident is recorded as a timestamped event:

| Event Type | Description |
|---|---|
| `alert_received` | Alert triggered the incident |
| `triage_started` / `triage_completed` | Triage skill dispatched / completed |
| `evidence_collected` | Evidence-collection skill completed |
| `hypothesis_generated` | Hypothesis produced by the RCA agent |
| `rci_started` / `rci_completed` | RCI dispatched / confirmed persisted |
| `rca_started` / `rca_completed` | RCA dispatched / confirmed persisted |
| `postmortem_started` / `postmortem_completed` | Postmortem dispatched / confirmed persisted |
| `agent_run_started` / `agent_run_completed` / `agent_run_failed` | Generic per-skill dispatch signal — fires for every skill the orchestrator calls, including ones with no dedicated type above (`k8s-debug`, `azure-*`, custom skills) |
| `correlated_alert` | A duplicate alert was correlated into this incident instead of opening a new one |
| `war_room_created` | A Teams war room meeting was created for this incident |
| `status_changed` | Lifecycle state change |
| `comment` | Human comment |
| `agent_action` | AI agent action |

The `_started`/`_completed` pairs only emit for a call the orchestrator
actually dispatches; `_completed` fires once the artifact is confirmed
persisted, not just because the underlying A2A call returned successfully.

### Evidence Collection

Evidence is typed and traceable:

| Evidence Type | Examples |
|---|---|
| `log` | Log lines, log file exports |
| `metric` | Time-series data, charts |
| `trace` | Distributed trace |
| `snapshot` | Automated observability snapshot |
| `agent_output` | Output produced by an AI agent |
| `manual` | Human-submitted notes |
| `screenshot` | Dashboard screenshots |
| `dashboard` | Dashboard link/embed |
| `config` | Configuration files |
| `heap_dump` | Memory dumps |

Evidence upload supports file attachments with blob storage integration (`POST /api/v1/incidents/:id/evidence/upload`).

### Incident Cockpit

`GET /api/v1/incidents/:id/full` returns the complete enriched view of an incident, including:
- Incident metadata
- All events (timeline)
- All evidence
- Agent runs (DAG)
- RCI, RCA, and Postmortem results
- Orchestrator decisions

### Agent Run DAG

Each investigation phase is tracked as a Directed Acyclic Graph (DAG):

- `GET /api/v1/incidents/:id/dag` — get the full investigation DAG
- `GET /api/v1/incidents/:id/runs` — list all agent runs
- `POST /api/v1/incidents/:id/runs/:runId/rerun` — rerun a failed agent

Each `AgentRun` node records: agent used, status, input/output, model, tokens consumed, duration, and error messages.

---

## 4. AI Investigation Pipeline

There is no fixed stage count or order. On `alert.received`, the orchestrator
builds the incident's context (affected software, similar past incidents,
evidence collected so far) and asks an LLM, on every iteration, which skill
to call next and whether another one is needed — each call's result feeds
the context for the next decision. Two incidents of the same alert type can
dispatch different skill sets: a Kubernetes-labeled incident pulls in
`k8s-debug`/`k8s-logs`; a cloud-hosted one pulls in `azure-*` skills instead.
None of this is hardcoded into a sequence.

```
Alert Received
        │
        ▼
┌───────────────────────────────────────────────────────────────────┐
│  ORCHESTRATOR — an LLM decides the next skill to call, given the   │
│  context accumulated so far (repeats until it decides it has       │
│  enough, or an iteration limit is hit)                              │
└────────────────────────────┬──────────────────────────────────────┘
                             │ each call's result feeds the next decision
                             ▼
        (example skills, called 0–N times each, in whatever
         order the LLM decides)
┌─────────┐ ┌──────────┐ ┌─────┐ ┌────────────┐
│ Triage  │ │ Evidence │ │ RCA │ │ Postmortem │  ...and others
└─────────┘ └──────────┘ └─────┘ └────────────┘  (k8s-debug, azure-*, ...)
```

The RCA agent (port 8092) is the one that produces hypothesis, RCI and RCA
together in a single call — not three separate stages — the orchestrator
just extracts and persists each artifact from that one response.

### Triage

**Output: `IncidentRCI` triage section**

- Severity assessment (critical / high / medium / low)
- Category classification: infrastructure / application / database / network / security
- Affected component identification
- Confidence score

### Evidence

**Output: Evidence collection plan**

- Recommended data sources
- Log queries with rationale
- Evidence priority: high / medium / low
- Types of evidence to collect

### RCA agent — hypothesis + Root Cause Investigation (RCI) + Root Cause Analysis (RCA)

A single call to this agent returns all three artifacts together:

- **Hypothesis**: structured root cause hypothesis + recommended investigation actions.
- **RCI** (`IncidentRCI`, below).
- **RCA** (`IncidentRCA`, next section).

**`IncidentRCI` fields:**

| Field | Description |
|---|---|
| `investigation_summary` | Narrative description of the investigation |
| `impact_assessment` | Business and technical impact |
| `affected_services` | List of impacted services |
| `affected_users_estimate` | Estimated user impact |
| `detection_method` | How the issue was detected |
| `time_to_detect` (TTD) | Time between start and detection |
| `acknowledgment_time` | Time to acknowledge |
| `evidence_ids` | Referenced evidence artifacts |

**`IncidentRCA` fields:**

| Field | Description |
|---|---|
| `root_cause_summary` | Concise root cause statement |
| `root_cause_category` | infrastructure / code / configuration / dependency / human_error / capacity / security |
| `contributing_factors` | List of contributing causes |
| `five_whys` | Structured five-whys breakdown |
| `confidence_score` | AI confidence (0–1) |
| `evidence_references` | Supporting evidence |

### Postmortem

**Agent:** Postmortem Agent (port 8093). Runs automatically when the incident
is resolved, not as part of the skill-selection loop above (or paused for
human approval first — see the HITL gate note under Notifications below).

**Output: `IncidentPostmortem`**

| Field | Description |
|---|---|
| `title` | Postmortem title |
| `executive_summary` | High-level summary for stakeholders |
| `incident_timeline` | Narrative timeline of events |
| `root_cause_detail` | Detailed root cause explanation |
| `impact_detail` | Full impact description |
| `lessons_learned` | Key takeaways |
| `what_went_well` | Positive outcomes |
| `what_went_wrong` | Areas of failure |
| `action_items` | Prioritized follow-up items with assignees |
| `prevention_measures` | Steps to prevent recurrence |
| `published` | Publication status |

### Orchestrator Decisions

The intelligent orchestrator logs every routing decision:

- Decision type and reasoning
- Selected agents
- Context used for the decision
- Confidence level

`GET /api/v1/incidents/:id/orchestrator/decisions`

---

## 5. Agent Framework (A2A)

### Google A2A Protocol

All agents implement the [Google Agent-to-Agent (A2A) Protocol](https://google.github.io/A2A/), enabling:
- Standardized task dispatch
- Structured input/output artifacts
- Agent capability advertisement via **Agent Cards**
- Health checking

### Agent Card

Each agent publishes a capability card describing:
- Agent name, description, version
- Supported task types
- Input/output specifications
- Required skills
- Resource requirements
- Authentication requirements

`GET /api/v1/a2a/agents/:id/card`

### A2A Task Lifecycle

```
submitted → running → completed / failed / cancelled
```

Each `A2ATask` records:
- Task type and priority
- Input message (structured)
- Output artifacts
- Orchestrator reasoning
- Dependencies on other tasks
- Full timing (submitted, started, completed)

### Agent Management

| Endpoint | Description |
|---|---|
| `GET /api/v1/a2a/agents` | List all registered A2A agents |
| `POST /api/v1/a2a/agents` | Register new agent |
| `GET /api/v1/a2a/agents/:id/card` | Get agent capability card |
| `POST /api/v1/a2a/agents/:id/health-check` | Health check single agent |
| `POST /api/v1/a2a/agents/health-check-all` | Health check all agents |

### Supported Auth Types for Agents

- Bearer token
- API key
- mTLS (mutual TLS)

### Hybrid Hosting Model

| Mode | Description |
|---|---|
| **Managed** | RootCauseway hosts the agent container; customer provides API keys |
| **BYOA** | Customer hosts their own agent; registers endpoint URL with RootCauseway |

Both modes are credential-routed through the JIT vault, so agents never hold long-lived secrets.

---

## 6. Skills Registry

Skills are reusable, composable capabilities that agents can use.

### Skill Definition

| Field | Description |
|---|---|
| `name` | Human-readable name |
| `slug` | Unique identifier |
| `category` | See categories below |
| `prompt_template` | Base prompt template |
| `input_schema` | Expected input JSON schema |
| `output_schema` | Expected output JSON schema |
| `required_tools` | Tool integrations needed |
| `resource_types` | What resource types this skill operates on |
| `permissions` | Required permissions |

### Skill Categories

- `infrastructure`
- `application`
- `database`
- `network`
- `security`
- `cloud`
- `observability`
- `custom`

### Agent-Skill Mapping

Skills can be linked to multiple agents. Each agent can override skill configuration.

| Endpoint | Description |
|---|---|
| `GET /api/v1/skills` | List all skills |
| `GET /api/v1/skills/:id/agents` | List agents using a skill |
| `POST /api/v1/a2a/agents/:id/skills` | Link skill to agent |
| `DELETE /api/v1/a2a/agents/:id/skills/:skillId` | Unlink skill |

---

## 7. Credentials & JIT Access

### Credential Providers

Supports pluggable credential backends:

| Provider | Description |
|---|---|
| `hashicorp_vault` | HashiCorp Vault dynamic secrets |
| `aws_sts` | AWS Security Token Service |
| `azure_managed_identity` | Azure Managed Identity |
| `gcp_workload_identity` | GCP Workload Identity Federation |
| `static` | Static secrets (dev/fallback) |
| `custom` | Custom provider integration |

### Resource Credentials

Credentials are scoped to software resources:

| Resource Type | Description |
|---|---|
| `kubernetes_cluster` | Cluster access |
| `database` | DB credentials |
| `cloud_account` | Cloud provider account |
| `api_endpoint` | External API key |
| `storage` | Object/block storage |
| `message_queue` | Queue system access |
| `cache` | Cache system access |
| `custom` | Custom resource |

### JIT Credential Leasing

Agents request short-lived credentials on demand:

1. Agent requests lease via `POST /api/v1/credentials/leases/request`
2. Platform validates against access policy
3. Credential is issued with TTL
4. Lease is recorded in audit trail
5. Lease revokable via `POST /api/v1/credentials/leases/:id/revoke`

### Access Policies

Policies control which agents/skills can access which resources:

- Target: agent / skill / agent_type
- Allowed actions: list of permitted operations
- Scope restrictions: environment, region, etc.
- TTL: maximum lease duration

---

## 8. Runbooks

### Runbook Definition

| Field | Description |
|---|---|
| `name` | Runbook name |
| `slug` | Unique identifier |
| `description` | Purpose and usage |
| `trigger_conditions` | When to use this runbook |
| `auto_trigger` | Whether to auto-execute on matching incidents |

### Step Types

| Type | Description |
|---|---|
| `manual` | Human must complete this step |
| `automated` | Executed automatically by an agent/skill |
| `approval` | Requires human approval before proceeding |
| `notification` | Sends a notification |
| `condition` | Branching logic |

Each step supports: timeout, failure handling strategy, retry count, and skill linkage.

### Runbook Execution

Executions track:
- Current step
- Step-by-step results
- Who triggered it (human or auto)
- Timing per step

`POST /api/v1/runbooks/:id/execute` — start execution  
`POST /api/v1/runbook-executions/:execId/steps/:stepId/complete` — complete manual step

---

## 9. Knowledge Base & Feedback Loop

### Knowledge Base

Persistent, searchable knowledge built from resolved incidents:

| Field | Description |
|---|---|
| `category` | Knowledge category |
| `error_pattern` | Pattern that triggers lookup |
| `root_cause_summary` | Known root cause |
| `resolution` | How to resolve |
| `lessons_learned` | Key lessons |
| `action_items` | Standard action items |
| `human_validated` | Whether validated by a human |
| `confidence` | Confidence score |
| `reference_count` | How often referenced |

`POST /api/v1/knowledge-base/search` — semantic/keyword search

### Human Feedback

After analysis, users can submit feedback on agent output:

| Field | Description |
|---|---|
| `target_type` | rci / rca / postmortem / triage / evidence |
| `rating` | positive / negative / neutral |
| `original_data` | What the agent produced |
| `corrected_data` | What the human corrected it to |

Feedback is used to improve future analysis and build the knowledge base.

### Similar Incident Matching

- `GET /api/v1/incidents/:id/similar` — list similar past incidents
- Similarity score + match criteria stored per link
- Used during evidence and RCA phases to reference known patterns

### Correlation Rules

Rules that group related alerts into a single incident:

- `GET /api/v1/correlation-rules` — list rules
- `POST /api/v1/correlation-rules` — create rule
- `GET /api/v1/incidents/:id/alert-groups` — view grouped alerts

---

## 10. Observability Sources

### Supported Sources

- Prometheus
- Datadog
- Grafana
- OpenTelemetry
- Custom

### Source Management

| Endpoint | Description |
|---|---|
| `POST /api/v1/observability/sources/:id/health` | Check source connectivity |
| `GET /api/v1/software/:id/observability` | Get observability config for a service |

### Evidence Snapshot Configs

Snapshot configurations define what data to automatically collect when an incident fires on a software resource:

- Metric queries to run
- Log queries to execute
- Dashboard screenshots to capture
- Trace queries

`POST /api/v1/observability/sources/:id/snapshots` — create snapshot config

---

## 11. Change Events

Tracks deployments and infrastructure changes to correlate with incidents:

| Field | Description |
|---|---|
| `software_id` | Affected software |
| `change_type` | deployment / config / infra / rollback |
| `description` | What changed |
| `author` | Who made the change |
| `timestamp` | When it happened |
| `metadata` | Additional context (commit SHA, pipeline ID, etc.) |

`GET /api/v1/software/:id/changes` — list changes for a service

---

## 12. Notifications & Escalation

### Notification Channels

| Channel Type | Description |
|---|---|
| `slack` | Slack webhook/bot integration |
| `teams` | Microsoft Teams |
| `pagerduty` | PagerDuty alert |
| `email` | SMTP email |
| `webhook` | Generic HTTP webhook |
| `custom` | Custom integration |

### Escalation Policies

- Severity filter (only escalate if severity ≥ threshold)
- Multi-step escalation chains
- Repeat interval (re-notify if not acknowledged)
- Linked to notification channels

### Notification Audit

Every notification is logged:
- Channel used
- Policy triggered
- Event type
- Recipient
- Status (sent / failed)
- Error message (if failed)
- Delivery timestamp

### War Room (Microsoft Teams)

One click on an incident creates a real Microsoft Teams meeting — not a
mocked link, a genuine calendar event with a joinable Teams call attached,
created via delegated OAuth against a connected service/bot Microsoft
account.

**`POST /api/v1/incidents/:id/warroom`** — creates the meeting and:

- **Invites the right people automatically.** The affected software's
  `stakeholders` + `sre_team` (Software Catalog, see below) are resolved
  into a deduplicated attendee list and added to the Teams invite — nobody
  has to go find the join link inside the incident.
- **Notifies configured channels.** `war_room_created` fires through the
  same `notifications`/escalation-policy path as `incident_created` and
  `rca_completed` — Slack/Teams/webhook/PagerDuty channels already
  configured for it just work, no separate setup.
- **Enables recording + transcription** on the meeting automatically
  (`recordAutomatically`/`allowTranscription`), so a transcript is
  available when the meeting ends without anyone remembering to start it.
- **Tracks a real lifecycle**: `scheduled` → `active` → `ended` →
  `summarized`. `active` is set the first time attendance data shows up
  for a still-`scheduled` meeting (checked opportunistically on read, not
  a dedicated poller) — a real signal instead of a status nobody ever set.

**`POST /api/v1/warroom/:id/end`** — marks the meeting ended, fetches the
transcript + attendance report from Graph, and publishes
`warroom.meeting.ended`; a Redis-stream consumer then summarizes the
transcript with an LLM (executive summary, key action items, participant
list) and writes it back — async, so ending the meeting in the UI doesn't
block on summarization.

**`POST /api/v1/organizations/:id/integrations/teams/oauth/disconnect`** —
clears the connected account (refresh token + connected email) without
touching the saved Azure AD app registration (tenant/client/secret), so
reconnecting doesn't require re-entering those.

---

## 13. Analytics

### Available Metrics

| Endpoint | Metric |
|---|---|
| `GET /api/v1/analytics/mttr` | Mean Time To Recovery |
| `GET /api/v1/analytics/trends` | Incident volume trends over time |
| `GET /api/v1/analytics/agent-effectiveness` | Agent accuracy and performance |
| `GET /api/v1/analytics/cost-by-model` | LLM token cost broken down by model |
| `GET /api/v1/analytics/cost-by-incident` | LLM cost per incident |

### MTTR Breakdown

- Total MTTR
- Time to Detect (TTD)
- Time to Acknowledge (TTA)
- Time to Mitigate (TTM)
- Time to Resolve (TTR)
- Breakdown by severity, software, time period

### Agent Effectiveness

- Analysis accuracy (via feedback ratings)
- Confidence score distributions
- False positive rate
- Time saved per investigation

---

## 14. Agent Marketplace

A catalog of pre-built agents that organizations can install and configure.

### Marketplace Agent Fields

| Field | Description |
|---|---|
| `name` | Agent name |
| `slug` | Unique identifier |
| `description` | What the agent does |
| `author` | Publisher |
| `version` | Semantic version |
| `category` | Agent category |
| `docker_image` | Container image |
| `agent_card` | A2A capability card |
| `skills` | Bundled skills |
| `required_credentials` | Credential types needed |
| `config_schema` | Configuration JSON schema |
| `rating` | Community rating |
| `verified` | RootCauseway-verified badge |
| `download_count` | Install count |

### Marketplace Operations

| Endpoint | Description |
|---|---|
| `GET /api/v1/marketplace` | Browse catalog |
| `GET /api/v1/marketplace/:slug` | Agent details |
| `POST /api/v1/marketplace/:slug/install` | Install agent |
| `GET /api/v1/marketplace/installed` | List installed agents |
| `DELETE /api/v1/marketplace/installed/:id` | Uninstall |

---

## 15. Software Catalog

The software catalog is the foundation for context-aware incident analysis.

### Software Entry Fields

| Field | Description |
|---|---|
| `name` / `slug` | Service identifier |
| `description` | What the service does |
| `owner` | Owning team or person |
| `repository_url` | Source code repo |
| `pipeline_url` | CI/CD pipeline |
| `cloud_provider` | aws / azure / gcp / on_prem |
| `cloud_resources` | Cloud resource IDs/ARNs |
| `database_info` | Database details |
| `infra_details` | Infrastructure context |
| `stakeholders` | Contacts for incidents |
| `sre_team` | Responsible SRE team |
| `architects` | System architects |
| `runbook_url` | Link to runbook |
| `dashboard_url` | Monitoring dashboard link |
| `dependencies` | Upstream/downstream services |

This context is injected into agent prompts during every investigation.

---

## 16. Authentication & Authorization

### Authentication Methods

| Method | Description |
|---|---|
| Local | Username + password |
| SSO — OIDC | Generic OpenID Connect |
| SSO — Google | Google Workspace |
| SSO — GitHub | GitHub OAuth |
| SSO — Azure AD | Microsoft Entra ID |
| SSO — Okta | Okta OIDC |
| SSO — SAML | Generic SAML 2.0 |
| API Key | Programmatic access with scoping |

### RBAC — Roles & Permissions

- Custom roles with granular permissions
- Permissions are resource + action combinations (e.g., `incidents:write`, `agents:delete`)
- Users can have multiple roles
- `PermissionGate` component enforces RBAC in the UI

### API Keys

- Scoped to specific resources or actions
- Revocable at any time
- Used for integrations and CLI access

### Session Management

- JWT-based sessions
- Session tracking with IP and user-agent
- Auto-provisioning of SSO users

---

## 17. Real-Time & WebSocket

### WebSocket Endpoint

```
GET /ws
```

Real-time events pushed to connected clients:
- Incident status changes
- Agent run progress
- New evidence added
- Pipeline stage completions

### Redis Pub/Sub Events

| Event | Description |
|---|---|
| `alert.received` | New alert ingested |
| `triage.completed` | Triage stage done |
| `evidence.collected` | Evidence artifact added |
| `hypothesis.generated` | Hypothesis produced |
| `agent.status` | Agent lifecycle event |

Multi-instance safe: Redis bridge ensures all backend instances can deliver WebSocket messages to any connected client.

---

## 18. Audit Log

Every state-changing action in the platform is recorded:

| Field | Description |
|---|---|
| `actor_id` | User or system that performed the action |
| `action` | Action performed |
| `resource_type` | What was affected |
| `resource_id` | Specific resource ID |
| `changes` | Before/after diff |
| `ip_address` | Originating IP |
| `user_agent` | Client info |
| `timestamp` | When it happened |

`GET /api/v1/audit-log` — queryable audit trail

---

## 19. CLI Tool

The `rootcauseway` CLI provides programmatic access to all platform features.

### Commands

| Command | Description |
|---|---|
| `rootcauseway config` | Manage CLI configuration |
| `rootcauseway auth` | Login, logout, manage API keys |
| `rootcauseway agents` | List, create, manage agents |
| `rootcauseway software` | Manage software catalog |
| `rootcauseway incidents` | List and inspect incidents |
| `rootcauseway analytics` | Query analytics metrics |

All commands support `--json` flag for machine-readable output.

---

## 20. Frontend Pages & UI

### Pages

| Page | Route | Description |
|---|---|---|
| Login | `/login` | Authentication entry point |
| Dashboard | `/` | Main overview |
| Incidents | `/incidents` | Incident list with filters |
| Incident Detail | `/incidents/:id` | Full incident cockpit |
| Software | `/software` | Service catalog |
| Agents | `/agents` | Agent registry |
| Skills | `/skills` | Skills management |
| Marketplace | `/marketplace` | Agent marketplace |
| Webhooks | `/webhooks` | Webhook configuration |
| Credentials | `/credentials` | Credential management |
| Data Sources | `/data-sources` | Observability sources |
| Runbooks | `/runbooks` | Runbook library |
| Analytics | `/analytics` | Metrics & dashboards |
| Notifications | `/notifications` | Notification config |
| Quarantine | `/quarantine` | Unmatched alert triage |
| Users | `/users` | User management |
| Roles | `/roles` | RBAC configuration |
| Audit Log | `/audit-log` | Audit trail |
| Settings | `/settings` | System configuration |
| Onboarding | `/onboarding` | Setup wizard |

### Key UI Components

| Component | Description |
|---|---|
| `RCAPanel` | Root Cause Analysis results display |
| `RCIPanel` | Investigation summary panel |
| `PostmortemView` | Postmortem viewer and editor |
| `EvidencePanel` | Evidence list and detail |
| `EvidenceUpload` | File upload for manual evidence |
| `IncidentTimeline` | Chronological event timeline |
| `RunsTimeline` | Agent execution DAG visualization |
| `FiveWhys` | Five-whys breakdown viewer |
| `OrchestratorDecisions` | Orchestration decision log |
| `ConfidenceMeter` | Visual confidence indicator |
| `PresenceIndicator` | Real-time user presence |
| `SeverityBadge` | Color-coded severity indicator |
| `StatusBadge` | Lifecycle status indicator |
| `PermissionGate` | RBAC-aware conditional rendering |
| `DataTable` | Sortable, filterable data grid |

### Real-Time Hooks

- `useWebSocket` — subscribes to live incident updates
- `useAuth` — session and permission context
- `useToastMutation` — toast notifications for API mutations

---

## 21. API Reference Summary

The full API is documented via OpenAPI at `/api/docs`.

### Endpoint Count by Domain

| Domain | Endpoints |
|---|---|
| Authentication | 8 |
| Incidents (CRUD + events) | 12 |
| Incident Analysis (RCI/RCA/Postmortem) | 9 |
| Agent Runs & DAG | 4 |
| A2A Agents | 8 |
| A2A Tasks | 4 |
| Skills | 7 |
| Credentials & Access | 14 |
| Runbooks & Executions | 14 |
| Notifications & Escalation | 11 |
| Analytics | 5 |
| Marketplace | 5 |
| Software Catalog | 5 |
| Observability Sources | 9 |
| Knowledge Base | 6 |
| Correlation Rules | 5 |
| Change Events | 3 |
| Users | 7 |
| Roles & Permissions | 7 |
| SSO Providers | 4 |
| Audit Log | 1 |
| Quarantine | 2 |
| Webhooks | 4 |
| Alert Ingestion | 1 |
| WebSocket | 1 |
| **Total** | **~166** |

---

## 22. Infrastructure & Deployment

### Development

```bash
make up              # Start full dev stack (docker compose)
make dev-backend     # Go API with hot reload
make dev-agent       # Python orchestrator with hot reload
make dev-frontend    # Vite dev server
```

### Production

```bash
cp .env.prod.example .env.prod
make prod-build      # Build all images
make prod-up         # Start production stack
make prod-logs       # Tail all logs
```

### Database Operations

```bash
make db-migrate      # Apply pending migrations
make db-rollback     # Rollback last migration
make db-status       # Show migration state
```

### Schema Migrations (14 files)

| Migration | Feature Added |
|---|---|
| 001 | Core schema (orgs, users, software, incidents, webhooks) |
| 002 | Incident cockpit (agent runs, RCI, RCA, postmortem) |
| 003 | A2A protocol + enriched software catalog |
| 004 | Skills registry + credential JIT vault |
| 005 | Feedback loop + knowledge base + similar incidents |
| 006 | Correlation rules + alert grouping |
| 007 | Notifications + escalation policies |
| 008 | Runbooks + execution tracking |
| 009 | Change events |
| 010 | Auth + RBAC + SSO + API keys + audit log |
| 011 | Agent marketplace |
| 012 | Observability sources + snapshot configs |
| 013 | Alert quarantine |
| 014 | Agent hosting model |

### Testing

```bash
make test            # All tests
make test-backend    # Go tests (go test ./... -v)
make test-agent      # Python tests (pytest)
make test-frontend   # React tests (vitest)
make lint            # golangci-lint + eslint
make ci              # Full CI pipeline locally
```

### Kubernetes

Terraform configurations in `/terraform/` for Kubernetes deployment.

### API Security Middleware

| Middleware | Description |
|---|---|
| `RequestID` | Unique request ID per request |
| `StructuredLogger` | JSON structured logging |
| `SecurityHeaders` | CORS, CSP, and security headers |
| `RateLimiter` | 100 req/min global, 20/min per user |
| `UnifiedAuthMiddleware` | JWT + API key validation |
| `AuditMiddleware` | Automatic audit trail |
| `Recovery` | Panic recovery |

---

*Generated: 2026-06-30 | Version tracked in git history*
