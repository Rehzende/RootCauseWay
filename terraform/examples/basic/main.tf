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

provider "rootcauseway" {
  api_url = "http://localhost:8080"
  token   = var.rootcauseway_token
}

# Register a software service
resource "rootcauseway_software" "api_gateway" {
  name           = "API Gateway"
  slug           = "api-gateway"
  description    = "Main API gateway service"
  cloud_provider = "aws"
  tags           = ["kubernetes", "api"]
}

# Set up a Prometheus webhook for this service
resource "rootcauseway_webhook" "prometheus" {
  name        = "Prometheus AlertManager"
  source      = "prometheus_alertmanager"
  software_id = rootcauseway_software.api_gateway.id
}
