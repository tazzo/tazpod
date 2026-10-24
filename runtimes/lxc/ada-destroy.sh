#!/usr/bin/env bash
# ada-destroy.sh — remove the Ada container. The engine's data lives on its own disk,
# so this deletes it: take a snapshot first if the state matters.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export PROXMOX_VE_API_TOKEN="$(gopass show -o bootstrap/proxmox/token-id)=$(gopass show -o bootstrap/proxmox/token-secret)"
cd "${SCRIPT_DIR}/terraform-ada"
terraform destroy -auto-approve -input=false -var "ssh_public_key=$(cat "${ADA_PROVISION_KEY:-/root/.ssh/tazpod-provision}.pub")"
