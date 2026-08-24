# RootCauseway Installation Guide

## Prerequisites

- Kubernetes 1.25+ (for Helm deployment)
- Helm 3.0+
- PostgreSQL 15+ (or use bundled)
- Redis 7+ (or use bundled)
- Docker & Docker Compose (for development)

## Quick Start (Helm)

```bash
helm repo add rootcauseway https://charts.rootcauseway.io
helm install rootcauseway rootcauseway/rootcauseway \
  --set postgresql.auth.password=YOUR_DB_PASSWORD \
  --set secrets.jwtSecret=YOUR_JWT_SECRET \
  --set secrets.openaiApiKey=YOUR_OPENAI_KEY
```

## Docker Compose (Development)

```bash
git clone https://github.com/Rehzende/RootCauseway/rootcauseway.git
cd rootcauseway
cp .env.prod.example .env.prod
# Edit .env.prod with your settings
make prod-up
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Backend API port | `8080` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_NAME` | Database name | `rootcauseway` |
| `DB_USER` | Database user | `rootcauseway` |
| `DB_PASSWORD` | Database password | _(required)_ |
| `REDIS_URL` | Redis connection URL | `redis://localhost:6379` |
| `JWT_SECRET` | Secret for JWT signing | _(required)_ |
| `OPENAI_API_KEY` | OpenAI API key for agents | _(optional)_ |
| `ANTHROPIC_API_KEY` | Anthropic API key for agents | _(optional)_ |
| `BACKEND_API_URL` | Internal backend URL for agents | `http://backend:8080/api/v1` |

### LLM Configuration

RootCauseway supports multiple LLM providers:

- **OpenAI**: Set `OPENAI_API_KEY` environment variable
- **Anthropic**: Set `ANTHROPIC_API_KEY` environment variable
- **Azure OpenAI**: Set `AZURE_OPENAI_ENDPOINT` and `AZURE_OPENAI_API_KEY`
- **Local models (LM Studio)**: Set `LLM_BASE_URL` to your local endpoint (e.g., `http://localhost:1234/v1`)

Configure per-agent LLM settings via the Agent Marketplace or A2A agent configuration.

### SSO Setup

RootCauseway supports OIDC-based SSO. Configure via the API or admin UI:

**Keycloak:**
```json
{
  "name": "keycloak",
  "provider_type": "oidc",
  "config": {
    "issuer_url": "https://keycloak.example.com/realms/rootcauseway",
    "client_id": "rootcauseway",
    "client_secret": "your-client-secret"
  }
}
```

**Auth0:**
```json
{
  "name": "auth0",
  "provider_type": "oidc",
  "config": {
    "issuer_url": "https://your-tenant.auth0.com/",
    "client_id": "your-client-id",
    "client_secret": "your-client-secret"
  }
}
```

**Okta:**
```json
{
  "name": "okta",
  "provider_type": "oidc",
  "config": {
    "issuer_url": "https://your-org.okta.com",
    "client_id": "your-client-id",
    "client_secret": "your-client-secret"
  }
}
```

## Upgrading

### Helm

```bash
helm upgrade rootcauseway rootcauseway/rootcauseway
```

### Docker Compose

```bash
git pull
make prod-up
```

## Troubleshooting

### Database connection errors

Verify PostgreSQL is running and accessible:
```bash
kubectl logs -l app.kubernetes.io/component=backend | grep "database"
```

### Agent service not connecting

Ensure the backend is healthy before agents start:
```bash
kubectl get pods -l app.kubernetes.io/component=backend
curl http://<backend>:8080/health
```

### Redis connection issues

Check Redis connectivity:
```bash
kubectl exec -it <redis-pod> -- redis-cli ping
```

### Migrations not running

Check the migration job:
```bash
kubectl get jobs | grep migrations
kubectl logs job/<release>-migrations
```
