# RootCauseway — Root Cause Analysis Intelligence

Plataforma que transforma alertas operacionais em investigação estruturada de incidentes.

## Screenshots

| | |
|---|---|
| ![Dashboard](docs/screenshots/dashboard.jpg) | ![Incident overview](docs/screenshots/incident-overview.jpg) |
| Dashboard — visão geral do cenário de incidentes | Incidente — contexto do software afetado, progresso da análise |
| ![RCA com 5 Whys](docs/screenshots/rca-analysis.jpg) | ![Integração Teams](docs/screenshots/teams-integration.jpg) |
| Root Cause Analysis — causa raiz, fatores contribuintes, 5 Whys | Settings > Integrations — conexão Teams para War Room |

## Quick Start

```bash
# Subir infraestrutura
make up

# Rodar testes
make test

# Desenvolvimento local
make db-up
make dev-backend   # Go API em :8080
make dev-agent     # Python agent service em :8081
make dev-frontend  # React em :5173
```

## Arquitetura

```
[Datadog/Prometheus/Grafana/OTel]
        ↓ webhook
   [Go Backend API]  ←→  [PostgreSQL]
        ↓ Redis pub/sub
   [Python Agent Service]
        ↓ Redis pub/sub
   [Go Backend API] → updates incident
        ↓
   [React Frontend] → incident cockpit
```

## Contratos

- API: `contracts/openapi/rootcauseway-api.yaml`
- Eventos Redis: `contracts/events/redis-events.yaml`
- Schema DB: `contracts/schemas/database.sql`
