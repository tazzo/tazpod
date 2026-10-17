#!/usr/bin/env bash
# install-t3code.sh — install T3 Code as a background service inside CT106 (tazpod-proxmox).
#
# Run this ON the container, as root:
#
#   ssh root@192.168.1.206 'bash -s' < runtimes/lxc/t3code/install-t3code.sh
#   # or, from the lab layer:
#   ansible -i runtimes/lxc/ansible/inventory.ini tazpod -m script -a runtimes/lxc/t3code/install-t3code.sh
#
# It is idempotent: run it again after a change to the files in this directory.
#
# What it does, in order:
#   1. enables lingering for the service user (without it a *user* service dies
#      with the last SSH session — measured Linger=no on CT106);
#   2. downloads the `t3` CLI with the vendor installer, as the service user,
#      into /home/<user>/.t3 + /home/<user>/.local/bin;
#   3. renders this layer's systemd drop-ins into the user's own config:
#        ~/.config/systemd/user/t3code.service.d/50-lab.conf   (slice, PATH, TMPDIR, stale-lock guard)
#        ~/.config/systemd/user/app-t3code.slice               (the resource cap)
#        /usr/local/lib/t3code/clean-stale-runtime.sh          (ExecStartPre helper)
#   4. installs the background service (`t3 service install`) and restarts it so
#      the drop-ins take effect;
#   5. runs verify-t3code.sh and fails loudly if a check does not pass.
#
# Nothing here runs as root except the file writes and `loginctl`: the server,
# the agents and the provider CLIs all run as T3_USER.
#
# Environment overrides:
#   T3_USER=tazpod        service user (default: the lab user that already owns
#                         opencode/pi/tailscale logins)
#   T3_CHANNEL=stable     stable | nightly
#   T3_VERSION=           pin an exact version (e.g. 0.1.0); empty = channel head
#   T3_SKIP_SERVICE=0     set 1 to install files only, without touching systemd
set -euo pipefail

T3_USER="${T3_USER:-tazpod}"
T3_CHANNEL="${T3_CHANNEL:-stable}"
T3_VERSION="${T3_VERSION:-}"
T3_SKIP_SERVICE="${T3_SKIP_SERVICE:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

die() { printf '\ninstall-t3code: %s\n' "$1" >&2; exit 1; }
say() { printf '  %s\n' "$1"; }

[ "$(id -u)" -eq 0 ] || die "run as root (the layer writes into the service user's systemd config and calls loginctl)"
id "$T3_USER" >/dev/null 2>&1 || die "user '$T3_USER' does not exist on this machine"

T3_USER_HOME="$(getent passwd "$T3_USER" | cut -d: -f6)"
T3_UID="$(id -u "$T3_USER")"
[ -n "$T3_USER_HOME" ] || die "cannot resolve the home directory of '$T3_USER'"
T3_HOME_DIR="${T3_USER_HOME}/.t3"
RUNTIME_DIR="/run/user/${T3_UID}"
USER_UNIT_DIR="${T3_USER_HOME}/.config/systemd/user"

as_user() {
  # A user service needs the user manager, which needs XDG_RUNTIME_DIR.
  runuser -u "$T3_USER" -- env \
    "HOME=${T3_USER_HOME}" \
    "XDG_RUNTIME_DIR=${RUNTIME_DIR}" \
    "DBUS_SESSION_BUS_ADDRESS=unix:path=${RUNTIME_DIR}/bus" \
    "T3CODE_HOME=${T3_HOME_DIR}" \
    "$@"
}

# ─── 1. lingering ───────────────────────────────────────────────────────────
say "[1/5] lingering for ${T3_USER}"
if [ "$(loginctl show-user "$T3_USER" --property=Linger --value)" != "yes" ]; then
  loginctl enable-linger "$T3_USER"
  say "enabled (a user service now survives logout and starts at boot)"
else
  say "already enabled"
fi
# With linger on, systemd-logind starts the user manager; make sure it is there.
if ! systemctl is-active --quiet "user@${T3_UID}.service"; then
  systemctl start "user@${T3_UID}.service"
  sleep 2
