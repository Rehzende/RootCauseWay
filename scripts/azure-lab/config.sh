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
# NOTE (2026-08-24): the resource-name defaults below are pinned to the
# actual Azure resources already deployed under the "rcai" prefix, from
# before the RootCauseway rebrand -- Azure resource names are immutable
# (renaming means recreating), so these stay "rcai-*" on purpose while
# every variable NAME in this file uses the ROOTCAUSEWAY_LAB_ prefix for
# consistency with the rest of the rebranded codebase. A blind rename pass
# once flipped these VALUES to "rootcauseway-*" too, which pointed every
# script here at resources that don't exist -- confirmed via `az resource
# list -g rcai-azure-lab`, every real name still says "rcai". Don't
# "fix" these back to rootcauseway-* without actually recreating the
# underlying Azure resources first.
: "${ROOTCAUSEWAY_LAB_RG:=rcai-azure-lab}"
: "${ROOTCAUSEWAY_LAB_VNET:=rcai-lab-vnet}"
: "${ROOTCAUSEWAY_LAB_SUBNET:=rcai-lab-aks-subnet}"
: "${ROOTCAUSEWAY_LAB_NSG:=rcai-lab-nsg}"
: "${ROOTCAUSEWAY_LAB_AKS:=rcai-lab-aks}"
: "${ROOTCAUSEWAY_LAB_AKS_NODE_COUNT:=1}"
: "${ROOTCAUSEWAY_LAB_AKS_NODE_SIZE:=Standard_D2s_v6}"
: "${ROOTCAUSEWAY_LAB_SP_NAME:=rcai-lab-aks-jit-sp}"
: "${ROOTCAUSEWAY_LAB_CHAOS_WORKSPACE:=rcai-lab-chaos}"
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

: "${ROOTCAUSEWAY_LAB_STORAGE:=rcailab${_suffix}}"
: "${ROOTCAUSEWAY_LAB_KEYVAULT:=rcai-lab-kv-${_suffix}}"
: "${ROOTCAUSEWAY_LAB_PG:=rcai-lab-pg-${_suffix}}"
: "${ROOTCAUSEWAY_LAB_PG_ADMIN_USER:=rcaiadmin}"
: "${ROOTCAUSEWAY_LAB_PG_SKU:=Standard_B1ms}"
: "${ROOTCAUSEWAY_LAB_ACR:=rcailabacr${_suffix}}"
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
