# RootCauseway — Root Cause Analysis Intelligence

Plataforma que transforma alertas operacionais em investigação estruturada de incidentes.

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
