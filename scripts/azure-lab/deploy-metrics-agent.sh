#!/usr/bin/env bash
# Wires up AKS -> homelab Prometheus metrics flow:
# 1. Ensures the Cloudflare quick tunnel (cloudflared-quick-tunnel.yaml)
#    is running on the homelab k3s, exposing its Prometheus's
#    remote-write receiver to the public internet. Needed because the
#    homelab is behind real ISP-level CGNAT (100.64.0.0/10) -- port
#    forwarding cannot work here, see project_backlog.md for the full
#    diagnosis. This is a *quick* tunnel (no Cloudflare account/domain
#    needed) so its https://*.trycloudflare.com hostname changes every
#    time the tunnel pod restarts -- this script always re-reads it fresh
#    from the pod's logs rather than hardcoding it anywhere.
# 2. Deploys Prometheus in Agent mode (metrics-agent.yaml) into AKS,
#    remote_write-ing to whatever that hostname currently is.
#
# Requires: provision.sh and deploy-demo-app.sh already run. SSH access to
# the homelab node (ROOTCAUSEWAY_LAB_HOMELAB_SSH in config.sh) for step 1.

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

echo "== Ensuring the Cloudflare tunnel is running on the homelab k3s =="
scp -q ./cloudflared-quick-tunnel.yaml "${ROOTCAUSEWAY_LAB_HOMELAB_SSH}:/home/rehzende/cloudflared-quick-tunnel.yaml"
ssh "$ROOTCAUSEWAY_LAB_HOMELAB_SSH" "kubectl apply -f /home/rehzende/cloudflared-quick-tunnel.yaml"
ssh "$ROOTCAUSEWAY_LAB_HOMELAB_SSH" "kubectl -n monitoring rollout status deployment/cloudflared-prometheus-tunnel --timeout=60s"

echo "== Reading the current tunnel hostname from its logs =="
TUNNEL_URL=""
for _ in $(seq 1 15); do
  TUNNEL_URL=$(ssh "$ROOTCAUSEWAY_LAB_HOMELAB_SSH" "kubectl -n monitoring logs deploy/cloudflared-prometheus-tunnel --tail=50" \
    | grep -oE 'https://[a-zA-Z0-9-]+\.trycloudflare\.com' | head -1 || true)
  [ -n "$TUNNEL_URL" ] && break
  sleep 2
done
if [ -z "$TUNNEL_URL" ]; then
  echo "Could not find the tunnel URL in cloudflared's logs -- check it manually:" >&2
  echo "  ssh $ROOTCAUSEWAY_LAB_HOMELAB_SSH \"kubectl -n monitoring logs deploy/cloudflared-prometheus-tunnel\"" >&2
  exit 1
fi
REMOTE_WRITE_URL="${TUNNEL_URL}/api/v1/write"
echo "   Tunnel: $TUNNEL_URL"

echo "== Fetching AKS admin kubeconfig =="
KCFG=$(mktemp)
trap 'rm -f "$KCFG"' EXIT
az aks get-credentials --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_AKS" --admin --overwrite-existing --file "$KCFG" --output none
export KUBECONFIG="$KCFG"

echo "== Deploying Prometheus Agent to AKS, remote_write -> $REMOTE_WRITE_URL =="
MANIFEST=$(mktemp)
trap 'rm -f "$KCFG" "$MANIFEST"' EXIT
sed "s|__REMOTE_WRITE_URL__|${REMOTE_WRITE_URL}|" ./metrics-agent.yaml > "$MANIFEST"
kubectl apply -f "$MANIFEST"
kubectl -n metrics-agent rollout status deployment/prometheus-agent --timeout=60s

echo ""
echo "Done. Verify data actually landed (run on the homelab node):"
echo "  kubectl exec -n monitoring deploy/monitoring-kube-prometheus-operator -- \\"
echo "    wget -qO- 'http://${ROOTCAUSEWAY_LAB_HOMELAB_PROMETHEUS_SVC}/api/v1/query?query=up%7Bcluster%3D%22${ROOTCAUSEWAY_LAB_AKS}%22%7D'"
echo ""
echo "Reminder: this URL changes if the tunnel pod ever restarts -- re-run"
echo "this script to pick up the new one and redeploy the agent."
