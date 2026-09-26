#!/usr/bin/env bash
# create.sh — TazPod Paperclip Container Orchestrator (CT 108)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"
ANSIBLE_DIR="${SCRIPT_DIR}/ansible"
CONFIG_DIR="${SCRIPT_DIR}/configs"
LOG_DIR="${SCRIPT_DIR}/logs"

mkdir -p "$LOG_DIR"

TS="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="${LOG_DIR}/create_${TS}.log"
RUNTIME_ENV="${CONFIG_DIR}/runtime.env"
INVENTORY="${ANSIBLE_DIR}/inventory.ini"
PLAYBOOK="${ANSIBLE_DIR}/paperclip-baseline.yml"
ANSIBLE_CFG="${ANSIBLE_DIR}/ansible.cfg"

# SSH to the Proxmox host (and later to the fresh guest, as root) uses the
# operator-owned `tazpod-provision` key, root-only on the trusted orchestration
# host. TAZ-12: NOT the operator's personal ~/.ssh/id_ed25519. Its public half is
# the only key Terraform injects into the guest; the private half never leaves
# this machine.
KEY="${SSH_KEY_PATH:-/root/.ssh/tazpod-provision}"
SSH=(ssh -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$KEY")

# I/O ceiling applied by the raw-config phase. It exists so one runaway agent run
# cannot starve the other guests on local-lvm; the numbers are the operator
# runtime's, kept identical on purpose (one storage budget for the lab).
IO_RBPS=104857600
IO_WBPS=104857600
IO_RIOPS=5000
IO_WIOPS=5000

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
    # Runtime facts (CT id, address, datastore, data volume, Proxmox host), all
    # plain values rendered by Terraform — never a secret.
    set -a; source "${RUNTIME_ENV}"; set +a
  fi
  # The Proxmox API token is never in a file in this repo: it is either already
  # exported by the operator or read from gopass here (the storage runtime's
  # pattern). Nothing prints it.
  if [ -z "${PROXMOX_VE_API_TOKEN:-}" ] && command -v gopass >/dev/null 2>&1; then
    local token_id token_secret
    token_id="$(gopass show bootstrap/proxmox/token-id 2>/dev/null | tr -d "'\"\t\r\n " || true)"
    token_secret="$(gopass show bootstrap/proxmox/token-secret 2>/dev/null | tr -d "'\"\t\r\n " || true)"
    if [ -n "$token_id" ] && [ -n "$token_secret" ]; then
      export PROXMOX_VE_API_TOKEN="${token_id}=${token_secret}"
    fi
  fi
}

# Every root-side operation on the Proxmox host goes through this, because an API
# token cannot write raw LXC keys or manipulate a guest's mount points the way the
# phases below need.
pve_ssh() {
  "${SSH[@]}" "root@${PROXMOX_HOST}" "$@"
}

guest_exists() {
  pve_ssh "pct status $1 >/dev/null 2>&1"
}

volume_exists() {
  pve_ssh "lvs --noheadings -o lv_name 2>/dev/null | tr -d ' ' | grep -qx '$1'"
}

# T4's objects (proxmox-access.tf) belong to the operator's apply. If they are in
# this state — i.e. the operator applied with -var manage_proxmox_access=true from
# this directory — a run with the shipped default would destroy them, silently
# revoking the agent's identity (or failing halfway through the apply). Refuse
# instead: dropping them has to be a deliberate act, and the objects stay alive.
assert_no_access_objects_in_state() {
  local found
  found="$(terraform state list 2>/dev/null | grep -E '^proxmox_virtual_environment_(user|role|acl|user_token|pool)' || true)"
  if [ -n "$found" ]; then
    echo "ERROR: the operator-applied T4 objects are in this state:" >&2
    echo "$found" | sed 's/^/  /' >&2
    echo "This apply would delete them. Either re-run with -var manage_proxmox_access=true (operator," >&2
    echo "with Permissions.Modify), or take them out of state first (they keep working):" >&2
    echo "  terraform state rm <each address above>" >&2
    return 1
  fi
}

# ─── Phases ───

