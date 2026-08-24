# RootCauseway API Reference

## Interactive Documentation

Swagger UI is available at: `http://<backend-host>:8080/api/docs`

OpenAPI spec: `http://<backend-host>:8080/api/docs/openapi.yaml`

## Authentication

All protected endpoints require a Bearer token or API key:

```
Authorization: Bearer <jwt-token>
X-API-Key: <api-key>
```

## Key Endpoints

### Incidents
- `GET /api/v1/incidents` - List incidents
- `GET /api/v1/incidents/:id` - Get incident details
- `GET /api/v1/incidents/:id/full` - Full incident cockpit view
- `GET /api/v1/incidents/:id/dag` - Incident investigation DAG

### Alert Ingestion
- `POST /api/v1/ingest/:token` - Ingest alerts (public, token-authenticated)

### Agent Marketplace
- `GET /api/v1/marketplace` - Browse marketplace agents (?category=&search=)
- `GET /api/v1/marketplace/:slug` - Get agent details
- `POST /api/v1/marketplace/:slug/install` - Install an agent
- `GET /api/v1/marketplace/installed` - List installed agents
- `DELETE /api/v1/marketplace/installed/:id` - Uninstall an agent

### A2A Agents
- `GET /api/v1/a2a/agents` - List A2A agents
- `POST /api/v1/a2a/agents` - Register agent
- `GET /api/v1/a2a/agents/:id/card` - Get agent card

### Analytics
- `GET /api/v1/analytics/mttr` - Mean time to resolution
- `GET /api/v1/analytics/trends` - Incident trends
- `GET /api/v1/analytics/agent-effectiveness` - Agent performance

### Runbooks
- `GET /api/v1/runbooks` - List runbooks
- `POST /api/v1/runbooks/:id/execute` - Execute a runbook

### Knowledge Base
- `GET /api/v1/knowledge-base` - List entries
- `POST /api/v1/knowledge-base/search` - Search by error pattern

## WebSocket

Connect to `ws://<host>/ws` with JWT token for real-time incident updates.
