#!/usr/bin/env bash
# ada-create.sh — create the Ada container and configure what is inside it.
#
# Run as root on the provisioning host (it needs /root/.ssh/tazpod-provision and an
# unlocked gopass store). It never prints a secret: gopass is read into variables,
# and the Ansible extra-vars file is 0600 and shredded at the end.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="${SCRIPT_DIR}/terraform-ada"
ANSIBLE_DIR="${SCRIPT_DIR}/ansible"
LOG_DIR="${SCRIPT_DIR}/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="${LOG_DIR}/ada-create_$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "$LOG_FILE") 2>&1

ADA_SRC="${ADA_SRC:-/home/tazpod/workspace/ada}"
PROVISION_KEY="${ADA_PROVISION_KEY:-/root/.ssh/tazpod-provision}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }; }
for tool in terraform ansible-playbook rsync gopass curl jq; do need "$tool"; done
[ -r "${PROVISION_KEY}" ] || { echo "provision key not readable: ${PROVISION_KEY} (run as root)"; exit 1; }
[ -d "${ADA_SRC}" ] || { echo "source tree not found: ${ADA_SRC}"; exit 1; }

echo "── secrets (never printed) ──"
export PROXMOX_VE_API_TOKEN="$(gopass show -o bootstrap/proxmox/token-id)=$(gopass show -o bootstrap/proxmox/token-secret)"
ADA_OPENCODE="$(gopass show -o infra/t3code/opencode-key)"
ADA_EXA="$(gopass show -o infra/exa/api-key)"
ADA_HTTP_TOKEN="$(gopass show -o infra/ada/http-token)" || {
  echo "missing gopass entry infra/ada/http-token (create it: openssl rand -hex 24 | gopass insert -f infra/ada/http-token)"
  exit 1
}

SSH_OPTS=(-o ConnectTimeout=10 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o BatchMode=yes -i "${PROVISION_KEY}")
SSH=(ssh "${SSH_OPTS[@]}")

echo "── terraform apply ──"
cd "$TF_DIR"
terraform init -input=false
terraform apply -auto-approve -input=false -var "ssh_public_key=$(cat "${PROVISION_KEY}.pub")"
IP="$(terraform output -raw ct_ip)"
CT_ID="$(terraform output -raw ct_id)"
echo "container ${CT_ID} at ${IP}"

echo "── wait for ssh ──"
for _ in $(seq 1 60); do
  if "${SSH[@]}" "root@${IP}" true 2>/dev/null; then break; fi
  sleep 3
done
"${SSH[@]}" "root@${IP}" true || { echo "ssh never came up"; exit 1; }

echo "── copy the engine ──"
rsync -az --delete \
  --exclude node_modules --exclude data --exclude workspace --exclude .git --exclude config.json \
  -e "ssh ${SSH_OPTS[*]}" "${ADA_SRC}/" "root@${IP}:/opt/ada/app/"

echo "── ansible: system, node, user, unit ──"
EA_FILE="$(mktemp)"; chmod 600 "$EA_FILE"
trap 'shred -u "$EA_FILE" 2>/dev/null || rm -f "$EA_FILE"' EXIT
jq -n --arg oc "$ADA_OPENCODE" --arg ex "$ADA_EXA" --arg ht "$ADA_HTTP_TOKEN" \
  '{ada_opencode_api_key:$oc, ada_exa_api_key:$ex, ada_http_token:$ht}' > "$EA_FILE"
ansible-playbook -i "${ANSIBLE_DIR}/inventory-ada.ini" "${ANSIBLE_DIR}/ada-baseline.yml" -e "@${EA_FILE}"

echo "── dependencies and start ──"
"${SSH[@]}" "root@${IP}" "chown -R ada:ada /opt/ada/app && cd /opt/ada/app && sudo -u ada npm ci --omit=dev && systemctl restart ada"

echo "── health ──"
for _ in $(seq 1 30); do
  if curl -fsS -H "Authorization: Bearer ${ADA_HTTP_TOKEN}" "http://${IP}:7801/health" >/dev/null 2>&1; then
    curl -s -H "Authorization: Bearer ${ADA_HTTP_TOKEN}" "http://${IP}:7801/health" | jq .
    break
  fi
  sleep 2
done

echo
echo "Ada is at http://${IP}:7801 (bearer token in infra/ada/http-token)"
echo "log: ${LOG_FILE}"
