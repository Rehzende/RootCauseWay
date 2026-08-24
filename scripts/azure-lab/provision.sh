#!/usr/bin/env bash
# Provisions the real-Azure validation lab: VNet/subnet/NSG, AKS (AAD +
# Azure RBAC for Kubernetes enabled -- required for the dynamic JIT token
# design, see project_backlog.md), Storage Account, Key Vault, Postgres
# Flexible Server, an ACR (for deploy-demo-app.sh), a service principal
# scoped to AKS for the JIT flow (its client_secret lands in Key Vault,
# never printed to a file), and a Chaos Studio workspace scoped to the
# resource group. Run deploy-demo-app.sh and deploy-metrics-agent.sh
# after this to get an actual workload + metrics flowing -- this script
# only stands up the Azure resources themselves.
#
# Idempotent-ish: every `az ... create` is safe to re-run (Azure no-ops or
# updates in place), so a failed run partway through can just be re-run.
#
# Requires: az cli logged in (`az login`), an active subscription selected
# (`az account set --subscription ...`).
#
# Cost note: this creates real billable resources. AKS (1 node --
# ROOTCAUSEWAY_LAB_AKS_NODE_SIZE, see config.sh; defaults to Standard_D2s_v6, NOT
# a burstable B-series -- some subscriptions don't have B-series quota in
# every region, check `az vm list-skus` if create fails on VM size) +
# Postgres Flexible Server (Burstable B1ms) are the two ongoing cost
# drivers if left running -- use stop.sh between test sessions (see that
# script) rather than leaving this up 24/7. ACR Basic tier is a few
# dollars/month, negligible but not zero.

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

echo "== Registering required resource providers (idempotent, no-ops if already registered) =="
for ns in Microsoft.ContainerService Microsoft.Storage Microsoft.KeyVault \
          Microsoft.DBforPostgreSQL Microsoft.Chaos Microsoft.ContainerRegistry \
          Microsoft.Insights Microsoft.AlertsManagement Microsoft.Monitor Microsoft.Network; do
  az provider register --namespace "$ns" --wait &
done
wait

echo "== Resource group: $ROOTCAUSEWAY_LAB_RG ($ROOTCAUSEWAY_LAB_LOCATION) =="
az group create --name "$ROOTCAUSEWAY_LAB_RG" --location "$ROOTCAUSEWAY_LAB_LOCATION" --output none

echo "== Network: VNet/subnet/NSG =="
az network vnet create \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" \
  --name "$ROOTCAUSEWAY_LAB_VNET" \
  --address-prefix 10.20.0.0/16 \
  --subnet-name "$ROOTCAUSEWAY_LAB_SUBNET" \
  --subnet-prefix 10.20.1.0/24 \
  --output none

az network nsg create \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" \
  --name "$ROOTCAUSEWAY_LAB_NSG" \
  --output none

# Baseline allow-everything-outbound rule at a priority the network-fault
# scenario can later override with a lower-priority Deny (NSGs evaluate
# lowest-priority-number first) -- see chaos-fault-network.sh.
az network nsg rule create \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" \
  --nsg-name "$ROOTCAUSEWAY_LAB_NSG" \
  --name allow-outbound-baseline \
  --priority 2000 \
  --direction Outbound \
  --access Allow \
  --protocol '*' \
  --source-address-prefixes '*' \
  --destination-address-prefixes '*' \
  --destination-port-ranges '*' \
  --output none

az network vnet subnet update \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" \
  --vnet-name "$ROOTCAUSEWAY_LAB_VNET" \
  --name "$ROOTCAUSEWAY_LAB_SUBNET" \
  --network-security-group "$ROOTCAUSEWAY_LAB_NSG" \
  --output none