phase_preflight() {
  local pub="${KEY}.pub"
  if [ ! -s "$pub" ]; then
    echo "ERROR: ${pub} missing or empty — Terraform injects this machine's provisioning key into the new guest" >&2
    return 1
  fi
  if [ -z "${PROXMOX_VE_API_TOKEN:-}" ]; then
    echo "ERROR: PROXMOX_VE_API_TOKEN is not set and gopass did not yield bootstrap/proxmox/token-id+token-secret" >&2
    return 1
  fi
  if [ ! -f "$PLAYBOOK" ]; then
    echo "WARN: ${PLAYBOOK} not found — the Terraform phases can still run, the Ansible phase will fail" >&2
  fi
  if [ "$(id -u)" != "0" ]; then
    echo "WARN: running as uid $(id -u); ${KEY} is root-only on this host, so SSH to the Proxmox host will fail" >&2
  fi
}

phase_terraform() {
  cd "$TERRAFORM_DIR"
  local pub="${KEY}.pub"
  local -a tf_vars=(-var "ssh_public_key=$(cat "$pub")")

  # Rebuild path. destroy.sh detaches mp0 before destroying the guest, so the
  # container's own data volume survives as an unreferenced volume
  # (local-lvm:vm-108-disk-1). If the guest is gone but that volume is still on
  # the host, hand its id to Terraform: the rebuild then re-attaches the
  # company's state instead of leaving it orphaned next to a fresh, empty volume.
  # Only in that exact case — while the guest exists, the volume is already
  # attached and passing the id again would put a diff on every run.
  if ! guest_exists "$CT_ID" && volume_exists "$DATA_VOLUME_NAME"; then
    echo "Guest CT ${CT_ID} is gone but the data volume ${STORAGE_POOL}:${DATA_VOLUME_NAME} survived — re-attaching it."
    tf_vars+=(-var "data_volume_id=${STORAGE_POOL}:${DATA_VOLUME_NAME}")
  fi

  terraform init -input=false
  assert_no_access_objects_in_state
  terraform apply -auto-approve -input=false "${tf_vars[@]}"
}

# The PVE-side configuration Terraform cannot express. What is genuinely needed
# for this guest:
#   * the cgroup2 I/O ceiling — there is no Terraform attribute for it;
#   * the capability drops — the only place a deliberate, statement-of-record
#     capability policy can live;
#   * the enforcement of the posture Terraform declares (unprivileged, AppArmor
#     at the default profile) — asserted here, because a guest provisioned with a
#     weaker boundary must stop the run, not start anyway.
# What was specific to the operator container and is deliberately NOT carried:
#   * the `lxc.idmap` lines — they existed to hand host uid 1000 (the pet volume's
#     owner) into CT106. This guest has its own volume, created by PVE with the
#     standard unprivileged mapping, and PVE writes that mapping itself: penning
#     a custom one here would be the operator container's shape, not this one's;
#   * the /dev/net/tun allow + bind mount — Tailscale's device on CT106. The
#     agent's egress path is the T4b chokepoint on vmbr0, and /dev/net/tun is raw
#     packet capability this guest has no use for;
#   * the `mp1: /mnt/shared` bind mount — the operator's workspace. This guest's
#     volume is declared in Terraform as mp0, not bound from the host;
#   * keyctl() — CT106 needs it for its gpg/Docker workload; this guest's key path
#     is the passphrase-less scoped store key (T3), which does not use the kernel
#     keyring. The feature is a checkbox in main.tf if that ever changes.
# The phase is idempotent: it builds the desired block, compares it with the lines
# it owns in the guest's config, and touches nothing (no restart) when they match.
phase_raw_config() {
  local ct_id="${1:?ct_id required}"
  echo "Applying the raw LXC config for CT ${ct_id} on ${PROXMOX_HOST} (root SSH; an API token cannot set raw keys)..."

  pve_ssh "CTID=${ct_id} IO_RBPS=${IO_RBPS} IO_WBPS=${IO_WBPS} IO_RIOPS=${IO_RIOPS} IO_WIOPS=${IO_WIOPS} bash -s" <<'SSHRAW'
set -eu

conf="/etc/pve/lxc/${CTID}.conf"
[ -f "$conf" ] || { echo "ERROR: ${conf} does not exist" >&2; exit 1; }

# Posture assertions. These two are the boundary DESIGN §8.3 relies on; if either
# was changed by hand, provision nothing more on top of it.
if ! grep -qE '^unprivileged:[[:space:]]*1' "$conf"; then
  echo "ERROR: CT ${CTID} is not unprivileged (${conf}) — refusing to apply a hardening policy to a privileged guest" >&2
  exit 1
fi
if grep -qE '^lxc\.apparmor\.profile[[:space:]]*[:=][[:space:]]*unconfined' "$conf"; then
  echo "ERROR: CT ${CTID} runs with AppArmor set to unconfined — refusing to continue" >&2
  exit 1
fi

# Resolve the block devices backing this guest. Done while the guest is running:
# PVE assigns kernel major:minor only to active volumes (an inactive thin volume
# reports -1). Both the rootfs and the data volume get the ceiling, so neither the
# guest's own writes nor its state volume can flood the pool.
devs=""
for vol in $(pct config "$CTID" | sed -n -E 's/^(rootfs|mp[0-9]+):[[:space:]]*([^,]+).*/\2/p'); do
  path="$(pvesm path "$vol" 2>/dev/null || true)"
  [ -n "$path" ] || continue
  majmin="$(lvs --noheadings -o kernel_major,kernel_minor "$path" 2>/dev/null | tr -s ' ' | sed -e 's/^ //' -e 's/ /:/')"
  case "$majmin" in
    [0-9]*:[0-9]*) devs="${devs}${devs:+ }${majmin}" ;;
    *) echo "WARN: no kernel device for ${vol} (inactive?) — its I/O ceiling is skipped" >&2 ;;
  esac
