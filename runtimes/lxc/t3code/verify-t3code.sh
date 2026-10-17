#!/usr/bin/env bash
# verify-t3code.sh — preflight and post-install verification for T3 Code on CT106.
#
# Run it on the container with root (the lab path is the Ansible/ssh one):
#
#   ssh root@192.168.1.206 'T3_USER=tazpod bash -s' < runtimes/lxc/t3code/verify-t3code.sh
#
# Every line is `PASS|WARN|FAIL <check> <detail>`; it exits 1 if anything FAILs.
# Run it BEFORE the install as a preflight: the checks that matter (linger,
# provider CLIs on the service PATH, tailnet, disk, bind) are exactly the things
# that made the operator's guide wrong about this machine.
#
# It never prints a secret: credential files are reported as present/absent only.
set -uo pipefail

T3_USER="${T3_USER:-tazpod}"
T3_PORT="${T3_PORT:-3773}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

FAILS=0
pass() { printf 'PASS %-28s %s\n' "$1" "$2"; }
warn() { printf 'WARN %-28s %s\n' "$1" "$2"; }
fail() { printf 'FAIL %-28s %s\n' "$1" "$2"; FAILS=$((FAILS + 1)); }

[ "$(id -u)" -eq 0 ] || warn "root" "not root: loginctl checks and the user's home may be unreadable"

id "$T3_USER" >/dev/null 2>&1 || { fail "user" "'$T3_USER' does not exist"; exit 1; }
USER_HOME="$(getent passwd "$T3_USER" | cut -d: -f6)"
USER_UID="$(id -u "$T3_USER")"
RUNTIME_DIR="/run/user/${USER_UID}"
T3_HOME_DIR="${USER_HOME}/.t3"

as_user() {
  runuser -u "$T3_USER" -- env "HOME=${USER_HOME}" "XDG_RUNTIME_DIR=${RUNTIME_DIR}" \
    "DBUS_SESSION_BUS_ADDRESS=unix:path=${RUNTIME_DIR}/bus" "T3CODE_HOME=${T3_HOME_DIR}" "$@"
}

# ── 1. the machine ──────────────────────────────────────────────────────────
host="$(hostname)"
ip4="$(ip -4 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | paste -sd, -)"
[ -n "$ip4" ] && pass "address" "${host} ${ip4}" || warn "address" "no global IPv4 found"

mem_high="$(cat /sys/fs/cgroup/memory.high 2>/dev/null || true)"
mem_max="$(cat /sys/fs/cgroup/memory.max 2>/dev/null || true)"
if [ -n "$mem_high$mem_max" ]; then
  pass "cgroup memory" "high=${mem_high:-?} max=${mem_max:-?}"
else
  warn "cgroup memory" "unreadable here (run as root); the layer assumes high=6GiB max=8GiB"
fi

avail_kb="$(df -Pk / 2>/dev/null | awk 'NR==2{print $4}')"
if [ -n "${avail_kb:-}" ] && [ "$avail_kb" -gt 20971520 ]; then
  pass "disk /" "$((avail_kb / 1048576)) GiB free"
elif [ -n "${avail_kb:-}" ]; then
  fail "disk /" "only $((avail_kb / 1048576)) GiB free; t3 doctor needs ~2x the database size twice over"
else
  warn "disk /" "could not read"
fi

db="${T3_HOME_DIR}/userdata/statev2.sqlite"
if [ -f "$db" ]; then
  db_mb=$(( $(stat -c %s "$db") / 1048576 ))
  tmp_free_kb="$(df -Pk "${T3_HOME_DIR}/tmp" 2>/dev/null | awk 'NR==2{print $4}')"
  if [ -n "${tmp_free_kb:-}" ] && [ "$tmp_free_kb" -gt $(( db_mb * 2 * 1024 )) ]; then
    pass "database headroom" "statev2.sqlite ${db_mb} MiB, free on TMPDIR $((tmp_free_kb / 1048576)) GiB"
  else
    fail "database headroom" "statev2.sqlite ${db_mb} MiB needs ~$((db_mb * 2)) MiB free next to it and in TMPDIR"
  fi
