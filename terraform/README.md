# Terraform Provider for RootCauseway

Manage your RootCauseway software catalog, webhooks, agents, and skills as code.

## Installation

```hcl
terraform {
  required_providers {
    rootcauseway = {
      source = "rootcauseway/rootcauseway"
    }
  }
}

provider "rootcauseway" {
  api_url = "http://localhost:8080"
  token   = var.rootcauseway_token
}
```

## Resources

### `rootcauseway_software`

Register a software service in the RootCauseway catalog.

```hcl
resource "rootcauseway_software" "api_gateway" {
  name           = "API Gateway"
  slug           = "api-gateway"
  description    = "Main API gateway service"
  cloud_provider = "aws"
  tags           = ["kubernetes", "api"]
}
```

| Argument | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Display name |
| `slug` | string | yes | URL-safe identifier |
| `description` | string | no | Description |
| `cloud_provider` | string | no | `aws`, `gcp`, `azure` |
| `tags` | list(string) | no | Tags for filtering |

### `rootcauseway_webhook`

Register an alert webhook source.

```hcl
resource "rootcauseway_webhook" "prometheus" {
  name        = "Prometheus AlertManager"
  source      = "prometheus_alertmanager"
  software_id = rootcauseway_software.api_gateway.id
}
```

| Argument | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Display name |
| `source` | string | yes | `prometheus_alertmanager`, `datadog`, `grafana`, `otel` |
| `software_id` | string | yes | Associated software ID |

### `rootcauseway_agent`

Register a custom A2A agent.

```hcl
resource "rootcauseway_agent" "triage" {
  name         = "Custom Triage Agent"
  agent_type   = "triage"
  endpoint_url = "http://triage:8090"
  skills       = ["triage"]
}
```

| Argument | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Display name |
| `agent_type` | string | yes | `triage`, `evidence`, `rca`, `postmortem` |
| `endpoint_url` | string | yes | Agent A2A endpoint |
| `skills` | list(string) | no | Skill identifiers |

### `rootcauseway_skill`

Register a skill in the skills registry.

```hcl
resource "rootcauseway_skill" "kubernetes_logs" {
  name        = "Kubernetes Log Analysis"
  slug        = "k8s-logs"
  description = "Analyze Kubernetes pod logs for errors"
  category    = "evidence"
}
```

## Data Sources

### `data.rootcauseway_software`

Look up existing software by slug.

```hcl
data "rootcauseway_software" "existing" {
  slug = "api-gateway"
}
```

### `data.rootcauseway_agents`

List registered agents, optionally filtered by type.

```hcl
data "rootcauseway_agents" "triage_agents" {
  agent_type = "triage"
}
```

## Usage

```hcl
terraform {
  required_providers {
    rootcauseway = {
      source = "rootcauseway/rootcauseway"
    }
  }
}

provider "rootcauseway" {
  api_url = "http://localhost:8080"
  token   = var.rootcauseway_token
}

resource "rootcauseway_software" "api_gateway" {
  name        = "API Gateway"
  slug        = "api-gateway"
  description = "Main API gateway service"
  cloud_provider = "aws"
  tags = ["kubernetes", "api"]
}

resource "rootcauseway_webhook" "prometheus" {
  name        = "Prometheus AlertManager"
  source      = "prometheus_alertmanager"
  software_id = rootcauseway_software.api_gateway.id
}

resource "rootcauseway_agent" "triage" {
  name         = "Custom Triage Agent"
  agent_type   = "triage"
  endpoint_url = "http://triage:8090"
  skills       = ["triage"]
}
```
