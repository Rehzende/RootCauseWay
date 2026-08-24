#!/usr/bin/env bash
# Shared config for the scripts/azure-lab/*.sh scripts -- source this, don't
# run it directly. Every name/size is overridable via env var so you can
# re-run against a second region/subscription without editing the files.
#
# Purpose of this whole resource group: a real-Azure validation lab for
# RootCauseway's RCA quality + JIT credential vault, covering Network/AKS/Storage/
# Key Vault/Postgres. See project_backlog.md ("Projeto grande em
# planejamento: validação em Azure real") for the full design writeup this
# implements.

set -euo pipefail

: "${ROOTCAUSEWAY_LAB_LOCATION:=brazilsouth}"
: "${ROOTCAUSEWAY_LAB_RG:=rootcauseway-azure-lab}"
: "${ROOTCAUSEWAY_LAB_VNET:=rootcauseway-lab-vnet}"
: "${ROOTCAUSEWAY_LAB_SUBNET:=rootcauseway-lab-aks-subnet}"
: "${ROOTCAUSEWAY_LAB_NSG:=rootcauseway-lab-nsg}"
: "${ROOTCAUSEWAY_LAB_AKS:=rootcauseway-lab-aks}"
: "${ROOTCAUSEWAY_LAB_AKS_NODE_COUNT:=1}"
: "${ROOTCAUSEWAY_LAB_AKS_NODE_SIZE:=Standard_D2s_v6}"
: "${ROOTCAUSEWAY_LAB_SP_NAME:=rootcauseway-lab-aks-jit-sp}"
: "${ROOTCAUSEWAY_LAB_CHAOS_WORKSPACE:=rootcauseway-lab-chaos}"
# Chaos Studio Workspaces is only available in a short list of regions
# (eastus2, westus2, northeurope, swedencentral, uksouth, japaneast,
# westcentralus, switzerlandnorth as of this writing -- brazilsouth is
# NOT one of them). The workspace can still target resources living in
# any region via --scopes, so only the workspace itself needs to live
# here, everything else stays in ROOTCAUSEWAY_LAB_LOCATION.
: "${ROOTCAUSEWAY_LAB_CHAOS_LOCATION:=eastus2}"

# Storage account and Key Vault names must be globally unique across all of
# Azure (not just your subscription) and have tight length/charset limits
# (storage: 3-24 lowercase alphanumeric only; Key Vault: 3-24 alphanumeric
# + hyphens). A stable suffix derived from the subscription ID keeps names
# deterministic across re-runs of this script without colliding with
# anyone else's resources, and short enough to fit both limits.
_sub_id="$(az account show --query id -o tsv)"
_suffix="$(echo -n "$_sub_id" | shasum -a 256 | cut -c1-8)"

: "${ROOTCAUSEWAY_LAB_STORAGE:=rootcausewaylab${_suffix}}"
: "${ROOTCAUSEWAY_LAB_KEYVAULT:=rootcauseway-lab-kv-${_suffix}}"
: "${ROOTCAUSEWAY_LAB_PG:=rootcauseway-lab-pg-${_suffix}}"
: "${ROOTCAUSEWAY_LAB_PG_ADMIN_USER:=rootcausewayadmin}"
: "${ROOTCAUSEWAY_LAB_PG_SKU:=Standard_B1ms}"
: "${ROOTCAUSEWAY_LAB_ACR:=rootcausewaylabacr${_suffix}}"
# Homelab node used to build/push images into the ACR -- `az acr build`
# (cloud-side build) hits TasksOperationsNotAllowed on new/low-usage
# subscriptions, so this script builds locally on the node and `docker
# push`es instead (push is outbound, unaffected by the node's CGNAT).
: "${ROOTCAUSEWAY_LAB_HOMELAB_SSH:=rehzende@192.168.68.110}"
: "${ROOTCAUSEWAY_LAB_HOMELAB_PROMETHEUS_SVC:=monitoring-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090}"

export ROOTCAUSEWAY_LAB_LOCATION ROOTCAUSEWAY_LAB_RG ROOTCAUSEWAY_LAB_VNET ROOTCAUSEWAY_LAB_SUBNET ROOTCAUSEWAY_LAB_NSG \
  ROOTCAUSEWAY_LAB_AKS ROOTCAUSEWAY_LAB_AKS_NODE_COUNT ROOTCAUSEWAY_LAB_AKS_NODE_SIZE ROOTCAUSEWAY_LAB_SP_NAME \
  ROOTCAUSEWAY_LAB_CHAOS_WORKSPACE ROOTCAUSEWAY_LAB_CHAOS_LOCATION ROOTCAUSEWAY_LAB_STORAGE ROOTCAUSEWAY_LAB_KEYVAULT \
  ROOTCAUSEWAY_LAB_PG ROOTCAUSEWAY_LAB_PG_ADMIN_USER ROOTCAUSEWAY_LAB_PG_SKU ROOTCAUSEWAY_LAB_ACR ROOTCAUSEWAY_LAB_HOMELAB_SSH \
  ROOTCAUSEWAY_LAB_HOMELAB_PROMETHEUS_SVC
