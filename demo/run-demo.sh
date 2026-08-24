#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "╔══════════════════════════════════════════════╗"
echo "║  RootCauseway Demo - Full E2E Validation             ║"
echo "╚══════════════════════════════════════════════╝"
echo ""

# Step 1: Install observability stack
echo "📦 Step 1: Installing observability stack..."
bash "$SCRIPT_DIR/setup-observability.sh"

# Step 2: Deploy sample app
echo ""
echo "🏪 Step 2: Deploying demo store app..."
kubectl apply -f "$SCRIPT_DIR/sample-app/deployment.yaml"
kubectl apply -f "$SCRIPT_DIR/sample-app/alerting-rules.yaml"

echo "Waiting for demo-store pods to be ready..."
kubectl wait --for=condition=ready pod -l app=store-api -n demo-store --timeout=120s
kubectl wait --for=condition=ready pod -l app=store-db -n demo-store --timeout=120s
kubectl wait --for=condition=ready pod -l app=payment-service -n demo-store --timeout=120s

echo ""
echo "✅ Demo store deployed:"
kubectl get pods -n demo-store

# Step 3: Register in RootCauseway
echo ""
echo "🔧 Step 3: Registering demo store in RootCauseway..."

# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@rootcauseway.local","password":"admin123!"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Register software
echo "  → Registering Store API..."
STORE_API=$(curl -s -X POST http://localhost:8080/api/v1/software \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Demo Store API",
    "slug": "demo-store-api",
    "description": "E-commerce store API with product catalog, cart, and checkout",
    "repository_url": "https://github.com/Rehzende/RootCauseway/demo-store-api",
    "pipeline_url": "https://github.com/Rehzende/RootCauseway/demo-store-api/actions",
    "cloud_provider": "on_prem",
    "cloud_resources": [
      {"type": "kubernetes", "name": "colima/demo-store", "region": "local"},
      {"type": "database", "name": "store-db", "engine": "postgresql"}
    ],
    "database_info": [
      {"type": "postgresql", "name": "store", "host": "store-db:5432", "engine": "PostgreSQL 16"}
    ],
    "stakeholders": [
      {"name": "Marcos Rezende", "email": "marcos@rootcauseway.local", "role": "Tech Lead"},
      {"name": "Bruno Braziel", "email": "bruno@rootcauseway.local", "role": "SRE"}
    ],
    "sre_team": [
      {"name": "Bruno Braziel", "email": "bruno@rootcauseway.local"},
      {"name": "Thiago Maeda", "email": "thiago@rootcauseway.local"}
    ],
    "architects": [
      {"name": "Gabriel Fraga", "email": "gabriel@rootcauseway.local"}
    ],
    "dependencies": ["store-cache", "payment-service"],
    "dashboard_url": "http://localhost:3001/d/demo-store",
    "tags": ["kubernetes", "postgresql", "redis", "e-commerce"]
  }')
STORE_API_ID=$(echo "$STORE_API" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "    Software ID: $STORE_API_ID"

# Register payment service
echo "  → Registering Payment Service..."
PAYMENT=$(curl -s -X POST http://localhost:8080/api/v1/software \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Demo Payment Service",
    "slug": "demo-payment-service",
    "description": "Payment processing service handling Stripe integration",
    "cloud_provider": "on_prem",
    "cloud_resources": [
      {"type": "kubernetes", "name": "colima/demo-store/payment", "region": "local"}
    ],
    "sre_team": [
      {"name": "Cesar Bonamigo", "email": "cesar@rootcauseway.local"}
    ],
    "dependencies": ["demo-store-api"],
    "tags": ["kubernetes", "payments", "stripe"]
  }')
PAYMENT_ID=$(echo "$PAYMENT" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "    Software ID: $PAYMENT_ID"

# Create webhook for Prometheus AlertManager
echo "  → Creating webhook for AlertManager..."
WEBHOOK=$(curl -s -X POST http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Prometheus AlertManager\",\"source\":\"prometheus_alertmanager\",\"software_id\":\"$STORE_API_ID\"}")
WEBHOOK_TOKEN=$(echo "$WEBHOOK" | python3 -c "import sys,json; print(json.load(sys.stdin)['endpoint_token'])")
echo "    Webhook Token: $WEBHOOK_TOKEN"

# Register observability data sources
echo "  → Registering Prometheus data source..."
curl -s -X POST http://localhost:8080/api/v1/observability/sources \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"Local Prometheus\",
    \"source_type\": \"prometheus\",
    \"base_url\": \"http://localhost:9090\",
    \"auth_type\": \"none\",
    \"capabilities\": [\"metrics\"],
    \"monitored_software_ids\": [\"$STORE_API_ID\", \"$PAYMENT_ID\"],
    \"description\": \"K8s cluster Prometheus\",
    \"environment\": \"demo\"
  }" > /dev/null

echo "  → Registering Loki data source..."
curl -s -X POST http://localhost:8080/api/v1/observability/sources \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"Local Loki\",
    \"source_type\": \"loki\",
    \"base_url\": \"http://localhost:3100\",
    \"auth_type\": \"none\",
    \"capabilities\": [\"logs\"],
    \"monitored_software_ids\": [\"$STORE_API_ID\", \"$PAYMENT_ID\"],
    \"description\": \"K8s cluster Loki\",
    \"environment\": \"demo\"
  }" > /dev/null

echo "  → Registering Tempo data source..."
curl -s -X POST http://localhost:8080/api/v1/observability/sources \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\": \"Local Tempo\",
    \"source_type\": \"tempo\",
    \"base_url\": \"http://localhost:3200\",
    \"auth_type\": \"none\",
    \"capabilities\": [\"traces\"],
    \"monitored_software_ids\": [\"$STORE_API_ID\"],
    \"description\": \"K8s cluster Tempo\",
    \"environment\": \"demo\"
  }" > /dev/null

# Step 4: Configure AlertManager to send to RootCauseway
echo ""
echo "🔔 Step 4: Configuring AlertManager webhook to RootCauseway..."
sed "s|WEBHOOK_TOKEN_PLACEHOLDER|$WEBHOOK_TOKEN|g" \
  "$SCRIPT_DIR/sample-app/alertmanager-config.yaml" | kubectl apply -f -

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║  ✅ Demo setup complete!                      ║"
echo "╚══════════════════════════════════════════════╝"
echo ""
echo "🌐 Access:"
echo "  RootCauseway Platform:  http://localhost:3000"
echo "  Grafana:        kubectl port-forward -n monitoring svc/prometheus-grafana 3001:80"
echo "  Prometheus:     kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090"
echo "  AlertManager:   kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-alertmanager 9093:9093"
echo ""
echo "🧪 To trigger an alert (simulate CPU spike):"
echo "  kubectl exec -n demo-store deploy/store-api -- sh -c 'dd if=/dev/zero of=/dev/null bs=1M &'"
echo ""
echo "🧪 To trigger OOM (payment service):"
echo "  kubectl exec -n demo-store deploy/payment-service -- sh -c 'tail /dev/zero'"
echo ""
echo "📊 Demo store registered in RootCauseway with:"
echo "  - Software: Demo Store API ($STORE_API_ID)"
echo "  - Software: Demo Payment Service ($PAYMENT_ID)"
echo "  - Webhook: Prometheus AlertManager → $WEBHOOK_TOKEN"
echo "  - Data Sources: Prometheus, Loki, Tempo"
echo ""
echo "Login: admin@rootcauseway.local / admin123!"
