#!/usr/bin/env bash
# Pauses the two ongoing cost drivers between test sessions -- AKS nodes
# and the Postgres Flexible Server -- without deleting anything. Storage
# Account and Key Vault cost is negligible at rest, left running.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

echo "== Stopping AKS cluster $ROOTCAUSEWAY_LAB_AKS (deallocates nodes, keeps config) =="
az aks stop --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_AKS"

echo "== Stopping Postgres Flexible Server $ROOTCAUSEWAY_LAB_PG (storage still bills) =="
az postgres flexible-server stop --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_PG"

echo "Done. Run start.sh before your next test session."
