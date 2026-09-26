#!/usr/bin/env bash
# destroy.sh — TazPod Paperclip Container Destroy (CT 108)
#
# What this destroys: the container CT 108 (`paperclip`) and the rootfs volume
# PVE created for it (local-lvm:vm-108-disk-0). Nothing else.
#
# What it preserves:
#   * the guest's OWN data volume (local-lvm:vm-108-disk-1, mp0 → /workspace),
#     because the phase below detaches mp0 *before* the destroy: PVE deletes the
#     volumes a guest references, and leaves unreferenced disks alone. Without
#     that step, destroying the container would destroy the carried company state
#     (master key, instance tree, run logs). create.sh finds the orphaned volume
#     and re-attaches it by name, which is the operator runtime's "the data disk
#     survives" pattern applied to a volume this guest owns;
#   * the PostgreSQL database, the gopass stores and every entry in them;
#   * the operator's environment — CT106 is not touched, and neither is the pet
#     volume vm-999-disk-2 it mounts;
#   * the T4 Proxmox objects (proxmox-access.tf: user, role, pool, ACLs, token).
#     They are the operator's; the guard below refuses to run while they are in
#     this state, so a guest rebuild can never revoke the agent's identity.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"
CONFIG_DIR="${SCRIPT_DIR}/configs"
RUNTIME_ENV="${CONFIG_DIR}/runtime.env"

# Same key as create.sh: root on the orchestration host, used here only to stop
# the guest and detach its data volume before Terraform removes the container.
KEY="${SSH_KEY_PATH:-/root/.ssh/tazpod-provision}"
SSH=(ssh -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$KEY")

if [ ! -f "${RUNTIME_ENV}" ]; then
  echo "ERROR: ${RUNTIME_ENV} missing — cannot tell which guest and volume this would destroy" >&2
  exit 1
fi
set -a; source "${RUNTIME_ENV}"; set +a

pve_ssh() {
  "${SSH[@]}" "root@${PROXMOX_HOST}" "$@"
}

guest_exists() {
  pve_ssh "pct status $1 >/dev/null 2>&1"
}

cd "$TERRAFORM_DIR"
terraform init -input=false

# T4's objects are the operator's apply (see proxmox-access.tf). With the shipped
# default (manage_proxmox_access=false) they are absent from state and nothing to
# worry about; if an operator applied them from this directory they ARE in state,
# and a destroy with the default would remove them. Refuse, and say what to do.
access_objects="$(terraform state list 2>/dev/null | grep -E '^proxmox_virtual_environment_(user|role|acl|user_token|pool)' || true)"
if [ -n "$access_objects" ]; then
  echo "ERROR: the operator-applied T4 objects are in this state:" >&2
  echo "$access_objects" | sed 's/^/  /' >&2
  echo "Destroying the guest must not revoke the agent's identity. Remove them from state first" >&2
  echo "(they keep working):" >&2
  echo "  terraform state rm <each address above>" >&2
  exit 1
fi

if guest_exists "$CT_ID"; then
  # Detach mp0 while the guest is stopped (raw keys and mount points are applied
  # at start, and a running container refuses the change in some PVE versions).
  echo "Detaching the data volume from CT ${CT_ID} so it survives the destroy..."
  preserved="$(pve_ssh "pct config ${CT_ID} | sed -n -E 's/^mp0:[[:space:]]*([^,]+).*/\1/p' | head -1")"
  pve_ssh "pct stop ${CT_ID} 2>/dev/null || true"
  if [ -n "$preserved" ]; then
    pve_ssh "pct set ${CT_ID} --delete mp0"
    echo "Detached: ${preserved}"
  else
    # An interrupted earlier destroy may already have detached it; deleting a
    # non-existent key would abort the run before the guest is even removed.
    echo "mp0 is already detached (nothing to delete)"
  fi
else
  echo "CT ${CT_ID} does not exist — nothing to detach; the data volume (if any) is already unreferenced."
fi

# ssh_public_key has no default (create.sh supplies the provisioning machine's
# key); destroy ignores it, but Terraform still requires a value for every
# variable.
terraform destroy -auto-approve -input=false -var "ssh_public_key="

# Say what actually happened rather than what was intended: PVE keeps
# unreferenced volumes, but the volume name is PVE's, so read it back.
if pve_ssh "lvs --noheadings -o lv_name 2>/dev/null | tr -d ' ' | grep -qx '${DATA_VOLUME_NAME}'"; then
  echo "CT ${CT_ID} destroyed. Data volume ${STORAGE_POOL}:${DATA_VOLUME_NAME} preserved."
  echo "  ./create.sh finds it and re-attaches it as mp0 — or by hand, on ${PROXMOX_HOST}:"
  echo "    pct set ${CT_ID} --mp0 ${STORAGE_POOL}:${DATA_VOLUME_NAME},mp=${DATA_VOLUME_PATH},backup=1"
else
  echo "CT ${CT_ID} destroyed." >&2
  echo "WARN: ${STORAGE_POOL}:${DATA_VOLUME_NAME} is gone — the data volume did NOT survive." >&2
  echo "WARN: either it was already detached/destroyed by hand, or a purge removed it; the carried" >&2
  echo "WARN: company state on it now needs the PostgreSQL/gopass side plus a fresh instance tree." >&2
fi
