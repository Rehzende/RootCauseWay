# RootCauseway Architecture

## Overview

RootCauseway (Root Cause Analysis Intelligence) is a multi-agent platform for automated incident investigation and root cause analysis.

## Components

### Backend (Go)
The central API server built with Gin. Handles authentication, RBAC, incident management, agent coordination, and the marketplace. Exposes REST APIs and WebSocket connections.

### Agent Service / Orchestrator (Python)
Coordinates the multi-agent investigation pipeline. Receives incident events via Redis pub/sub, decides which agents to invoke, and manages the investigation DAG.

### Agents
Specialized AI agents that perform specific investigation tasks:

- **Triage Agent** - Initial incident classification, severity assessment, and routing
- **Evidence Agent** - Collects logs, metrics, traces, and change events related to the incident
- **RCA Agent** - Analyzes evidence to determine root cause using causal reasoning
- **Postmortem Agent** - Generates structured postmortem reports with action items

Each agent exposes an A2A (Agent-to-Agent) protocol endpoint and can be deployed independently.

### Frontend (React)
Dashboard for incident management, investigation cockpit, agent marketplace, runbooks, and analytics.

### Infrastructure
- **PostgreSQL** - Primary data store for incidents, agents, configurations
- **Redis** - Event pub/sub, caching, rate limiting

## Data Flow

```
Alert Source --> Webhook Ingestion --> Incident Created
                                        |
                                        v
                                   Redis Pub/Sub
                                        |
                                        v
                                   Orchestrator
                                   /    |    \
                                  v     v     v
                              Triage Evidence RCA --> Postmortem
                                  \     |     /
                                   v    v    v
                               Results stored in DB
                                        |
                                        v
                                   WebSocket --> Frontend Dashboard
```

## Agent Marketplace

The marketplace allows installing additional agents from a curated catalog. Each marketplace agent includes:
- Agent Card (A2A protocol metadata)
- Skills definition
- Docker image reference
- Configuration schema

When installed, a marketplace agent is registered as an A2A agent in the organization and made available to the orchestrator.
