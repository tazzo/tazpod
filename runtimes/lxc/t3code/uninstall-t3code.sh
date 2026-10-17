#!/usr/bin/env bash
# uninstall-t3code.sh — remove the layer's footprint from CT106, keep the data.
#
#   ssh root@192.168.1.206 'bash -s' < runtimes/lxc/t3code/uninstall-t3code.sh
#   ... --purge    also delete ~/.t3 (projects, threads, settings, the database)
#
# Run as root on the container. It stops the user service, removes the drop-ins
# this layer installed, and leaves the `t3` CLI in place unless --purge is given
# (the CLI is the vendor's installer; `t3 uninstall` as the service user removes
# it together with its downloads).
set -euo pipefail

T3_USER="${T3_USER:-tazpod}"
PURGE=0
[ "${1:-}" = "--purge" ] && PURGE=1

[ "$(id -u)" -eq 0 ] || { echo "run as root" >&2; exit 1; }
USER_HOME="$(getent passwd "$T3_USER" | cut -d: -f6)"
USER_UID="$(id -u "$T3_USER")"
RUNTIME_DIR="/run/user/${USER_UID}"
UNIT_DIR="${USER_HOME}/.config/systemd/user"

as_user() {
  runuser -u "$T3_USER" -- env "HOME=${USER_HOME}" "XDG_RUNTIME_DIR=${RUNTIME_DIR}" \
    "DBUS_SESSION_BUS_ADDRESS=unix:path=${RUNTIME_DIR}/bus" "$@"
}

if as_user systemctl --user cat t3code.service >/dev/null 2>&1; then
  as_user "${USER_HOME}/.local/bin/t3" service uninstall || as_user systemctl --user disable --now t3code.service || true
fi

rm -f "${UNIT_DIR}/t3code.service.d/50-lab.conf"
rmdir "${UNIT_DIR}/t3code.service.d" 2>/dev/null || true
rm -f "${UNIT_DIR}/app-t3code.slice"
rm -f /usr/local/lib/t3code/clean-stale-runtime.sh
rmdir /usr/local/lib/t3code 2>/dev/null || true
as_user systemctl --user daemon-reload || true

if [ "$PURGE" = "1" ]; then
  as_user "${USER_HOME}/.local/bin/t3" uninstall --yes || true
  rm -rf "${USER_HOME}/.t3"
  echo "purged ${USER_HOME}/.t3"
fi

# Lingering is deliberately left on: other user services on this container may
# rely on it, and turning it off is a lab decision, not a side effect.
echo "removed the t3code service and this layer's drop-ins for ${T3_USER}"