done
if [ -z "$devs" ]; then
  echo "ERROR: no backing device resolved for CT ${CTID} — refusing to write a partial policy" >&2
  exit 1
fi

# Only the lines this phase owns are ever removed (the operator runtime deletes
# every lxc.* line, which also erases PVE's own generated ones). `[:=]` matches
# the `=` form an operator runtime phase would have written, so a guest rebuilt
# from an older shape is cleaned up rather than doubled.
owned_re='^lxc\.(cap\.drop|cgroup2\.io\.max)[[:space:]]*[:=]'
want="$(mktemp)"
{
  # Capability policy: the operator runtime's list, verbatim. LXC unions
  # lxc.cap.drop, so a capability the unprivileged default set already removes is
  # a harmless duplicate, while the ones it does not remove are exactly the reach
  # this policy exists to take away (raw device access, module loading, process
  # inspection, MAC policy override). Nothing is added.
  echo "lxc.cap.drop: sys_rawio"
  echo "lxc.cap.drop: sys_module"
  echo "lxc.cap.drop: sys_ptrace"
  echo "lxc.cap.drop: mac_admin"
  echo "lxc.cap.drop: mac_override"
  for dev in $devs; do
    echo "lxc.cgroup2.io.max: ${dev} rbps=${IO_RBPS} wbps=${IO_WBPS} riops=${IO_RIOPS} wiops=${IO_WIOPS}"
  done
} > "$want"
have="$(grep -E "$owned_re" "$conf" || true)"
if [ "$(cat "$want")" = "$have" ]; then
  echo "raw config already applied for CT ${CTID} — no restart needed (idempotent no-op)"
  rm -f "$want"
  exit 0
fi

# Raw keys only take effect at container start, so the guest is restarted — and
# only when something actually changes (the comparison above).
pct stop "$CTID" 2>/dev/null || true
# Same short pause the operator runtime's phase takes before writing: PVE can
# still hold the container's config lock for a moment after a stop, and a write
# into that window is rejected.
sleep 2
sed -i -E "/$owned_re/d" "$conf"
cat "$want" >> "$conf"
rm -f "$want"
pct start "$CTID"
echo "raw config applied to CT ${CTID}:"
grep -E "$owned_re" "$conf" | sed 's/^/  /'
SSHRAW
}

phase_wait_ssh() {
  local ip="$1"
  echo "Removing stale host key for ${ip}..."
  ssh-keygen -R "${ip}" 2>/dev/null || true
  echo "Waiting for SSH on ${ip}..."
  for i in $(seq 1 30); do
    if "${SSH[@]}" "root@${ip}" "echo ok" 2>/dev/null; then
      echo "SSH ready."
      return 0
    fi
    sleep 5
  done
  echo "SSH timeout on ${ip}" >&2
  return 1
}