else
  pass "database headroom" "no statev2.sqlite yet (nothing to repair)"
fi

# ── 2. lingering and the user manager ───────────────────────────────────────
if [ "$(loginctl show-user "$T3_USER" --property=Linger --value 2>/dev/null)" = "yes" ]; then
  pass "linger" "Linger=yes for ${T3_USER}"
else
  fail "linger" "Linger=no: a user service dies with the last session (fix: loginctl enable-linger ${T3_USER})"
fi

if as_user systemctl --user is-system-running >/dev/null 2>&1; then
  pass "user manager" "$(as_user systemctl --user is-system-running 2>/dev/null)"
else
  fail "user manager" "systemctl --user is not answering for ${T3_USER}"
fi

# ── 3. the CLI and the service ──────────────────────────────────────────────
T3_BIN="${USER_HOME}/.local/bin/t3"
if [ -x "$T3_BIN" ]; then
  pass "t3 CLI" "$(as_user "$T3_BIN" --version 2>&1 | head -n1)"
else
  warn "t3 CLI" "not installed yet at ${T3_BIN}"
fi

unit="$(as_user systemctl --user cat t3code.service 2>/dev/null || true)"
if [ -n "$unit" ]; then
  svc_state="$(as_user systemctl --user is-active t3code.service 2>/dev/null)"
  svc_enabled="$(as_user systemctl --user is-enabled t3code.service 2>/dev/null)"
  if [ "$svc_state" = "active" ]; then
    pass "service" "t3code.service active (${svc_enabled})"
  else
    fail "service" "t3code.service ${svc_state:-unknown}"
  fi
  slice="$(as_user systemctl --user show t3code.service -p Slice --value 2>/dev/null)"
  if [ "$slice" = "app-t3code.slice" ]; then
    pass "service slice" "Slice=${slice}"
  else
    fail "service slice" "Slice=${slice:-none}: the drop-in 50-lab.conf is missing or not reloaded"
  fi
else
  warn "service" "t3code.service is not installed"
fi

# ── 4. the cap ──────────────────────────────────────────────────────────────
if as_user systemctl --user show app-t3code.slice -p MemoryMax >/dev/null 2>&1; then
  lim="$(as_user systemctl --user show app-t3code.slice -p MemoryHigh -p MemoryMax -p CPUQuota -p TasksMax --value 2>/dev/null | paste -sd' ' -)"
  if [ -n "${lim// /}" ] && [ "$(as_user systemctl --user show app-t3code.slice -p MemoryMax --value 2>/dev/null)" != "infinity" ]; then
    pass "resource cap" "app-t3code.slice MemoryHigh/Max/CPUQuota/TasksMax = ${lim}"
  else
    fail "resource cap" "app-t3code.slice has no limits (static unit missing)"
  fi
else
  warn "resource cap" "app-t3code.slice is not loaded"
fi

# ── 5. providers, on the PATH the service actually has ──────────────────────
svc_path="$(as_user systemctl --user show t3code.service -p Environment --value 2>/dev/null \
  | tr ' ' '\n' | sed -n 's/^PATH=//p' | head -n1)"
if [ -n "$svc_path" ]; then
  found=""
  for b in opencode pi claude codex cursor-agent grok agy muse; do
    if as_user env "PATH=${svc_path}" sh -c "command -v ${b} >/dev/null 2>&1"; then found="${found} ${b}"; fi
  done
  if [ -n "${found// /}" ]; then
    pass "providers on PATH" "found:${found}"
  else
    fail "providers on PATH" "no provider CLI resolves on the service PATH (${svc_path})"
  fi
  case ":${svc_path}:" in
    *":${USER_HOME}/.local/bin:"*) pass "PATH has .local/bin" "yes" ;;
    *) fail "PATH has .local/bin" "opencode/pi are symlinked there; the service will not see them" ;;
  esac
