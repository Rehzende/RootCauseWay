#!/bin/bash
set -e

echo "=== Adding Helm repos ==="
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

echo ""
echo "=== Creating monitoring namespace ==="
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "=== Installing Prometheus + AlertManager ==="
helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --set grafana.enabled=true \
  --set grafana.adminPassword=admin \
  --set alertmanager.enabled=true \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
  --wait --timeout 5m

echo ""
echo "=== Installing Loki ==="
helm upgrade --install loki grafana/loki \
  --namespace monitoring \
  --set loki.auth_enabled=false \
  --set loki.useTestSchema=true \
  --set loki.commonConfig.replication_factor=1 \
  --set singleBinary.replicas=1 \
  --set backend.replicas=0 \
  --set read.replicas=0 \
  --set write.replicas=0 \
  --set minio.enabled=false \
  --set loki.storage.type=filesystem \
  --set singleBinary.persistence.size=2Gi \
  --wait --timeout 5m

echo ""
echo "=== Installing Promtail (log collector) ==="
helm upgrade --install promtail grafana/promtail \
  --namespace monitoring \
  --set config.clients[0].url=http://loki:3100/loki/api/v1/push \
  --wait --timeout 3m

echo ""
echo "=== Installing Tempo (traces) ==="
helm upgrade --install tempo grafana/tempo \
  --namespace monitoring \
  --set tempo.storage.trace.backend=local \
  --set persistence.enabled=true \
  --set persistence.size=2Gi \
  --wait --timeout 5m

echo ""
echo "=== Observability stack installed ==="
echo ""
echo "Services:"
echo "  Prometheus:    kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090"
echo "  Grafana:       kubectl port-forward -n monitoring svc/prometheus-grafana 3001:80"
echo "  AlertManager:  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-alertmanager 9093:9093"
echo "  Loki:          kubectl port-forward -n monitoring svc/loki 3100:3100"
echo "  Tempo:         kubectl port-forward -n monitoring svc/tempo 3200:3200"
echo ""
echo "Grafana login: admin / admin"