phase_ansible_baseline() {
  local ip="$1"
  cd "$SCRIPT_DIR"
  if [ ! -f "$PLAYBOOK" ]; then
    echo "ERROR: ${PLAYBOOK} missing — the provisioning layer (base tools, the paperclip service account, its key and store) is not present" >&2
    return 1
  fi
  if [ ! -f "$INVENTORY" ]; then
    echo "ERROR: ${INVENTORY} missing — the provisioning layer's inventory carries the host alias (create.sh only supplies the address)" >&2
    return 1
  fi
  # Explicit config when the layer ships one: the guest's overlayfs has no ACL
  # support, so the unprivileged-become temp-file strategy must fall back (the
  # operator runtime's config exists for the same reason).
  if [ -f "$ANSIBLE_CFG" ]; then
    export ANSIBLE_CONFIG="$ANSIBLE_CFG"
  else
    echo "WARN: ${ANSIBLE_CFG} absent — using Ansible's default config" >&2
  fi
  # The guest's address is static, and it is also passed explicitly so the
  # playbook does not depend on the inventory's host line carrying it.
  ANSIBLE_HOST_KEY_CHECKING=False \
    ansible-playbook -i "$INVENTORY" -e "ansible_host=${ip}" "$PLAYBOOK"
}

phase_verify() {
  local ip="$1"
  echo "Verifying CT ${CT_ID} at ${ip}..."
  "${SSH[@]}" "root@${ip}" "EXPECTED_IP=${IP_ADDRESS} WORKSPACE=${DATA_VOLUME_PATH} PROXMOX_HOST=${PROXMOX_HOST} bash -s" <<'VERIFY'
set -u

echo "── address ──"
if hostname -I | tr ' ' '\n' | grep -qx "${EXPECTED_IP}"; then
  echo "address: ${EXPECTED_IP}"
else
  echo "FAIL: the guest does not hold ${EXPECTED_IP} — the T4 credentials (Vault AppRole bound CIDRs, PVE scoping) are written against that address" >&2
  exit 1
fi

echo "── data volume ──"
if mountpoint -q "${WORKSPACE}"; then
  echo "${WORKSPACE}: mounted (the guest's own volume — not the operator's pet volume)"
else
  echo "FAIL: ${WORKSPACE} is not a mount point — PAPERCLIP_HOME would sit on the rootfs and the carried company state would be lost at the next rebuild" >&2
  exit 1
fi

echo "── identity ──"
id paperclip 2>/dev/null || echo "WARN: user paperclip absent (provisioning layer incomplete?)"

echo "── service ──"
printf 'paperclip: %s\n' "$(systemctl is-active paperclip.service 2>/dev/null)"

echo "── boundary (must refuse) ──"
if id paperclip >/dev/null 2>&1; then
  # The negative acceptance the design asks for: the agent's identity must not be
  # able to log in to the hypervisor, because the guest holds no key that opens it.
  if sudo -u paperclip -H ssh -o BatchMode=yes -o ConnectTimeout=3 -o StrictHostKeyChecking=no "root@${PROXMOX_HOST}" true 2>/dev/null; then
    echo "FAIL: the agent user can log in to the Proxmox host" >&2
    exit 1
  fi
  echo "proxmox login as the agent user: refused (as designed)"
else
  echo "SKIP: cannot test without the paperclip user"
fi
VERIFY
  echo "Verification complete."
}

main() {
  load_secrets
  # Every phase below is written against these facts; without them a phase would
  # fail later with an empty host or volume name (and under `set -u` the failure
  # would not even say which fact is missing).
  if [ -z "${CT_ID:-}" ] || [ -z "${IP_ADDRESS:-}" ] || [ -z "${DATA_VOLUME_NAME:-}" ] || [ -z "${PROXMOX_HOST:-}" ]; then
    echo "ERROR: ${RUNTIME_ENV} missing or incomplete (needs CT_ID, IP_ADDRESS, DATA_VOLUME_NAME, PROXMOX_HOST)" >&2
    exit 1
  fi

  timed_phase "0. Preflight" phase_preflight
  timed_phase "1. Terraform Create" phase_terraform
  timed_phase "2. Raw Config Apply" phase_raw_config "$CT_ID"
  timed_phase "3. Wait SSH" phase_wait_ssh "$IP_ADDRESS"
  timed_phase "4. Ansible Baseline" phase_ansible_baseline "$IP_ADDRESS"
  timed_phase "5. Verify" phase_verify "$IP_ADDRESS"

  print_timings
  echo "Deployment successful." | tee -a "$LOG_FILE"
}

main "$@"
