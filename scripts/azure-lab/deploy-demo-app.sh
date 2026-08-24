#!/usr/bin/env bash
# Builds and deploys the demo-app (FastAPI + Postgres, see demo-app/) to
# AKS: resets the Postgres admin password (never stored anywhere -- this
# app is the only thing meant to hold it, as a k8s Secret), builds the
# image on the homelab node and pushes it to the ACR (az acr build is
# blocked on this subscription, see provision.sh), then applies the k8s
# manifests with the real values substituted in.
#
# Requires: provision.sh already run. kubectl and docker not required
# locally -- this drives the homelab node over SSH for the build/push and
# uses the AKS admin kubeconfig (fetched fresh each run) for the deploy.

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

echo "== Resetting Postgres admin password (fresh credential for the app Secret) =="
PG_PASSWORD="$(openssl rand -base64 20 | tr -d '/+=' | cut -c1-24)"
az postgres flexible-server update --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_PG" --admin-password "$PG_PASSWORD" --output none

echo "== Building + pushing demo-app image on the homelab node =="
ACR_LOGIN_SERVER="${ROOTCAUSEWAY_LAB_ACR}.azurecr.io"
ACR_PASSWORD=$(az acr credential show -n "$ROOTCAUSEWAY_LAB_ACR" --query "passwords[0].value" -o tsv)
rsync -az --delete ./demo-app/ "${ROOTCAUSEWAY_LAB_HOMELAB_SSH}:/home/rehzende/rootcauseway-azure-demo-app/"
ssh "$ROOTCAUSEWAY_LAB_HOMELAB_SSH" "echo '$ACR_PASSWORD' | docker login ${ACR_LOGIN_SERVER} -u ${ROOTCAUSEWAY_LAB_ACR} --password-stdin \
  && cd /home/rehzende/rootcauseway-azure-demo-app \
  && docker build -t ${ACR_LOGIN_SERVER}/demo-app:latest . \
  && docker push ${ACR_LOGIN_SERVER}/demo-app:latest"

echo "== Fetching AKS admin kubeconfig =="
KCFG=$(mktemp)
trap 'rm -f "$KCFG"' EXIT
az aks get-credentials --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_AKS" --admin --overwrite-existing --file "$KCFG" --output none
export KUBECONFIG="$KCFG"

echo "== Applying k8s manifests =="
MANIFEST=$(mktemp)
trap 'rm -f "$KCFG" "$MANIFEST"' EXIT
sed -e "s|__DB_HOST__|${ROOTCAUSEWAY_LAB_PG}.postgres.database.azure.com|" \
    -e "s|__DB_USER__|${ROOTCAUSEWAY_LAB_PG_ADMIN_USER}|" \
    -e "s|__DB_PASSWORD__|${PG_PASSWORD}|" \
    -e "s|__ACR_LOGIN_SERVER__|${ACR_LOGIN_SERVER}|" \
    ./demo-app/k8s.yaml > "$MANIFEST"
kubectl apply -f "$MANIFEST"

echo "== Waiting for rollout =="
kubectl -n demo rollout status deployment/demo-app --timeout=120s

echo ""
echo "Done. Quick check:"
echo "  kubectl --kubeconfig <(az aks get-credentials --admin -g $ROOTCAUSEWAY_LAB_RG -n $ROOTCAUSEWAY_LAB_AKS -f -) -n demo get pods"
