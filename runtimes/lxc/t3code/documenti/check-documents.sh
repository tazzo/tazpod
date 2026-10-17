#!/usr/bin/env bash
# check-documents.sh — the project's queue documents are on the hub, and carry
# the words the queue looks for: plan, roadmap, progress.
#
#   bash runtimes/lxc/t3code/documenti/check-documents.sh
#
# It reads the live hub (http://127.0.0.1:7807/api/v1/documents?project=t3code)
# and falls back to the snapshot in this directory when the hub is not reachable
# from where it runs, saying which of the two it used. Exit 0 only when all three
# words are present.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
URL="${T3_HUB_DOCUMENTS_URL:-http://127.0.0.1:7807/api/v1/documents?project=t3code}"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

source_used="live hub"
if ! curl -fsS -m 5 "$URL" -o "$TMP" 2>/dev/null; then
  cp "${HERE}/pubblicati.json" "$TMP"
  source_used="snapshot ${HERE}/pubblicati.json (hub not reachable from here)"
fi

FAILS=0
for word in plan roadmap progress; do
  if python3 - "$TMP" "$word" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
word = sys.argv[2]
# the endpoint answers with both `written` (project documents) and `documents`
# (delivery records); only `written` carries the documents.
docs = doc.get("written")
if docs is None:
    docs = doc.get("documents", [])
titled = [d.get("title", "") for d in docs if word in d.get("title", "").lower()]
anywhere = [d.get("title", "") for d in docs
            if word in (d.get("title", "") + " " + d.get("body", "")).lower()]
if titled:
    print(f"  {word}: titled document -> {titled[0]} ({len(titled)} titled, {len(anywhere)} mentioning)")
    sys.exit(0)
if anywhere:
    print(f"  {word}: only mentioned in the body of -> {anywhere[0]} ({len(anywhere)} mentioning, none titled)")
    sys.exit(1)
print(f"  {word}: MISSING")
sys.exit(1)
PY
  then
    printf 'PASS document %s\n' "$word"
  else
    printf 'FAIL document %s\n' "$word"; FAILS=$((FAILS + 1))
  fi
done

printf 'source: %s\n' "$source_used"
if [ "$FAILS" -eq 0 ]; then printf 'check-documents: plan, roadmap and progress are all on the hub\n'; else printf 'check-documents: %s missing\n' "$FAILS"; fi
exit $(( FAILS > 0 ))