else
  warn "providers on PATH" "the service is not installed, cannot read its PATH"
fi

authed=""
for f in "${USER_HOME}/.local/share/opencode/auth.json" \
         "${USER_HOME}/.codex/auth.json" \
         "${USER_HOME}/.claude/.credentials.json" \
         "${USER_HOME}/.pi/auth.json"; do
  [ -f "$f" ] && authed="${authed} ${f#${USER_HOME}/}"
done
[ -n "${authed// /}" ] && pass "provider logins" "credentials present:${authed}" \
                       || warn "provider logins" "no known credential file found (log in before the trial)"

# ── 6. the network route the proposal recommends ────────────────────────────
if command -v tailscale >/dev/null 2>&1; then
  ts_state="$(tailscale status 2>&1 | head -n1)"
  ts_ip="$(tailscale ip -4 2>/dev/null | head -n1)"
  if [ -n "$ts_ip" ]; then
    pass "tailscale" "up, ${ts_ip}"
  else
    fail "tailscale" "not joined to the tailnet (${ts_state}) — the recommended route, and pairing over it, needs this"
  fi
  if tailscale serve status >/dev/null 2>&1; then
    pass "tailscale serve" "$(tailscale serve status 2>&1 | head -n2 | paste -sd'; ' -)"
  else
    warn "tailscale serve" "no serve mapping yet (t3 pair --tailscale creates one)"
  fi
else
  fail "tailscale" "tailscale is not installed; the recommended remote route is unavailable"
fi

# ── 7. can the service user bind the port at all ────────────────────────────
if ss -ltn 2>/dev/null | grep -q ":${T3_PORT}\b"; then
  holder="$(ss -ltnp 2>/dev/null | grep ":${T3_PORT}\b" | head -n1)"
  case "$holder" in
    *t3*|*node*) pass "port ${T3_PORT}" "held by the T3 server: ${holder}" ;;
    *) fail "port ${T3_PORT}" "held by something else: ${holder}" ;;
  esac
else
  if ! command -v python3 >/dev/null 2>&1; then
    warn "port ${T3_PORT}" "free; python3 missing so the bind test was skipped"
  elif [ "$(as_user python3 -c "import socket;s=socket.socket();s.bind(('127.0.0.1',${T3_PORT}));s.close();print('ok')" 2>/dev/null | tr -d '[:space:]')" = "ok" ]; then
    pass "port ${T3_PORT}" "free, and ${T3_USER} can bind loopback"
  else
    fail "port ${T3_PORT}" "${T3_USER} cannot bind loopback: the server would fail with EACCES"
  fi
fi

# ── 8. the in-app browser ───────────────────────────────────────────────────
chrome="$(command -v google-chrome chromium chromium-browser 2>/dev/null | head -n1)"
[ -n "$chrome" ] && pass "headless browser" "$chrome" \
                 || warn "headless browser" "not installed; first browser tab downloads ~120 MB, or run 'sudo t3 browser setup'"
userns="$(sysctl -n kernel.apparmor_restrict_unprivileged_userns 2>/dev/null || echo '?')"
[ "$userns" = "0" ] && pass "browser sandbox" "unprivileged userns allowed" \
                    || warn "browser sandbox" "apparmor_restrict_unprivileged_userns=${userns}: run 'sudo t3 browser setup', or set T3CODE_SERVER_BROWSER_SANDBOX=0 for the environment"

# ── verdict ─────────────────────────────────────────────────────────────────
if [ "$FAILS" -eq 0 ]; then
  printf '\nverify-t3code: all checks passed\n'
else
  printf '\nverify-t3code: %s check(s) FAILED\n' "$FAILS"
fi
exit $(( FAILS > 0 ))
