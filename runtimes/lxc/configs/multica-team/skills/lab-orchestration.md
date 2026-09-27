# Skill: Lab Orchestration Protocol

## What this is

This is the shared rulebook for **how work travels** between the TazLab agents on
Multica. `skill://lab-team` says *who* does what (the roster of specialists and their
domains); this skill says *how* a piece of work moves from a goal to something live, who
is allowed to move it, where a human has to say yes, and what happens when a run fails.

They are read together: `lab-team` for the map, this skill for the road. When the two
disagree about a rule of movement, this skill wins; when they disagree about ownership,
`lab-team` wins. Both lose to the live system — read the repository, the cluster and the
control plane before you act from memory.

The protocol exists because the agents of this lab are **single-concurrency workers with
no shared memory**. Every rule below compensates for one of those two facts. Concurrency
1 means a handoff is a *wakeup*, not a call. No shared memory means the issue is the only
channel that survives — if it is not written on the issue, it did not happen.

## The shape of the work

```
   operator (the only human)
      │  a goal
      ▼
   Ambrogio  ──opens──▶  parent issue          ← the blackboard: context, evidence, decision
      ▲                      │
      │                      ├─ stage 1 ─▶ specialist A  ──┐
      │                      │                             │  each sub-issue is a
      │                      ├─ stage 2 ─▶ specialist B  ──┤  run of exactly one agent
      │                      │                             │
      │  child_done wakeup ◀──┴─────────────────────────────┘
      │                       "stage 1 finished" / "everything finished"
      │
      │  change ready, committed, NOT published
      ▼
   Release  ──▶  in_review  +  armed "when the operator comments" wakeup   ← THE GATE
                        │
                        │  the operator answers on the issue
                        ▼
                    push / merge → pipeline → image automation → deploy
                        │
                        ▼
                    verify on the LIVE surface, paste the evidence, close
```

Nothing moves outside the boxes. There is no side channel: no chat, no shell on the
host, no assumption that the other agent remembers.

## 1. The ladder: issue → sub-issue → stage

An **issue** is the atomic unit of work and the only durable channel between agents. It
carries the request, the runs, the evidence and the decision record. Everything below is
a property of that record.

- A **sub-issue** is created with `--parent <issue-id>`. Use it whenever the work needs a
  different agent than the parent's assignee, or a different acceptance criterion.
- A **stage** is the ordinal on that sub-issue: `multica issue create --parent <id>
  --stage <n>` (`n >= 1`). The platform documents the semantics exactly: *"Stage ordinal
  (>=1) grouping this sub-issue into an ordered barrier group under its parent; omit for
  unstaged. The parent assignee is woken only when every sub-issue in a stage finishes."*
- `multica issue children <parent-id>` lists the sub-issues grouped by stage.

Rules that keep stages meaningful:

- **A stage is a barrier, not a label.** Because the wakeup fires only when *every*
  sub-issue of the stage is closed, a stage of two parallel sub-issues is a legitimate
  "both of these, then continue" — and a stage of one sub-issue that will never close is
  a deadlock. If you cannot name what closes the stage, do not open it.
- **Stage order is the dependency order.** Stage *n+1* is opened only when stage *n* is
  green; that is what the wakeup at the barrier is for.
- **A cancelled sub-issue does not block the barrier but is not finished work.** The
  woken parent decides whether the cancelled work is still needed — which is exactly what
  the platform's own instruction to the woken assignee says.
- **Do not stage what is not sequential.** Independent work goes to whichever agent owns
  it, in parallel, not into an artificial ladder.

## 2. Wakeups: how a run starts

A run starts when someone or something asks for it. There is no steering mid-run: a
comment written while a run is executing does **not** reach that run — it becomes an
input to a later run. The consequences are hard rules:

- **State the blocker and end the run.** An agent that needs a decision posts its
  question as its final comment and stops. Waiting inside a run cannot be observed and
  cannot be un-blocked.
- **One comment per run.** The final comment *is* the result. Progress chatter costs a
  run's worth of context and tells the reader nothing.

What can start a run:

| Trigger | Effect |
| --- | --- |
| Assignment of an issue to an agent | one run, at creation or reassignment |
| A wakeup rule on the issue | a new ordinary run when its condition is met |
| A human comment or @-mention | the assigned agent (or the mentioned one) is woken |

