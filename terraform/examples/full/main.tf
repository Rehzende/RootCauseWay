terraform {
  required_providers {
    rootcauseway = {
      source = "rootcauseway/rootcauseway"
    }
  }
}

variable "rootcauseway_token" {
  type      = string
  sensitive = true
}

variable "rootcauseway_api_url" {
  type    = string
  default = "http://localhost:8080"
}

provider "rootcauseway" {
  api_url = var.rootcauseway_api_url
  token   = var.rootcauseway_token
}

# ---------------------------------------------------------------------------
# Software Catalog
# ---------------------------------------------------------------------------

resource "rootcauseway_software" "api_gateway" {
  name           = "API Gateway"
  slug           = "api-gateway"
  description    = "Main API gateway service"
  cloud_provider = "aws"
  tags           = ["kubernetes", "api", "critical"]
}

resource "rootcauseway_software" "payments" {
  name           = "Payments Service"
  slug           = "payments"
  description    = "Payment processing microservice"
  cloud_provider = "aws"
  tags           = ["kubernetes", "payments", "critical"]
}

resource "rootcauseway_software" "notifications" {
  name           = "Notification Service"
  slug           = "notifications"
  description    = "Email and push notification service"
  cloud_provider = "gcp"
  tags           = ["kubernetes", "notifications"]
}

# ---------------------------------------------------------------------------
# Webhooks
# ---------------------------------------------------------------------------

resource "rootcauseway_webhook" "prometheus_gateway" {
  name        = "Prometheus - API Gateway"
  source      = "prometheus_alertmanager"
  software_id = rootcauseway_software.api_gateway.id
}

resource "rootcauseway_webhook" "datadog_payments" {
  name        = "Datadog - Payments"
  source      = "datadog"
  software_id = rootcauseway_software.payments.id
}

resource "rootcauseway_webhook" "grafana_notifications" {
  name        = "Grafana - Notifications"
  source      = "grafana"
  software_id = rootcauseway_software.notifications.id
}

# ---------------------------------------------------------------------------
# Custom Agents
# ---------------------------------------------------------------------------

resource "rootcauseway_agent" "triage" {
  name         = "Custom Triage Agent"
  agent_type   = "triage"
  endpoint_url = "http://triage-agent:8090"
  skills       = ["triage", "severity-classification"]
}

resource "rootcauseway_agent" "evidence" {
  name         = "Evidence Collector"
  agent_type   = "evidence"
  endpoint_url = "http://evidence-agent:8091"
  skills       = ["k8s-logs", "metrics-query", "trace-lookup"]
}

resource "rootcauseway_agent" "rca" {
  name         = "Root Cause Analyzer"
  agent_type   = "rca"
  endpoint_url = "http://rca-agent:8092"
  skills       = ["five-whys", "fault-tree", "timeline-correlation"]
}

resource "rootcauseway_agent" "postmortem" {
  name         = "Postmortem Writer"
  agent_type   = "postmortem"
  endpoint_url = "http://postmortem-agent:8093"
  skills       = ["postmortem-generation"]
}

# ---------------------------------------------------------------------------
# Skills
# ---------------------------------------------------------------------------

resource "rootcauseway_skill" "k8s_logs" {
  name        = "Kubernetes Log Analysis"
  slug        = "k8s-logs"
  description = "Analyze Kubernetes pod logs for errors and anomalies"
  category    = "evidence"
}

resource "rootcauseway_skill" "metrics_query" {
  name        = "Metrics Query"
  slug        = "metrics-query"
  description = "Query Prometheus/Datadog metrics for anomaly detection"
  category    = "evidence"
}

resource "rootcauseway_skill" "five_whys" {
  name        = "Five Whys Analysis"
  slug        = "five-whys"
  description = "Structured 5 Whys root cause analysis methodology"
  category    = "rca"
}

# ---------------------------------------------------------------------------
# Data Sources
# ---------------------------------------------------------------------------

data "rootcauseway_software" "existing_gateway" {
  slug = "api-gateway"

  depends_on = [rootcauseway_software.api_gateway]
}

data "rootcauseway_agents" "all_triage" {
  agent_type = "triage"

  depends_on = [rootcauseway_agent.triage]
}

# ---------------------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------------------

output "api_gateway_id" {
  value = rootcauseway_software.api_gateway.id
}

output "webhook_urls" {
  value = {
    prometheus = rootcauseway_webhook.prometheus_gateway.id
    datadog    = rootcauseway_webhook.datadog_payments.id
    grafana    = rootcauseway_webhook.grafana_notifications.id
  }
}

output "triage_agents" {
  value = data.rootcauseway_agents.all_triage
}
