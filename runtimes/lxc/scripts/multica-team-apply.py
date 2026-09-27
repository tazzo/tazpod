#!/usr/bin/env python3
"""Reconcile the TazLab agent team on the Multica control plane from a declarative roster.

The roster (`configs/multica-team/roster.yml`) is the source of truth: it names the
workspace, the runtime, the shared skills and the agents, and it points at the markdown
file that holds each agent's instructions. This script makes the control plane match it —
creating, updating and re-binding only what actually drifted.

Design notes, because they are the reason this is a script and not three Ansible task files:

* **Idempotence is decided by comparison, not by hope.** Every object is read back and
  compared field by field before anything is written; a converged roster produces zero
  writes. That is what makes `--dry-run` honest and a second Ansible run `changed=0`.
* **No id is stored anywhere.** The workspace is resolved from its slug and the runtime
  from its name at run time, so a rebuilt control plane does not invalidate the roster.
* **Nothing is deleted.** An agent or a skill that leaves the roster is left alone in the
  control plane. Removal is a deliberate operator action, not a side effect.
* **Nothing secret is handled.** `custom_env`, `mcp_config` and credentials are outside
  this script's remit by construction: they never appear in the roster.
* **The CLI is the interface.** It is the same binary the daemon layer installs, it is
  authenticated from `~/.multica/config.json`, and it is the surface the platform documents
  for exactly these operations.

Output: a single JSON object on stdout (`{"changed", "actions", "errors"}`), so Ansible can
set `changed_when` from it; the human-readable trace goes to stderr.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

import yaml


class Failure(Exception):
    """A condition the caller must see as a hard failure (not a missing-but-optional one)."""


def log(message: str) -> None:
    print(message, file=sys.stderr, flush=True)


def cli(args: list[str], env: dict[str, str], *, expect_json: bool = True):
    """Run the `multica` CLI with no shell, and return its parsed JSON output."""
    proc = subprocess.run(
        ["multica", *args],
        capture_output=True,
        text=True,
        env=env,
        check=False,
    )
    if proc.returncode != 0:
        raise Failure(
            f"`multica {' '.join(args)}` exited {proc.returncode}: "
            f"{(proc.stderr or proc.stdout).strip()}"
        )
    if not expect_json:
        return None
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise Failure(
            f"`multica {' '.join(args)}` did not return JSON ({exc}): {proc.stdout[:400]!r}"
        ) from exc


def read_text(path: Path, what: str) -> str:
    if not path.is_file():
        raise Failure(f"{what}: {path} does not exist")
    return path.read_text(encoding="utf-8")


def norm(text: str) -> str:
    """Compare bodies without trailing whitespace, so a stray newline is not drift."""
    return text.rstrip()


# ─── resolution ─────────────────────────────────────────────────────────────


def resolve_workspace(env: dict[str, str], slug: str) -> str:
    workspaces = cli(["workspace", "list", "--output", "json"], env)
    for workspace in workspaces:
        if workspace.get("slug") == slug:
            return workspace["id"]
    raise Failure(
        f"workspace `{slug}` not found; the token belongs to: "
        + ", ".join(w.get("slug", "?") for w in workspaces)
    )


def resolve_runtime(env: dict[str, str], name: str) -> str:
    runtimes = cli(["runtime", "list", "--output", "json"], env)
    for runtime in runtimes:
        if name in (runtime.get("name"), runtime.get("custom_name")):
            return runtime["id"]
    raise Failure(
        f"runtime `{name}` not found; available: "
        + ", ".join(sorted(filter(None, (r.get("name") for r in runtimes))))
    )


# ─── skills ─────────────────────────────────────────────────────────────────


def reconcile_skills(roster: dict, base: Path, env: dict[str, str], dry: bool, actions: list):
    """Create or update each declared skill; return {name: id}."""
    existing = {s["name"]: s for s in cli(["skill", "list", "--output", "json"], env)}
    ids: dict[str, str] = {}

    for skill in roster.get("skills", []):
        name = skill["name"]
        body = read_text(base / skill["content"], f"skill `{name}` content")
        description = " ".join((skill.get("description") or "").split())

        if name not in existing:
            actions.append(f"skill create: {name}")
            log(f"[skill] {name}: absent -> create")
            if not dry:
                created = cli(
                    [
                        "skill", "create",
                        "--name", name,
                        "--description", description,
                        "--content", body,
                        "--output", "json",
                    ],
                    env,
                )
                ids[name] = created["id"]
            else:
                ids[name] = f"<new:{name}>"
            continue

        current = cli(
            ["skill", "get", existing[name]["id"], "--with-content", "--output", "json"], env
        )
        ids[name] = current["id"]
        drifted = []
        if norm(current.get("content") or "") != norm(body):
            drifted.append("content")
        if (current.get("description") or "") != description:
            drifted.append("description")
        if not drifted:
            log(f"[skill] {name}: converged")
            continue

        actions.append(f"skill update: {name} ({', '.join(drifted)})")
        log(f"[skill] {name}: drift in {', '.join(drifted)} -> update")
        if not dry:
            cli(
                [
                    "skill", "update", current["id"],
                    "--description", description,
                    "--content", body,
                    "--output", "json",
                ],
                env,
            )
    return ids


# ─── agents ─────────────────────────────────────────────────────────────────


def desired_invocation(permission: str, workspace_id: str, permission_mode: str, targets) -> bool:
    """Does the stored invocation permission already match the declared one?"""
    if permission == "workspace":
        if permission_mode != "public_to":
            return False
        return any(
            t.get("target_type") == "workspace" and t.get("target_id") == workspace_id
            for t in (targets or [])
        )
    if permission == "private":
        return permission_mode != "public_to" and not targets
    raise Failure(f"unknown permission `{permission}` (expected `workspace` or `private`)")


def reconcile_agents(
    roster: dict, base: Path, env: dict[str, str], workspace_id: str,
    runtime_id: str, skill_ids: dict[str, str], dry: bool, actions: list,
):
    defaults = roster.get("defaults") or {}
    existing = {a["name"]: a for a in cli(["agent", "list", "--output", "json"], env)}

    for agent in roster.get("agents", []):
        name = agent["name"]
        desired = {**defaults, **agent}
        instructions = read_text(base / desired["instructions"], f"agent `{name}` instructions")
        description = " ".join((desired.get("description") or "").split())
        concurrency = int(desired.get("max_concurrent_tasks", 1))
        permission = desired.get("permission", "workspace")
        model = desired.get("model", "")
        wanted_skills = list(desired.get("skills") or [n for n in skill_ids])

        if name not in existing:
            actions.append(f"agent create: {name}")
            log(f"[agent] {name}: absent -> create")
            if dry:
                continue
            created = cli(
                [
                    "agent", "create",
                    "--name", name,
                    "--description", description,
                    "--instructions", instructions,
                    "--runtime-id", runtime_id,
                    "--max-concurrent-tasks", str(concurrency),
                    "--permission-mode", "public_to" if permission == "workspace" else "private",
                    *(["--public-to-workspace"] if permission == "workspace" else []),
                    *(["--model", model] if model else []),
                    "--output", "json",
                ],
                env,
            )
            agent_id = created["id"]
            ids = [skill_ids[s] for s in wanted_skills]
            actions.append(f"agent skills set: {name} -> {','.join(wanted_skills)}")
            cli(["agent", "skills", "set", agent_id, "--skill-ids", ",".join(ids), "--output", "json"], env)
            continue

        current = cli(["agent", "get", existing[name]["id"], "--output", "json"], env)
        agent_id = current["id"]
        drifted: list[str] = []
        update: list[str] = []

        if (current.get("description") or "") != description:
            drifted.append("description")
            update += ["--description", description]
        if norm(current.get("instructions") or "") != norm(instructions):
            drifted.append("instructions")
            update += ["--instructions", instructions]
        if current.get("max_concurrent_tasks") != concurrency:
            drifted.append(f"max_concurrent_tasks {current.get('max_concurrent_tasks')}->{concurrency}")
            update += ["--max-concurrent-tasks", str(concurrency)]
        if (current.get("model") or "") != model:
            drifted.append(f"model {current.get('model')!r}->{model!r}")
            update += ["--model", model]
        if not desired_invocation(
            permission, workspace_id, current.get("permission_mode"), current.get("invocation_targets")
        ):
            drifted.append(f"permission->{permission}")
            update += ["--permission-mode", "public_to" if permission == "workspace" else "private"]
            if permission == "workspace":
                update.append("--public-to-workspace")
        if current.get("runtime_id") != runtime_id:
            drifted.append(f"runtime {current.get('runtime_id')}->{runtime_id}")
            update += ["--runtime-id", runtime_id]

        if drifted:
            actions.append(f"agent update: {name} ({'; '.join(drifted)})")
            log(f"[agent] {name}: drift in {'; '.join(drifted)} -> update")
            if not dry:
                cli(["agent", "update", agent_id, *update, "--output", "json"], env)
        else:
            log(f"[agent] {name}: converged")

        bound = [s["id"] for s in cli(["agent", "skills", "list", agent_id, "--output", "json"], env)]
        wanted = [skill_ids[s] for s in wanted_skills]
        if sorted(bound) != sorted(wanted):
            names = ", ".join(wanted_skills)
            actions.append(f"agent skills set: {name} -> {names}")
            log(f"[agent] {name}: skill bindings differ -> set [{names}]")
            if not dry:
                cli(
                    ["agent", "skills", "set", agent_id, "--skill-ids", ",".join(wanted), "--output", "json"],
                    env,
                )


# ─── entry point ────────────────────────────────────────────────────────────


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--roster", required=True, help="path to roster.yml")
    parser.add_argument(
        "--dry-run", action="store_true",
        help="report the drift that would be reconciled, and change nothing",
    )
    opts = parser.parse_args()

    actions: list[str] = []
    try:
        roster_path = Path(opts.roster).resolve()
        base = roster_path.parent
        roster = yaml.safe_load(read_text(roster_path, "roster"))

        env = dict(os.environ)
        workspace_id = resolve_workspace(env, roster["workspace"])
        env["MULTICA_WORKSPACE_ID"] = workspace_id
        runtime_id = resolve_runtime(env, roster["runtime"])
        log(f"[resolve] workspace {roster['workspace']} = {workspace_id}")
        log(f"[resolve] runtime {roster['runtime']} = {runtime_id}")

        skill_ids = reconcile_skills(roster, base, env, opts.dry_run, actions)
        reconcile_agents(roster, base, env, workspace_id, runtime_id, skill_ids, opts.dry_run, actions)
    except Failure as exc:
        print(json.dumps({"changed": False, "actions": actions, "errors": [str(exc)]}))
        log(f"[ERROR] {exc}")
        return 1

    print(json.dumps({"changed": bool(actions), "actions": actions, "errors": []}))
    log(f"[{ 'DRY RUN' if opts.dry_run else 'APPLY' }] {len(actions)} action(s)"
        + ("" if actions else " — roster already converged"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
