#!/usr/bin/env bash
# check-layer.sh — static checks for this layer. No root, no machine access, no
# network: it proves the files are syntactically sound and that t3.json really is
# valid against the published T3 project-file schema (a snapshot of
# https://t3.codes/schema/t3.json taken on 2026-10-17 lives in
# documenti/t3.schema.json, so the check works offline).
#
#   bash runtimes/lxc/t3code/check-layer.sh      # exits 0, prints one line per check
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FAILS=0
ok()   { printf 'PASS %s\n' "$1"; }
bad()  { printf 'FAIL %s\n' "$1"; FAILS=$((FAILS + 1)); }

for f in install-t3code.sh verify-t3code.sh uninstall-t3code.sh setup-worktree.sh; do
  if bash -n "${HERE}/${f}" 2>/dev/null; then ok "bash -n ${f}"; else bad "bash -n ${f}"; fi
done
if sh -n "${HERE}/systemd/clean-stale-runtime.sh" 2>/dev/null; then
  ok "sh -n systemd/clean-stale-runtime.sh"
else
  bad "sh -n systemd/clean-stale-runtime.sh"
fi

python3 - "$HERE/t3.json" "$HERE/documenti/t3.schema.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
schema = json.load(open(sys.argv[2], encoding="utf-8"))
errors = []
def check(inst, sch, path="$"):
    t = sch.get("type")
    if t == "object":
        if not isinstance(inst, dict):
            errors.append(f"{path}: not an object"); return
        for k in sch.get("required", []):
            if k not in inst: errors.append(f"{path}: missing {k}")
        props = sch.get("properties", {})
        if sch.get("additionalProperties") is False:
            for k in inst:
                if k not in props: errors.append(f"{path}: unexpected property {k}")
        for k, v in inst.items():
            if k in props: check(v, props[k], f"{path}.{k}")
    elif t == "array":
        if not isinstance(inst, list):
            errors.append(f"{path}: not an array"); return
        for i, v in enumerate(inst): check(v, sch.get("items", {}), f"{path}[{i}]")
    elif t == "string":
        if not isinstance(inst, str):
            errors.append(f"{path}: not a string"); return
        if "enum" in sch and inst not in sch["enum"]: errors.append(f"{path}: {inst!r} not allowed")
        if "minLength" in sch and len(inst) < sch["minLength"]: errors.append(f"{path}: too short")
        if "maxLength" in sch and len(inst) > sch["maxLength"]: errors.append(f"{path}: too long")
    elif t == "boolean":
        if not isinstance(inst, bool): errors.append(f"{path}: not a boolean")
check(doc, schema)
if errors:
    print("FAIL t3.json vs " + schema["$id"] + ": " + "; ".join(errors)); sys.exit(1)
print(f"PASS t3.json valid against {schema['$id']} ({len(doc.get('scripts', []))} script(s))")
PY
[ $? -eq 0 ] || FAILS=$((FAILS + 1))

for f in PROPOSTA.md README.md t3.json setup-worktree.sh documenti/PIANO.md documenti/ROADMAP.md documenti/PROGRESSO.md documenti/pubblicati.json; do
  if [ -s "${HERE}/${f}" ]; then ok "present ${f}"; else bad "present ${f}"; fi
done

if [ "$FAILS" -eq 0 ]; then printf '\ncheck-layer: all checks passed\n'; else printf '\ncheck-layer: %s failed\n' "$FAILS"; fi
exit $(( FAILS > 0 ))