fi
[ -d "$RUNTIME_DIR" ] || die "no ${RUNTIME_DIR}: the user manager did not start (linger is on but logind did not spawn it)"

# ─── 2. the CLI ─────────────────────────────────────────────────────────────
say "[2/5] t3 CLI for ${T3_USER} (channel ${T3_CHANNEL}${T3_VERSION:+, pinned ${T3_VERSION}})"
INSTALL_BIN_DIR="${T3_USER_HOME}/.local/bin"
if as_user sh -c "PATH=${INSTALL_BIN_DIR}:\$PATH command -v t3 >/dev/null 2>&1"; then
  say "already installed: $(as_user sh -c "PATH=${INSTALL_BIN_DIR}:\$PATH t3 --version" 2>/dev/null || echo 'version unknown')"
else
  # The vendor installer needs only sh, tar, curl and sha256sum — no Node.
  as_user env \
    "T3CODE_CHANNEL=${T3_CHANNEL}" \
    ${T3_VERSION:+"T3CODE_VERSION=${T3_VERSION}"} \
    "T3CODE_HOME=${T3_HOME_DIR}" \
    "T3CODE_INSTALL_BIN_DIR=${INSTALL_BIN_DIR}" \
    sh -c 'curl -fsSL https://t3.codes/install.sh | sh'
fi
T3_BIN="${INSTALL_BIN_DIR}/t3"
[ -x "$T3_BIN" ] || die "the installer did not produce ${T3_BIN}"
say "installed: $(as_user "$T3_BIN" --version)"

# The provider CLIs live under the Node bin directory the shell used at install
# time (nvm on this lab); the service does not read ~/.bashrc, so bake it in.
NODE_BIN_DIR="$(as_user bash -lc 'dirname "$(command -v node)"' 2>/dev/null || true)"
[ -n "$NODE_BIN_DIR" ] || NODE_BIN_DIR="/usr/bin"

# ─── 3. drop-ins ────────────────────────────────────────────────────────────
say "[3/5] systemd drop-ins under ${USER_UNIT_DIR}"
install -d -o "$T3_USER" -g "$T3_USER" -m 0755 \
  "${USER_UNIT_DIR}/t3code.service.d" \
  "${T3_HOME_DIR}/tmp" \
  /usr/local/lib/t3code

render() { # render <template> <destination> <mode>
  sed -e "s|@T3_USER@|${T3_USER}|g" \
      -e "s|@T3_USER_HOME@|${T3_USER_HOME}|g" \
      -e "s|@NODE_BIN_DIR@|${NODE_BIN_DIR}|g" \
      "$1" > "$2"
  chown "${T3_USER}:${T3_USER}" "$2"
  chmod "$3" "$2"
}

render "${SCRIPT_DIR}/systemd/t3code.service.d/50-lab.conf" \
       "${USER_UNIT_DIR}/t3code.service.d/50-lab.conf" 0644
render "${SCRIPT_DIR}/systemd/app-t3code.slice" \
       "${USER_UNIT_DIR}/app-t3code.slice" 0644
render "${SCRIPT_DIR}/systemd/clean-stale-runtime.sh" \
       /usr/local/lib/t3code/clean-stale-runtime.sh 0755
say "wrote 50-lab.conf, app-t3code.slice and /usr/local/lib/t3code/clean-stale-runtime.sh"

if [ "$T3_SKIP_SERVICE" != "1" ]; then
  as_user systemctl --user daemon-reload
  # ─── 4. the service ───────────────────────────────────────────────────────
  say "[4/5] the background service"
  if as_user systemctl --user cat t3code.service >/dev/null 2>&1; then
    as_user systemctl --user restart t3code.service
  else
    as_user "$T3_BIN" service install
  fi
  # `service install` starts it; a drop-in change needs the restart above.
  sleep 3
  as_user systemctl --user --no-pager status t3code.service | sed -n '1,6p' || true
fi

# ─── 5. verify ──────────────────────────────────────────────────────────────
say "[5/5] verification"
T3_USER="$T3_USER" bash "${SCRIPT_DIR}/verify-t3code.sh"
say "done"
