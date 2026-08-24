#!/usr/bin/env bash
# Provisions a dedicated, least-privilege service principal for the
# azure-agent A2A microservice (agents/azure-agent/) -- the counterpart to
# k8s-agent's kubectl access, but for Azure-native evidence: AKS
# control-plane events (Activity Log), Key Vault access audit, and NSG
# rules for the lab's network.
#
# Deliberately a SEPARATE identity from $ROOTCAUSEWAY_LAB_SP_NAME (the AKS-JIT SP):
# that one is scoped narrowly to minting AKS RBAC tokens at lease-request
# time (identity/access), this one is read-only diagnostics across three
# unrelated Azure services. Mixing them would mean the JIT SP -- which
# exists specifically to be a tightly-scoped, single-purpose credential --
# quietly accumulates unrelated read permissions over time.
#
# Roles granted, all built-in, all read-only, all scoped to just this
# resource group:
#   - Reader                -- generic resource metadata (NSG rules, AKS/
#                               Storage/Postgres resource properties)
#   - Monitoring Reader      -- Activity Log + metrics (AKS control-plane
#                               events, diagnostic settings)
#   - Key Vault Reader       -- vault properties + diagnostic settings only.
#                               Does NOT grant read access to keys, secrets,
#                               or certificates -- azure-agent can see that
#                               someone accessed a secret, never the secret
#                               itself.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
source ./config.sh

: "${ROOTCAUSEWAY_LAB_AZURE_AGENT_SP_NAME:=rcai-azure-agent-sp}"
export ROOTCAUSEWAY_LAB_AZURE_AGENT_SP_NAME

echo "== Service principal for azure-agent: $ROOTCAUSEWAY_LAB_AZURE_AGENT_SP_NAME =="
EXISTING_APP_ID=$(az ad sp list --display-name "$ROOTCAUSEWAY_LAB_AZURE_AGENT_SP_NAME" --query "[0].appId" -o tsv)
SP_TENANT=$(az account show --query tenantId -o tsv)
if [ -n "$EXISTING_APP_ID" ]; then
  echo "   (already exists, resetting its credential for a fresh secret)"
  SP_APP_ID="$EXISTING_APP_ID"
  SP_SECRET=$(az ad app credential reset --id "$SP_APP_ID" --query password -o tsv)
else
  SP_JSON=$(az ad sp create-for-rbac --name "$ROOTCAUSEWAY_LAB_AZURE_AGENT_SP_NAME" --skip-assignment --output json)
  SP_APP_ID=$(echo "$SP_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["appId"])')
  SP_SECRET=$(echo "$SP_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["password"])')
fi

RG_ID=$(az group show --name "$ROOTCAUSEWAY_LAB_RG" --query id -o tsv)

for role in Reader "Monitoring Reader" "Key Vault Reader"; do
  if ! az role assignment list --assignee "$SP_APP_ID" --scope "$RG_ID" --query "[?roleDefinitionName=='$role']" -o tsv | grep -q .; then
    echo "== Granting '$role' at resource group scope =="
    az role assignment create --role "$role" --assignee "$SP_APP_ID" --scope "$RG_ID" --output none
  else
    echo "   ('$role' already assigned)"
  fi
done

echo "== Storing the SP's credentials in Key Vault (RBAC, not access policy) =="
# Same self-grant note as provision.sh: you need Key Vault Secrets Officer
# on yourself the first time this runs, if it wasn't already granted there.
az keyvault secret set --vault-name "$ROOTCAUSEWAY_LAB_KEYVAULT" --name "azure-agent-tenant-id" --value "$SP_TENANT" --output none
az keyvault secret set --vault-name "$ROOTCAUSEWAY_LAB_KEYVAULT" --name "azure-agent-client-id" --value "$SP_APP_ID" --output none
az keyvault secret set --vault-name "$ROOTCAUSEWAY_LAB_KEYVAULT" --name "azure-agent-client-secret" --value "$SP_SECRET" --output none

echo "== Done =="
echo "SP app id: $SP_APP_ID"
echo "Tenant id: $SP_TENANT"
echo "Secret stored in Key Vault '$ROOTCAUSEWAY_LAB_KEYVAULT' as azure-agent-{tenant-id,client-id,client-secret}."
echo "Register these in RootCauseway as an azure_key_vault credential provider (backend/internal/services/azure_keyvault_provider.go)"
echo "so azure-agent's Azure SDK calls go through the JIT vault instead of a static env var."
