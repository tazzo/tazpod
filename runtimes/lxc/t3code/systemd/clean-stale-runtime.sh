#!/bin/sh
# clean-stale-runtime.sh — ExecStartPre guard for t3code.service.
#
# The trap (operator's guide §10, "Collisioni PID in Container"): T3 Code records
# the running server in
#   <T3CODE_HOME>/userdata/server-runtime.json     (pid + origin; apps/server/src/config.ts:146)
# and refuses to start when it believes a server is already up for that data
# directory ("A T3 Code server is already running for <dir> (pid N, ...)"). After
# an unclean container restart the file can survive while the pid it names is
# gone — or, worse, has been recycled by an unrelated process.
#
# So: before the launcher reads it, drop the file only when its pid is not
# alive. A live pid is left alone — that is a real second server, and stealing
# its runtime file would be worse than refusing to start. The stale file is
# moved aside, not deleted, so the incident stays inspectable.
#
# Rendered by install-t3code.sh into /usr/local/lib/t3code/clean-stale-runtime.sh
# with @T3_USER_HOME@ substituted; T3CODE_HOME from the unit wins when present.
set -eu

base_dir="${T3CODE_HOME:-@T3_USER_HOME@/.t3}"
runtime="${base_dir}/userdata/server-runtime.json"
[ -f "$runtime" ] || exit 0

pid="$(sed -n 's/.*"pid"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$runtime" | head -n 1)"
[ -n "$pid" ] || exit 0
kill -0 "$pid" 2>/dev/null && exit 0

stale="${runtime}.stale.$(date +%Y%m%d%H%M%S)"
mv -f "$runtime" "$stale" 2>/dev/null || true
printf 't3code: stale runtime file (pid %s is gone) moved to %s\n' "$pid" "$stale" >&2
exit 0