echo "== AKS: $ROOTCAUSEWAY_LAB_AKS (AAD + Azure RBAC for Kubernetes enabled) =="
SUBNET_ID=$(az network vnet subnet show \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" --vnet-name "$ROOTCAUSEWAY_LAB_VNET" --name "$ROOTCAUSEWAY_LAB_SUBNET" \
  --query id -o tsv)

# Idempotent: unlike `az group/storage account/keyvault create`, `az aks
# create` errors outright if the cluster already exists (no create-or-
# update semantics) -- check first rather than letting a re-run fail here.
if az aks show --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_AKS" --output none 2>/dev/null; then
  echo "   (already exists, skipping create)"
else
  az aks create \
    --resource-group "$ROOTCAUSEWAY_LAB_RG" \
    --name "$ROOTCAUSEWAY_LAB_AKS" \
    --node-count "$ROOTCAUSEWAY_LAB_AKS_NODE_COUNT" \
    --node-vm-size "$ROOTCAUSEWAY_LAB_AKS_NODE_SIZE" \
    --network-plugin azure \
    --vnet-subnet-id "$SUBNET_ID" \
    --enable-aad \
    --enable-azure-rbac \
    --generate-ssh-keys \
    --output none
fi

AKS_ID=$(az aks show --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_AKS" --query id -o tsv)

echo "== Storage account: $ROOTCAUSEWAY_LAB_STORAGE =="
az storage account create \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" \
  --name "$ROOTCAUSEWAY_LAB_STORAGE" \
  --sku Standard_LRS \
  --kind StorageV2 \
  --output none

echo "== Key Vault: $ROOTCAUSEWAY_LAB_KEYVAULT =="
# Idempotent: `az keyvault create` errors outright if the vault already
# exists (even soft-deleted-and-purgeable ones can collide) -- check first.
if az keyvault show --name "$ROOTCAUSEWAY_LAB_KEYVAULT" --output none 2>/dev/null; then
  echo "   (already exists, skipping create)"
else
  az keyvault create \
    --resource-group "$ROOTCAUSEWAY_LAB_RG" \
    --name "$ROOTCAUSEWAY_LAB_KEYVAULT" \
    --enable-rbac-authorization true \
    --output none
fi

echo "== Container Registry: $ROOTCAUSEWAY_LAB_ACR =="
# Holds the demo-app image (see deploy-demo-app.sh) -- attached to AKS so
# the kubelet identity gets AcrPull automatically, no imagePullSecrets.
if az acr show --name "$ROOTCAUSEWAY_LAB_ACR" --output none 2>/dev/null; then
  echo "   (already exists, skipping create)"
else
  az acr create --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_ACR" --sku Basic --output none
fi
az aks update --name "$ROOTCAUSEWAY_LAB_AKS" --resource-group "$ROOTCAUSEWAY_LAB_RG" --attach-acr "$ROOTCAUSEWAY_LAB_ACR" --output none
# Admin credentials: `az acr build` (cloud-side build) is blocked on this
# subscription (TasksOperationsNotAllowed), so deploy-demo-app.sh builds
# on the homelab node and `docker push`es instead -- needs a username/
# password to `docker login` with, not just the AKS kubelet's AcrPull
# role (that only covers pulls, not the push from an external Docker).
az acr update --name "$ROOTCAUSEWAY_LAB_ACR" --admin-enabled true --output none

echo "== Postgres Flexible Server: $ROOTCAUSEWAY_LAB_PG =="
# Idempotent: like AKS, `az postgres flexible-server create` errors
# outright if the server already exists -- check first.
if az postgres flexible-server show --resource-group "$ROOTCAUSEWAY_LAB_RG" --name "$ROOTCAUSEWAY_LAB_PG" --output none 2>/dev/null; then
  echo "   (already exists, skipping create)"
else
  PG_ADMIN_PASSWORD="$(openssl rand -base64 24)"
  az postgres flexible-server create \
    --resource-group "$ROOTCAUSEWAY_LAB_RG" \
    --name "$ROOTCAUSEWAY_LAB_PG" \
    --location "$ROOTCAUSEWAY_LAB_LOCATION" \
    --sku-name "$ROOTCAUSEWAY_LAB_PG_SKU" \
    --tier Burstable \
    --storage-size 32 \
    --admin-user "$ROOTCAUSEWAY_LAB_PG_ADMIN_USER" \
    --admin-password "$PG_ADMIN_PASSWORD" \
    --public-access 0.0.0.0 \
    --yes \
    --output none
  echo "   (admin password generated, not stored anywhere -- Postgres is a"
  echo "    network-reachability-only target in this lab, no DB credential"
  echo "    is meant to be used by RootCauseway. Save it yourself now if you want"
  echo "    to log in manually later: $PG_ADMIN_PASSWORD)"
fi

echo "== Service principal for AKS JIT: $ROOTCAUSEWAY_LAB_SP_NAME =="
# Least-privilege read role, scoped to just this cluster -- this is the
# identity RootCauseway's dynamic-token AKS provider (see project_backlog.md)
# will use at lease-request time to mint a short-lived Azure AD token.
# Idempotent: `az ad sp create-for-rbac` would create a SECOND app with
# the same display name on a re-run (display names aren't unique in
# Entra ID) -- if one already exists, reset its credential instead of
# creating a duplicate.
EXISTING_APP_ID=$(az ad sp list --display-name "$ROOTCAUSEWAY_LAB_SP_NAME" --query "[0].appId" -o tsv)
SP_TENANT=$(az account show --query tenantId -o tsv)
if [ -n "$EXISTING_APP_ID" ]; then
  echo "   (already exists, resetting its credential for a fresh secret)"
  SP_APP_ID="$EXISTING_APP_ID"
  SP_SECRET=$(az ad app credential reset --id "$SP_APP_ID" --query password -o tsv)
else
  SP_JSON=$(az ad sp create-for-rbac --name "$ROOTCAUSEWAY_LAB_SP_NAME" --skip-assignment --output json)
  SP_APP_ID=$(echo "$SP_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["appId"])')
  SP_SECRET=$(echo "$SP_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["password"])')
fi

# Idempotent: `az role assignment create` errors on an exact duplicate
# assignment, so check first rather than letting a re-run fail here.
if ! az role assignment list --assignee "$SP_APP_ID" --scope "$AKS_ID" --query "[?roleDefinitionName=='Azure Kubernetes Service RBAC Reader']" -o tsv | grep -q .; then
  az role assignment create \
    --role "Azure Kubernetes Service RBAC Reader" \
    --assignee "$SP_APP_ID" \
    --scope "$AKS_ID" \
    --output none
else
  echo "   (role assignment already exists)"
fi

echo "== Storing the SP's Key Vault secrets (RBAC, not access policy) =="
# You (the person running this script) need Key Vault RBAC rights of your
# own the first time, since --enable-rbac-authorization means the classic
# access-policy model is off. Grant yourself Secrets Officer if this fails:
#   az role assignment create --role "Key Vault Secrets Officer" \
#     --assignee $(az ad signed-in-user show --query id -o tsv) \
#     --scope $(az keyvault show -n "$ROOTCAUSEWAY_LAB_KEYVAULT" --query id -o tsv)
az keyvault secret set --vault-name "$ROOTCAUSEWAY_LAB_KEYVAULT" --name "aks-jit-tenant-id" --value "$SP_TENANT" --output none
az keyvault secret set --vault-name "$ROOTCAUSEWAY_LAB_KEYVAULT" --name "aks-jit-client-id" --value "$SP_APP_ID" --output none
az keyvault secret set --vault-name "$ROOTCAUSEWAY_LAB_KEYVAULT" --name "aks-jit-client-secret" --value "$SP_SECRET" --output none

echo "== Chaos Studio: registering provider + bootstrapping workspace =="
az provider register --namespace Microsoft.Chaos --wait
az extension add --name chaos --upgrade --yes 2>/dev/null || az extension add --name chaos --yes

RG_ID=$(az group show --name "$ROOTCAUSEWAY_LAB_RG" --query id -o tsv)
az chaos setup \
  --name "$ROOTCAUSEWAY_LAB_CHAOS_WORKSPACE" \
  --resource-group "$ROOTCAUSEWAY_LAB_RG" \
  --location "$ROOTCAUSEWAY_LAB_CHAOS_LOCATION" \
  --scopes "$RG_ID" \
  --output none

cat <<EOF

== Provisioning done ==

Resource group: $ROOTCAUSEWAY_LAB_RG
AKS:            $ROOTCAUSEWAY_LAB_AKS
Storage:        $ROOTCAUSEWAY_LAB_STORAGE
Key Vault:      $ROOTCAUSEWAY_LAB_KEYVAULT
Postgres:       $ROOTCAUSEWAY_LAB_PG
Chaos Studio:   $ROOTCAUSEWAY_LAB_CHAOS_WORKSPACE

Next steps (manual, not yet scripted -- see project_backlog.md):
1. Install Chaos Mesh on the AKS cluster (needed before any AKS fault can
   run):
     az aks get-credentials --admin --resource-group $ROOTCAUSEWAY_LAB_RG --name $ROOTCAUSEWAY_LAB_AKS
     helm repo add chaos-mesh https://charts.chaos-mesh.org && helm repo update
     kubectl create ns chaos-testing
     helm install chaos-mesh chaos-mesh/chaos-mesh --namespace=chaos-testing \\
       --set chaosDaemon.runtime=containerd --set chaosDaemon.socketPath=/run/containerd/containerd.sock

2. See what Chaos Studio auto-discovered for this resource group:
     az chaos workspace show-discovery --resource-group $ROOTCAUSEWAY_LAB_RG --workspace-name $ROOTCAUSEWAY_LAB_CHAOS_WORKSPACE

3. Cost control between test sessions: run stop.sh / start.sh (not 24/7).
4. Full teardown when done for good: run teardown.sh.
EOF
