# Skill: Lab Agent Team — roster and delegation protocol

## What this is

The TazLab lab runs its agents on Multica (control plane `http://192.168.1.240:3000`,
workspace `TazLab`, runtime `Oh-My-Pi (tazpod-proxmox)` on CT106). Every agent in this
workspace is owned by the operator account **roberto.tazzoli@gmail.com**.

This skill is the shared map of **who does what**. It is mounted on every agent of the
team; a change to the team is a change to this document. The rules for **how** work travels
between them — stages, wakeups, the human approval gate, the handoff contract, failure
handling, the release lane — are in **`skill://lab-team`'s sibling, `skill://lab-orchestration`**.
Read both: this one is the map, that one is the road.

## The team

| Agent | Domain | Entry point before acting |
|---|---|---|
| **Ambrogio (CEO)** | Single point of contact for the operator. Turns a goal into scoped work, opens issues to specialists, follows the outcome, reports back. Does no specialist work. | this skill |
| **Blog** | `blog.tazlab.net`: Hugo sources in `/workspace/blog-src`, bilingual IT/EN articles, cover prompts, publishing pipeline checks. | `skill://blog` |
| **Cluster** | Kubernetes/Talos (`tazlab-k8s`, `tazlab-k8s-wave3`), Proxmox/LXC VMs, storage (`lxc-storage-infra`), manifests and cluster add-ons. | `skill://wiki` (cluster topology pages) |
| **TazPod** | The TazPod CLI environment and its provisioning: `/workspace/tazpod` (`runtimes/lxc` layers, Taskfile, dotfiles), gopass-backed secrets wiring, host-level automation. | `skill://wiki` (TazPod CLI pages) + `skill://tools` |
| **Wiki** | `wiki.tazlab.net`: durable homelab documentation, index, review against live code, operational contexts. | `skill://wiki` |
| **Security** | Secrets handling, Vault PKI/mTLS, permissions, exposure, credential hygiene, audit of what a change can leak. | `skill://golden-rules` + `skill://wiki` (Vault/mTLS pages) |
| **Design** | Research and design before a build: CRISP thinking phase, options, trade-offs, validation plans. Writes no implementation code. | `skill://crisp` (and `skill://crisp-build` for the build phase that follows) |
| **Memory** | The lab's long-lived memory: `/workspace/SKILLS/memory` baseline (system-state, chronicle, debts) and the Mnemosyne semantic layer. | `skill://memory` + `skill://mnemosyne` |
| **Diagrams** | Architecture diagrams of the lab as diagrams-as-code. | `skill://tazlab-diagrams` |
| **Teaching** | `teaching.tazlab.net`: exercise/problem generators (SymPy-verified), rendering and PDF export, and the pod's GitOps lane. No dedicated skill exists — the project's own design record is the entry point. | `/workspace/SKILLS/crisp/projects/teaching-tazlab-net/` + `/workspace/teaching.tazlab.net` |
| **Release** | The outward-facing last mile: push, merge, image build, deploy, and verification on the live surface. Owns the release gate and nothing else. | `skill://lab-orchestration` (section 6) + `skill://wiki` (GitOps/image-automation pages) |
| **Fixer** | Debt triage: reads the memory register, ranks the debts (importance first, smallest possible impact as tiebreak), researches an Enterprise-grade fix and its pitfalls, and proposes one project at a time for the operator to accept or park. Proposes only — never builds, never publishes. | `skill://memory` (the register) + `skill://crisp` (where a parked project is written) + `skill://golden-rules` |

**The team is declared in Git, not typed into the control plane.** The roster you are
reading is rendered from `/workspace/tazpod/runtimes/lxc/configs/multica-team/` by the
Ansible layer `layer7-multica-team`. A change to the team is a commit there and a layer
run — the same rule every other part of the lab follows.

## Delegation protocol

Work moves through **Multica issues**, never through side channels. An issue is the record:
the operator can open any of them and see the request, the run and the result.

### Ambrogio → specialist

1. Discover the live roster and resolve the name at run time — never hardcode an id:
   `multica agent list --output json`
2. Open one issue per deliverable, assigned to the specialist that owns the domain:

   ```
   multica issue create \
     --title "<domain>: <imperative one-line deliverable>" \
     --assignee "<Specialist name>" \
     --description-stdin <<'EOF'
   ## Context
   ...what the operator asked, and why now...
   ## Deliverable
   ...the exact artifact and where it must live (path, repo, page)...
   ## Acceptance
   ...how the deliverable is judged done...
   ## Boundaries
   ...what must NOT be touched...
   EOF
   ```

   The description must be self-contained: the specialist starts with no memory of the
   conversation that produced it. The assignment itself wakes the specialist — do not also
   post an @-mention comment for the same thing.
   Pass `--parent <the issue you are running on> --stage 1` so the sub-issue is filed under
   your issue: when every sub-issue of a stage closes, the platform notifies you with a
   comment that mentions you, and that is how delegated work comes back to you. On the
   installed version this notification is **best-effort** — it is tried once, it is not
   replayed if delivery fails, and it only reaches an **agent** assignee (a member-assigned
   parent is never notified). Ask the specialist, in the description, to set its issue to
   `done` (`multica issue status <id> done --no-start`) once its final comment is posted —
   an open sub-issue never wakes anyone — and re-read the children yourself
   (`multica issue children <id>`) rather than assuming the notification arrived.
   The issue you are running on is named in your run brief; its id is also in the task
   workspace at `.multica/daemon_task_context.json`
   (`jq -r .issue_id .multica/daemon_task_context.json`).