The kinds available to an agent are exactly the flags the **installed** CLI accepts
(`multica issue wakeup create --help`): event subscriptions (`--event <type>`, with
`--filter-actor-type member|agent --filter-actor-id <uuid>`, `--task-id` or
`--filter-agent-id` for run events), timers (`--kind at --after <dur>` / `--at <rfc3339>`,
`--kind every --every <dur>`, `--kind cron --cron '<expr>' --timezone <tz>`), `--mode
once|continuous` and `--parent <comment-id>` for result delivery. Two properties matter
here:

- **A wakeup is prospective.** It observes facts from *after* it was armed. Arm it before
  you stop, or the event you are waiting for will happen unwatched.
- **A person can be the trigger.** `--event comment.created --filter-actor-type member
  --filter-actor-id <operator-user-id>` waits for *that member* to write on the issue.
  This is the platform primitive the approval gate in section 3 is built on.

**Use only what the installed version has.** The upstream checkout documents a richer
wakeup surface — condition waits (`--until-status`, `--until-pr`, `--until-children-done`,
`--until-issue`), `--max-fires`, deadlines (`--expires-in`, `--on-timeout`) and the
`wakeup runs|trigger|checkin` subcommands — but **none of it is in the version this lab
runs** (`multica version` → 0.5.3; its `wakeup` subtree is `create|disable|events|get|list|
update`). A protocol step that names a flag the CLI rejects is not a step. When the control
plane is upgraded, re-read `multica issue wakeup create --help` and extend this section;
do not assume. Equivalents on 0.5.3: a condition wait becomes an event subscription plus the
standing rule that the woken run re-reads current state; a deadline becomes a timer
(`--kind at`) whose instruction says what to do when it fires and the human gate is still
closed.

**The operator's identity is resolved at run time, never hardcoded.** The approval wakeup
is armed once per **owner** account, because the operator has two logins in this workspace
and either one may be the one that answers:

```sh
multica workspace member list --output json \
  | jq -r '.[] | select(.role == "owner") | .user_id'
```

Each id becomes its own `--event comment.created --filter-actor-type member
--filter-actor-id <id>` rule (`multica workspace member list` is the authority; a rebuild
does not invalidate a `user_id`, but a new operator account must be added to the gate).
Without a filter the rule wakes on *any* comment, including a peer agent's — which would
let a machine open a gate that only a human may open.

The **stage-completion wakeup** is the other half: it wakes the *parent's assignee* when
every sub-issue of a stage has reached a terminal status, and once more when every
sub-issue is closed. An agent does not arm it and it has no stored rule on 0.5.3: the
platform **tries once** to deliver it, as a comment that @-mentions the parent's assignee,
and a failed delivery is not replayed. Three consequences to design around:

- **The parent must be an agent.** The wake is delivered as `mention://agent/<id>`; a
  member-assigned parent is never notified (a member mention enqueues nothing). Delegated
  work must therefore be filed under an issue whose assignee is an agent.
- **It is best-effort, and 0.5.3 fires it even on an already-closed parent.** A delegated
  issue that must come back cannot rely on it alone — that is why section 4 requires the
  specialist to close its own issue *and* report, and why the parent re-reads the children
  with `multica issue children <id>` rather than trusting that it was woken.
- **A cancelled sub-issue closes the barrier but is not finished work.** The woken parent
  decides whether the cancelled work is still needed.

**The rule that makes delegation return.** An open sub-issue wakes nobody. Every
delegation therefore ends with an explicit request: *move your issue to `done`
(`multica issue status <id> done --no-start`) once your final comment is posted.* A
specialist that leaves its issue open has silently swallowed the work.

## 3. The human gate

**There is no approval object in Multica.** Nothing in the product blocks an action on a
human signature: agents are trusted to follow their instructions, and the platform's own
model of "done" is a status, not a verification. The gate is therefore a *protocol*, built
from parts that do exist, and it is only as strong as the two rules below.

**The gate = an issue held in `in_review`, plus an armed actor-filtered wakeup, plus the
instruction not to act before the approval comment.**

The statuses are `backlog`, `todo`, `in_progress`, `in_review`, `done`, `blocked`,
`cancelled`, grouped into four lifecycle categories, and the category — not the label —
carries the behaviour:

