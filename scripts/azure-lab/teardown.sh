#!/usr/bin/env bash
# Deletes everything provision.sh created. The resource group delete
# covers every resource inside it (AKS, Storage, Key Vault, Postgres, ACR,
# VNet/NSG, Chaos Studio workspace) in one shot, run in the background
# (--no-wait) since it can take several minutes -- this also takes down
# the demo-app and the metrics-agent, since both live inside that AKS
# cluster, nothing separate to clean up for those. The service principal
# is a tenant-level Azure AD object, NOT inside the resource group, so it
# needs deleting separately or it lingers forever.
#
# NOT covered by this script: the Cloudflare tunnel (cloudflared) running
# on the HOMELAB k3s (deploy-metrics-agent.sh put it there, not here) --
# that's a different cluster entirely. Clean it up separately if wanted:
#   ssh $ROOTCAUSEWAY_LAB_HOMELAB_SSH "kubectl delete -f /home/rehzende/cloudflared-quick-tunnel.yaml"
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

read -r -p "This deletes resource group '$ROOTCAUSEWAY_LAB_RG' and its service principal '$ROOTCAUSEWAY_LAB_SP_NAME' for real. Type the resource group name to confirm: " CONFIRM
if [ "$CONFIRM" != "$ROOTCAUSEWAY_LAB_RG" ]; then
  echo "Confirmation didn't match, aborting. Nothing was deleted."
  exit 1
fi

echo "== Deleting service principal $ROOTCAUSEWAY_LAB_SP_NAME =="
SP_APP_ID=$(az ad sp list --display-name "$ROOTCAUSEWAY_LAB_SP_NAME" --query "[0].appId" -o tsv)
if [ -n "$SP_APP_ID" ]; then
  az ad sp delete --id "$SP_APP_ID"
else
  echo "   (not found, already gone)"
fi

echo "== Deleting resource group $ROOTCAUSEWAY_LAB_RG (backgrounded, takes a few minutes) =="
az group delete --name "$ROOTCAUSEWAY_LAB_RG" --yes --no-wait

echo "Delete started. Check progress with: az group show --name $ROOTCAUSEWAY_LAB_RG (errors out once it's gone)."
