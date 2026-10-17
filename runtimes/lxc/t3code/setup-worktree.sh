#!/usr/bin/env bash
# setup-worktree.sh — the worktree-create hook declared in t3.json.
#
# Copy to <repo>/.t3/setup-worktree.sh and commit it together with t3.json at the
# repository root. T3 Code runs it in a terminal at the new worktree's root, with
# T3CODE_PROJECT_ROOT pointing at the primary checkout — verified in the upstream
# source (packages/shared/src/projectScripts.ts:56 sets T3CODE_PROJECT_ROOT to the
# project's cwd, and scripts/setup-worktree.ts refuses to run without it).
#
# The cold-start problem it solves: a fresh worktree holds only tracked files, so
# every ignored file the build needs (.env, local config, generated assets) is
# missing. Link them from the primary checkout instead of copying: a rotated key
# then propagates to every live worktree at once.
#
# It is idempotent and never overwrites something that already exists in the
# worktree — a second run on a reused worktree changes nothing.
set -euo pipefail

: "${T3CODE_PROJECT_ROOT:?T3CODE_PROJECT_ROOT is not set: run this through t3.json, not by hand}"
cd "$(pwd)"

linked=0
for f in .env .env.local .envrc .t3/local.env; do
  [ -e "${T3CODE_PROJECT_ROOT}/${f}" ] || continue
  [ -e "$f" ] && continue
  mkdir -p "$(dirname "$f")"
  ln -s "${T3CODE_PROJECT_ROOT}/${f}" "$f"
  printf 'linked %s -> %s\n' "$f" "${T3CODE_PROJECT_ROOT}/${f}"
  linked=$((linked + 1))
done

printf 'worktree ready: %s (%s link(s))\n' "$PWD" "$linked"