| Category | Statuses | Behaviour that matters here |
| --- | --- | --- |
| `unstarted` | `backlog`, `todo` | open work |
| `started` | `in_progress`, `in_review`, `blocked` | **open**; wakeups stay enabled |
| `done` | `done` | **terminal**; all wakeup configurations are disabled |
| `closed` | `cancelled` | **terminal**; all wakeup configurations are disabled |

Hence the two rules:

1. **An issue waiting for a human is `in_review`, never `done`.** `in_review` is a
   *started* status: the issue is still open, the wakeup stays armed, and the agent can
   be woken by the answer. Moving it to `done` or `cancelled` disables every wakeup on it
   and the answer arrives with nobody listening. "Done is not an intermediate step toward
   Closed", and it is not a parking spot either.
2. **The gate is only real if the instruction is explicit.** The agent that reaches the
   gate writes, on the issue: the exact action it will take, the command, the expected
   effect, the rollback, and the sentence *"I will not run this until the operator
   comments on this issue."* Then it arms the wakeup and ends the run. An agent that
   merely mentions "awaiting approval" has not built a gate, it has written a wish.

**Who may open the gate: an owner account, and only an owner account.** The wakeup is
armed per owner `user_id` (section 2), so a peer agent's comment cannot open it, and no
agent may answer another agent's gate — not with a comment, not with a status change, not
by re-running the gated issue itself. An agent that has the operator's authorization from
another channel records that authorization *on the issue* (what was authorized, when, in
what words) and lets the operator's own comment be the trigger; it does not forge the
trigger. If the operator approves in conversation rather than on the issue, the gate is
satisfied only once the approval is written on the issue by an owner account — that
comment is the audit record, and it is the thing the protocol is built to produce.

### What needs the gate

Split every action by blast radius, and gate the outward-facing half:

| Class | Examples | Rule |
| --- | --- | --- |
| **Read** | `kubectl get`, `git log`, `multica issue get`, opening a preview on a LAN port | always allowed |
| **Reversible, inside the lab** | a commit on a local branch, a file in the working tree, a `--check` render, a local build | allowed; report it |
| **Outward-facing or irreversible** | `git push`, a merge to a shared branch, an image tag a deployment consumes, a deploy, a DNS or certificate change, a credential rotation, a deletion | **gate: propose on an issue, wait for the operator's comment** |
| **Never** | writing a secret value anywhere, touching another project's resources, a destructive command on a shared database | not a gate — a boundary |

The distinction is not "how risky does it feel" but **"can a third party observe it, and
can it be undone without asking anyone"**. A commit is private until it is pushed. A push
is public the moment it lands. The lab's rule is therefore the simple one: *the branch is
the lab's, the remote is the world's.*

## 4. The handoff contract

A handoff is a new issue to another agent. Nothing travels with it except what is written
on it — the receiving agent starts with **no memory** of the conversation that produced
it. Every handoff therefore carries four blocks, in this order:

```
## Context      what the operator asked, why now, what was already decided
## Deliverable  the exact artifact(s) and where they must live (repo, path, branch, page)
## Acceptance   how the result is judged — the command or observation that proves it
## Boundaries   what must not be touched, and which gate applies
```

Two more blocks are mandatory when the handoff crosses the **release boundary** (section
6), because there the receiver acts on the world:

```
## Source       repository path, branch, commit SHA, remote name
## Release plan the exact command, the chain it triggers, the rollback, the evidence to bring back
```

Rules for the handoff:

- **Name the assignee by name, never by id.** Resolve it at run time
  (`multica agent list --output json`); ids are not portable across a rebuild.
- **The assignment itself wakes the receiver.** Do not also @-mention it for the same
  thing — that is a second run for one deliverable.
- **Point at artifacts, do not paste them.** The receiver reads the repository, the
  branch and the commit. A pasted copy is a stale copy the moment it is pasted.
- **Evidence travels, conclusions do not.** `411 passed in 52.50 s`, `scrollWidth ==
  clientWidth at 390 px`, the byte size of the PDF — those are facts the receiver can
  check. "Looks good" is not.
- **Say what you did not verify.** An unverified claim that the receiver believes is the
  most expensive thing a handoff can carry.

### The context that does not fit in an issue

Long context has three homes, and only one of them is the transcript:

1. **The issue** — decisions, acceptance, evidence, the gate. Durable, reviewable, small.
2. **The artifact** — the commit, the file, the manifest. Authoritative, and it is what
   the next agent reads.
