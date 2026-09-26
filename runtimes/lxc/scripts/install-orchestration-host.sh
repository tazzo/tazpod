#!/usr/bin/env bash
# install-orchestration-host.sh — prepare the trusted TazPod orchestration host.
#
# Run as root ON the trusted orchestration host (tazlab, 192.168.1.200). This is
# the host that owns the operator-only provisioning key /root/.ssh/tazpod-provision
# and whose source address the key's authorized_keys entry is pinned to with
# `from="192.168.1.200"` (TAZ-12). Because of that `from=` restriction the
# orchestration MUST originate from tazlab; an admin workstation can only drive it
# through `ssh tazlab`.
#
# What it does (idempotent):
#   1. installs ansible-core, terraform, gopass, git and their prerequisites
#   2. clones the tazpod checkout and the tazlab-secrets checkout under /root
#   3. verifies the provisioning keypair, its ownership/mode and the key pin
#   4. prints the TAZ-19 acceptance commands
#
# It never prints or copies a private key. For gopass/terraform phases the
# operator supplies credentials out of band (see the runbook).
#
# Usage:
#   sudo env GITHUB_TOKEN=<token> ./install-orchestration-host.sh
#   sudo TAZPOD_REPO_URL=... SECRETS_REPO_URL=... ./install-orchestration-host.sh
#
# Ref: TAZ-10 design memo (O3), TAZ-12 apply report, TAZ-19.

set -euo pipefail

ORCH_ROOT="${ORCH_ROOT:-/root}"
TAZPOD_DIR="${ORCH_ROOT}/tazpod"
SECRETS_DIR="${ORCH_ROOT}/tazlab-secrets"
KEY="${ORCH_KEY_PATH:-/root/.ssh/tazpod-provision}"
PROVISION_FROM="${PROVISION_FROM:-192.168.1.200}"
TAZPOD_REPO_URL="${TAZPOD_REPO_URL:-https://github.com/tazzo/tazpod.git}"
SECRETS_REPO_URL="${SECRETS_REPO_URL:-https://github.com/tazzo/tazlab-secrets.git}"

log()  { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33mWARN: %s\033[0m\n' "$*" >&2; }
die()  { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

require_root() {
  [ "$(id -u)" -eq 0 ] || die "run this as root on the orchestration host (tazlab)"
}

install_packages() {
  command -v apt-get >/dev/null 2>&1 || die "only Debian/Ubuntu hosts are supported"
  log "Installing base packages (ansible-core, gopass, git, helpers)"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  # ansible-core/gopass are in Debian 13 (trixie / PVE 9). If the distro package
  # is missing, fall back to pipx/upstream below.
  apt-get install -y --no-install-recommends \
    ca-certificates curl gnupg git openssh-client gopass ansible-core python3 || true

  if ! command -v ansible-playbook >/dev/null 2>&1; then
    warn "ansible-core not available from apt; installing with pip"
    apt-get install -y --no-install-recommends python3-pip python3-venv
    python3 -m venv /opt/ansible
    /opt/ansible/bin/pip install --upgrade pip
    /opt/ansible/bin/pip install 'ansible-core>=2.16'
    ln -sf /opt/ansible/bin/ansible /usr/local/bin/ansible
    ln -sf /opt/ansible/bin/ansible-playbook /usr/local/bin/ansible-playbook
  fi

  if ! command -v gopass >/dev/null 2>&1; then
    warn "gopass not available from apt; install it from https://github.com/gopasspw/gopass/releases"
  fi
}

install_terraform() {
  if command -v terraform >/dev/null 2>&1; then
    log "terraform already present: $(terraform version | head -n1)"
    return 0
  fi
  log "Installing terraform from the HashiCorp apt repository"
  install -m 0755 -d /etc/apt/keyrings
  if [ ! -s /etc/apt/keyrings/hashicorp-archive-keyring.gpg ]; then
    curl -fsSL https://apt.releases.hashicorp.com/gpg \
      | gpg --dearmor -o /etc/apt/keyrings/hashicorp-archive-keyring.gpg
    chmod 0644 /etc/apt/keyrings/hashicorp-archive-keyring.gpg
  fi
  . /etc/os-release
  echo "deb [signed-by=/etc/apt/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com ${VERSION_CODENAME} main" \
    > /etc/apt/sources.list.d/hashicorp.list
  apt-get update -y
  apt-get install -y terraform
}

clone_checkout() {
  local url="$1" dir="$2" name
  name="$(basename "$dir")"
  if [ -d "$dir/.git" ]; then
    log "$name checkout already present at $dir; leaving it untouched"
    return 0
  fi
  log "Cloning $name into $dir"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    git -c credential.helper="!f() { echo username=x-access-token; echo password=${GITHUB_TOKEN}; }; f" \
      clone --depth 1 "$url" "$dir"
  else
    git clone --depth 1 "$url" "$dir" \
      || die "clone of $name failed; re-run with GITHUB_TOKEN set (never store the token)"
  fi
}

verify_key() {
  log "Verifying the provisioning key ${KEY}"
  [ -f "$KEY" ] || die "missing ${KEY}: generate it on tazlab and keep it 0600 root:root"
  local mode owner
  mode="$(stat -c '%a' "$KEY")"
  owner="$(stat -c '%U:%G' "$KEY")"
  [ "$mode" = "600" ] || die "${KEY} must be mode 0600 (found $mode)"
  [ "$owner" = "root:root" ] || die "${KEY} must be owned root:root (found $owner)"
  if [ ! -s "${KEY}.pub" ]; then
    die "${KEY}.pub is missing: create.sh's terraform phase injects this public key into a new CT"
  fi
  log "Private key present, 0600 root:root (not printed). Public key:"
  ssh-keygen -lf "${KEY}.pub" || true
  log "Authorized_keys pin on the target must contain from=\"${PROVISION_FROM}\" for this key"
}

verify_acceptance() {
  log "TAZ-19 acceptance commands (run from ${TAZPOD_DIR}/runtimes/lxc/ansible)"
  cat <<EOF
cd ${TAZPOD_DIR}/runtimes/lxc/ansible
ANSIBLE_CONFIG=\$PWD/ansible.cfg ansible -i inventory.ini tazpod -m ping
ANSIBLE_CONFIG=\$PWD/ansible.cfg ansible-playbook -i inventory.ini tazpod-baseline.yml --check --diff
EOF
  log "Toolchain versions"
  for t in ansible ansible-playbook terraform gopass git; do
    if command -v "$t" >/dev/null 2>&1; then
      printf '  %-18s %s\n' "$t" "$($t --version 2>/dev/null | head -n1)"
    else
      printf '  %-18s MISSING\n' "$t"
    fi
  done
}

main() {
  require_root
  install_packages
  install_terraform
  mkdir -p "$ORCH_ROOT"
  clone_checkout "$TAZPOD_REPO_URL" "$TAZPOD_DIR"
  clone_checkout "$SECRETS_REPO_URL" "$SECRETS_DIR"
  verify_key
  verify_acceptance
  log "Done. Next: copy runtime.env to ${TAZPOD_DIR}/runtimes/lxc/configs/ (secret, operator-only) and run the check above."
}

main "$@"
