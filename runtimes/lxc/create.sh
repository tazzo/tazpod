#!/usr/bin/env bash
# create.sh — TazPod LXC Proxmox Orchestrator
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"
PET_DIR="${SCRIPT_DIR}/terraform-pet"
ANSIBLE_DIR="${SCRIPT_DIR}/ansible"
CONFIG_DIR="${SCRIPT_DIR}/configs"
LOG_DIR="${SCRIPT_DIR}/logs"

mkdir -p "$LOG_DIR"

TS="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="${LOG_DIR}/create_${TS}.log"
RUNTIME_ENV="${CONFIG_DIR}/runtime.env"
INVENTORY="${ANSIBLE_DIR}/inventory.ini"

# SSH settings
KEY="$HOME/.ssh/id_ed25519"
SSH="ssh -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $KEY"

# Timing
declare -A PHASE_TIMES
PHASE_ORDER=()

timed_phase() {
  local desc="$1"; shift
  echo "─── ${desc} ───" | tee -a "$LOG_FILE"
  PHASE_ORDER+=("$desc")
  local start=$(date +%s)
  "$@" 2>&1 | tee -a "$LOG_FILE"
  local end=$(date +%s)
  PHASE_TIMES["$desc"]=$((end - start))
}

print_timings() {
  echo "" | tee -a "$LOG_FILE"
  echo "Phase timings:" | tee -a "$LOG_FILE"
  for phase in "${PHASE_ORDER[@]}"; do
    printf "  %s: %ds\n" "$phase" "${PHASE_TIMES[$phase]}" | tee -a "$LOG_FILE"
  done
}

load_secrets() {
  if [ -f "${RUNTIME_ENV}" ]; then
    set -a; source "${RUNTIME_ENV}"; set +a
  fi
}

API() {
  local method="$1"; shift; local path="$1"; shift
  curl -sk -X "$method" "https://192.168.1.200:8006/api2/json${path}" \
    -H "Authorization: PVEAPIToken=${PROXMOX_VE_API_TOKEN}" "$@"
}

# ─── Phases ───

phase_pet_ensure() {
  local ip="${1:-192.168.1.200}"
  echo "Verifying pet CT 999 and volume vm-999-disk-2..."
  if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes "root@${ip}" "pct status 999 >/dev/null 2>&1 && lvs | grep -q vm-999-disk-2"; then
    echo "Pet CT 999 già pronto, volume vm-999-disk-2 presente."
    return 0
  fi
  echo "Pet CT 999 o volume vm-999-disk-2 mancante. Eseguo Terraform..."
  cd "$PET_DIR"
  terraform init -input=false
  terraform apply -auto-approve -input=false
}

phase_terraform() {
  cd "$TERRAFORM_DIR"
  terraform init -input=false
  terraform apply -auto-approve -input=false
}


phase_raw_config() {
  local ct_id="${1:-106}"
  local proxmox_ip="${2:-192.168.1.200}"
  echo "Applying raw LXC config for CT ${ct_id} via SSH to Proxmox host ${proxmox_ip}..."

  ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes "root@${proxmox_ip}" "bash -s" << SSHRAW
    set -e
    CTID=$ct_id

    pct stop \$CTID 2>/dev/null || true
    sleep 2

    # Remove stale lxc.* entries, then add fresh ones
    sed -i '/^lxc\./d' /etc/pve/lxc/\${CTID}.conf
    for opt in \
      "lxc.cgroup2.devices.allow=c 10:200 rwm" \
      "lxc.mount.entry=/dev/net/tun dev/net/tun none bind,create=file" \
      "lxc.cap.drop=sys_rawio" \
      "lxc.cap.drop=sys_module" \
      "lxc.cap.drop=sys_ptrace" \
      "lxc.cap.drop=mac_admin" \
      "lxc.cap.drop=mac_override" \
      "lxc.idmap=u 0 100000 1000" \
      "lxc.idmap=g 0 100000 1000" \
      "lxc.idmap=u 1000 1000 1" \
      "lxc.idmap=g 1000 1000 1" \
      "lxc.idmap=u 1001 101001 64535" \
      "lxc.idmap=g 1001 101001 64535"; do
      echo "\$opt" >> /etc/pve/lxc/\${CTID}.conf
    done

    # Shared mount bind
    mkdir -p /mnt/shared /var/lib/lxc/\${CTID}/rootfs/mnt/shared 2>/dev/null || true
    grep -q "^mp1:" /etc/pve/lxc/\${CTID}.conf || echo "mp1: /mnt/shared,mp=/mnt/shared,acl=1" >> /etc/pve/lxc/\${CTID}.conf

    pct start \$CTID
}

