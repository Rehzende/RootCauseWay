#!/usr/bin/env bash
# Resumes AKS + Postgres before a test session. Takes a few minutes for
# both to come back Ready/Available.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

echo "== Starting AKS cluster $ROOTCAUSEWAY_LAB_AKS =="
az aks start --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_AKS"

echo "== Starting Postgres Flexible Server $ROOTCAUSEWAY_LAB_PG =="
az postgres flexible-server start --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_PG"

echo "Done. Both should be usable within a few minutes."