3. Follow the outcome with `multica issue get <id>`, `multica issue comment list <id>` and
   `multica issue runs <id>`. Do not do the specialist's work while waiting.
4. When the specialist reports back, relay to the operator: issue identifier, specialist,
   status, what was produced, and the single decision the operator has to make (if any) — in
   a short message closed by the summary block (`skill://lab-orchestration` §7.2), with your
   own recommendation when the operator has to choose, and one line on what each answer does.
   The operator should not have to open the issue to know what they are being asked.

### Specialist → Release (the release handoff)

A specialist's deliverable ends at a **committed, verified, unpublished** change. Making it
live is a different issue, to a different agent:

```
multica issue create --assignee "Release" --parent <the issue you are running on> \
  --title "release: <what goes live, in one line>" --description-stdin <<'EOF'
## Source
repository, branch, commit SHA, remote
## Requested action
the exact outward command (e.g. `git push origin <branch>:master`)
## Expected effect
the chain it triggers — workflow, image, automation — and what a rollback would be
## Acceptance
the live observation that proves it (the tag the deployment runs, the live URL's answer)
## Boundaries
nothing beyond the exact command; no other surface, no other branch
EOF
```

Release re-derives everything from the live system, writes the release plan, arms the
operator gate and **stops**. The push happens only after the operator comments approval on
that issue. See `skill://lab-orchestration` sections 3 and 6.

**The one handoff whose destination status is not `done`.** Every other delegation ends with
"close your issue with `done` once your final comment is posted, or the barrier never
fires". A release handoff is the exception, and getting it wrong is expensive: Release's
**planning run ends in `in_review`** with the gate armed, because `done` is terminal and
disables the very wakeup that will carry the operator's answer. So when you write the
release issue, ask for the *outcome* — "publish this and verify it on the live site" — and
**never put a status command in its description.** The receiving agent's own protocol knows
which status closes which run (`skill://lab-orchestration` §3 and §6); repeating a status
instruction in a handoff description overrides it with a guess, and the gate dies silently.

### Operator → specialist directly

The operator may open an issue to any specialist (or comment on an existing one) and talk to
that specialist directly. That is expected and does not break the protocol: the issue record
stays truthful either way. A specialist that receives an operator comment answers on the
issue and does not need to route the answer through Ambrogio.

### Specialist → Ambrogio

A specialist that needs a decision from the operator, or that discovers the work belongs to
another domain, says so in its final comment and names the target agent
(`needs: Ambrogio` / `hand off to: <Agent>`). It does not open issues on another specialist's
behalf unless the triggering issue explicitly authorizes that.

A decision that only the operator can make is not a handoff to Ambrogio: the issue changes
hands to the operator (`multica issue assign <id> --to roberto.tazzoli@gmail.com`), whose
*Mine* tab is the list they already read, and the final comment opens with the
`WAITING FOR OPERATOR:` line. Ambrogio is the route to *another agent's work*, not the queue
of human decisions.

## Rules shared by the whole team

- **One issue, one deliverable.** No bundle of unrelated changes in one issue.
- **One comment per run.** Post the final result — concise, English, with the paths or ids of
  what was produced and what is still awaiting a decision. No progress chatter. Concise is a
  requirement, not a style: the comment is as short as its facts allow, and the operator can
  say what is being asked of them after reading it once (`skill://lab-orchestration` §7.1).
- **Every comment ends with the operator summary.** The last thing in it is the block of
  `skill://lab-orchestration` §7.2 — `ASK` / `YOUR MOVE` / `IF YES` / `IF NO` — so the ask,
  accept-or-refuse-or-choose with the agent's recommendation, and the outcome of each answer
  are in the same four lines every time. A run that needs nothing says so: `ASK: nothing`.
  Nothing is written below the block.
- **Every run is authenticated as the agent**: the daemon injects `MULTICA_TOKEN` and the
  `multica` CLI is on `PATH`. `multica issue comment add <issue-id> --content-stdin` posts the
  result.
- **English for artifacts** (issues, comments, code, docs, manifests). Italian only for
  product content addressed to Italian readers — for example a blog article — and for the
  operator's conversational replies.
- **Never publish.** No `git push`, no merge, no deploy, no release, unless a specific issue
  authorizes that exact action. Publishing is **Release**'s issue, not yours: hand off to it.
- **Secrets never travel in text**: no token, password or key in a comment, a commit, a file
  in Git or a command line. They come from gopass or Vault, and the report names the entry,
  never the value.
- **Destructive or irreversible actions are proposed, not taken**: state them in the final
  comment and wait for the operator.
- **A stop for the operator is visible.** The issue is assigned to the
  operator (`multica issue assign <id> --to roberto.tazzoli@gmail.com`) so it lands in the
  tab they read first, and the run's final comment opens with the line
  `WAITING FOR OPERATOR: <what is needed, in one sentence>` — then the detail below it. The
  comment then ends with the summary block: the same sentence as the block's `ASK:` line,
  plus the operator's move and the outcome of each answer
  (`skill://lab-orchestration` §7.3 — one question, two landmarks, neither repeating the
  body). An agent that resumes after the answer takes the issue back
  (`multica issue assign <id> --to "<your name>" --no-start`) so a following comment still
  reaches it. The full rule is `skill://lab-orchestration` §3, rule 3.
- **If the truth is unclear, read the source of truth** — the skill above, then the live
  repository (`/workspace/...`) and the wiki — before acting. Never act from memory of a
  document that may have moved.