phase_ansible_baseline() {
  cd "$SCRIPT_DIR"
  ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook -i "$INVENTORY" "$ANSIBLE_DIR/tazpod-baseline.yml"
}

phase_transfer_oauth() {
  local ip="${1:-192.168.1.206}"
  echo "Transferring OAuth credentials from gopass to LXC..."

  $SSH "root@${ip}" "mkdir -p /home/tazpod/secrets && chown tazpod:tazpod /home/tazpod/secrets && chmod 700 /home/tazpod/secrets"

  gopass show bootstrap/tailscale/oauth-client-id >/dev/null 2>&1 && \
    gopass show bootstrap/tailscale/oauth-client-id | $SSH "root@${ip}" "cat > /home/tazpod/secrets/tailscale-oauth-client-id && chmod 600 /home/tazpod/secrets/tailscale-oauth-client-id && chown tazpod:tazpod /home/tazpod/secrets/tailscale-oauth-client-id"

  gopass show bootstrap/tailscale/oauth-client-secret >/dev/null 2>&1 && \
    gopass show bootstrap/tailscale/oauth-client-secret | $SSH "root@${ip}" "cat > /home/tazpod/secrets/tailscale-oauth-client-secret && chmod 600 /home/tazpod/secrets/tailscale-oauth-client-secret && chown tazpod:tazpod /home/tazpod/secrets/tailscale-oauth-client-secret"

  gopass show bootstrap/tailscale/api-key >/dev/null 2>&1 && \
    gopass show bootstrap/tailscale/api-key | $SSH "root@${ip}" "cat > /home/tazpod/secrets/tailscale-api-key && chmod 600 /home/tazpod/secrets/tailscale-api-key && chown tazpod:tazpod /home/tazpod/secrets/tailscale-api-key"

  gopass show bootstrap/tailscale/tailnet >/dev/null 2>&1 && \
    gopass show bootstrap/tailscale/tailnet | $SSH "root@${ip}" "cat > /home/tazpod/secrets/tailscale-tailnet && chmod 600 /home/tazpod/secrets/tailscale-tailnet && chown tazpod:tazpod /home/tazpod/secrets/tailscale-tailnet"
}

phase_tailscale_start() {
  local ip="${1:-192.168.1.206}"
  echo "Starting Tailscale on LXC..."
  $SSH "root@${ip}" "tailscale up --auth-key \$(cat /home/tazpod/secrets/tailscale-oauth-client-secret) --accept-routes=false --advertise-routes=192.168.1.0/24 2>&1"
  sleep 3
  $SSH "root@${ip}" "cd /home/tazpod && bash start-lxc.sh"
}

phase_verify() {
  local ip="${1:-192.168.1.206}"
  echo "Verifying deployment..."
  $SSH "root@${ip}" "/usr/local/bin/tazpod --version"
  $SSH "root@${ip}" "tailscale status"
  echo "Verification complete."
}

 
phase_wait_ssh() {
  local ip="${1:-192.168.1.206}"
  echo "Removing stale host key for ${ip}..."
  ssh-keygen -R "${ip}" 2>/dev/null || true
  echo "Waiting for SSH on ${ip}..."
  for i in $(seq 1 30); do
    if $SSH "root@${ip}" "echo ok" 2>/dev/null; then
      echo "SSH ready."
      return 0
    fi
    sleep 5
  done
  echo "SSH timeout" >&2
  return 1
}

main() {
  local ip="${1:-192.168.1.206}"
  local ct_id="${2:-106}"

  load_secrets

  timed_phase "0. Pet Volume Ensure"  phase_pet_ensure
  timed_phase "1. Terraform Create"   phase_terraform
  timed_phase "2. Raw Config Apply"   phase_raw_config "$ct_id"
  timed_phase "3. Wait SSH"           phase_wait_ssh "$ip"
  timed_phase "4. Ansible Baseline"   phase_ansible_baseline
  timed_phase "5. Transfer OAuth"     phase_transfer_oauth "$ip"
  timed_phase "6. Tailscale Start"    phase_tailscale_start "$ip"
  timed_phase "7. Verify"             phase_verify "$ip"

  print_timings
  echo "Deployment successful." | tee -a "$LOG_FILE"
}

main "$@"