3. **The lab's memory** — `skill://memory` and the wiki, for facts that outlive the
   issue. The **Memory** agent owns it; a decision worth remembering is handed to Memory,
   not buried in a closed issue.

Never let a run's transcript be the only copy of anything.

## 5. Failure, retry, escalation

Runs fail: a test breaks, a command exits non-zero, a pipeline goes red, a wakeup is
missed. The protocol fixes what happens next so that failure is legible instead of silent.

- **Fail loudly on the issue.** A failed run ends with a comment that names the exact
  command, the observed error, and whether the world changed before it failed. A silent
  failure leaves a sub-issue open and the parent never wakes.
- **Retry budget: two attempts for a reversible action, zero for an outward-facing one.**
  A failing local build may be fixed and retried in the same run. A failed push, merge or
  deploy is never blindly retried — the first thing to establish is whether it *landed*
  (`git ls-remote`, `kubectl get`, the pipeline's own record). Retrying an action that
  already succeeded is how a fast-forward becomes a force-push.
- **A retry is idempotent or it is not a retry.** Re-running a build is safe; re-running a
  push is safe only because `git` refuses a non-fast-forward. Before re-running anything
  outward-facing, ask what a second execution would do to a world where the first one
  worked.
- **Compensate, do not just stop.** An action that went live and then failed verification
  is undone deterministically: the deploy is rolled back to the previous image tag, the
  merge is reverted with a revert commit (never a history rewrite), the branch is left
  as it was. State the compensation in the failure comment even when it was not needed.
- **Escalate to the operator when the budget is spent, or when the blast radius is
  larger than the issue.** Escalation is a comment on the issue plus the issue returned to
  `in_review` — the same gate as section 3, used for a different reason. Name the decision
  the operator has to make; do not hand over the problem.
- **Never work around a failure by weakening the check.** If a test blocks a release, the
  release waits. Editing the test to make it pass is not a failure-handling strategy, it
  is a defect with a nicer commit message.

## 6. The release lane

Publishing is a **separate handoff to a separate agent**: the **Release** agent. This is
deliberate, and it is the maker–checker split: the agent that produced a change is the
worst-placed agent to be the only one that ships it, and the agent that ships it should
have no opinion about the content.

The lane, end to end:

```
specialist: commits on a branch, verifies locally, stops          → issue: in_review
   ↓ handoff (stage, new issue, assignee Release, --parent)
Release: re-derives the state (branch, commit, clean tree, pipeline, rollback)
         posts the release plan, arms the wakeup, stops            → issue: in_review
   ↓ the operator comments approval on the issue
Release: executes the exact approved command
         verifies on the live surface, pastes the evidence
         states the rollback                                          → issue: done
   ↓ child_done
the parent's assignee reports to the operator
```

Rules for the lane:

- **Release re-derives, it does not trust.** The branch, the SHA, the working tree, the
  remote's real state (`git ls-remote`), the workflow that will fire, and the automation
  that consumes the image are all checked from the live system before the plan is written.
  A stale SHA in a handoff is the normal case, not the exception.
- **What is approved is what is executed.** If the operator approves "push
  `restyle/teaching-web:master`", that is the command that runs. A different command, or
  an extra step, is a new gate.
- **Verification is on the live surface.** "The pipeline is green" is not "the change is
  live". The evidence is the tag the deployment actually runs, the response of the live
  URL, the content the browser receives.
- **The rollback is stated before the push, not after the failure.**
- **Afterwards, the record moves.** The wiki page, the memory entry and the parent issue
  are updated by their owners — not by Release, who only reports what it observed.

## 7. Standing rules (all agents)

- **Work moves on issues only.** Chat with the operator is conversation; an issue is a
  commitment. Anything another agent must act on is an issue.
- **English for artifacts** (issues, comments, code, docs, manifests); Italian only for
  product content addressed to Italian readers and for the operator's conversational
  replies.
- **Never a secret in text.** Name the gopass entry or the Vault path; never the value.
- **Propose, do not perform, the irreversible.** Sections 3 and 6.
- **The truth is the live system.** Read the repository, the cluster and the control plane
  before acting from a document that may have moved.
- **The operator is the only approver.** No agent may approve another agent's gate. A
  gate is answered by a human comment, not by a peer's agreement.
