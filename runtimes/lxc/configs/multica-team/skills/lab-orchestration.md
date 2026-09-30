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
  cannot be un-blocked. A stop *for the operator* is also made **findable**: the issue goes
  back to the operator and the comment opens with the `WAITING FOR OPERATOR:` line
  (section 3), because a question the operator never sees is the same as no question.
- **One comment per run.** The final comment *is* the result. Progress chatter costs a
  run's worth of context and tells the reader nothing. *How* that comment has to read —
  one line, the block that says what the operator owes, then the detail — is section 7, and
  it binds every message an agent posts, this one included.

What can start a run:

| Trigger | Effect |
| --- | --- |
| Assignment of an issue to an agent | one run, at creation or reassignment |
| A wakeup rule on the issue | a new ordinary run when its condition is met |
| A human comment or @-mention | the assigned **agent** (or the mentioned one) is woken; a member assignee is reached by neither, which is why handing an issue to the operator is a *loan* (section 3, rule 3) |

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
  member-assigned parent is never woken *as a run* — the platform files an inbox item for
  that member instead (`resolveWakeTarget` → `outcome: notified`) and starts no agent.
  Delegated work must therefore be filed under an issue whose assignee is an agent, and an
  issue with open children under it must not be handed to the operator (section 3, rule 3).
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
from parts that do exist, and it is only as strong as the four rules below.

**The gate = an issue held in `in_review`, assigned to the operator, plus an armed
actor-filtered wakeup, plus the instruction not to act before the approval comment, plus a
comment whose first line says a decision is owed and whose second landmark says what the
decision is.**

The statuses are `backlog`, `todo`, `in_progress`, `in_review`, `done`, `blocked`,
`cancelled`, grouped into four lifecycle categories, and the category — not the label —
carries the behaviour:

| Category | Statuses | Behaviour that matters here |
| --- | --- | --- |
| `unstarted` | `backlog`, `todo` | open work |
| `started` | `in_progress`, `in_review`, `blocked` | **open**; wakeups stay enabled |
| `done` | `done` | **terminal**; all wakeup configurations are disabled |
| `closed` | `cancelled` | **terminal**; all wakeup configurations are disabled |

Hence the four rules:

1. **An issue waiting for a human is `in_review`, never `done`.** `in_review` is a
   *started* status: the issue is still open, the wakeup stays armed, and the agent can
   be woken by the answer. Moving it to `done` or `cancelled` disables every wakeup on it
   and the answer arrives with nobody listening. "Done is not an intermediate step toward
   Closed", and it is not a parking spot either.
   **The failure mode is real and it is the agent's own last command.** A run that arms its
   gate and then closes with the habitual `multica issue status <id> done --no-start` has
   disarmed the gate it just built, in the same run, silently — the issue looks complete and
   the operator's approval is delivered to nobody. A gate run's last command is `in_review`;
   `done` belongs to a release that has been executed and verified live.
2. **The gate is only real if the instruction is explicit.** The agent that reaches the
   gate writes, on the issue: the exact action it will take, the command, the expected
   effect, the rollback, and the sentence *"I will not run this until the operator
   comments on this issue."* Then it arms the wakeup and ends the run. An agent that
   merely mentions "awaiting approval" has not built a gate, it has written a wish.
3. **An issue waiting for a human is assigned to that human.** A gate the operator cannot
   find is not a gate. Before the run ends, the agent that stopped hands the issue over:

   ```sh
   multica issue assign <issue-id> --to roberto.tazzoli@gmail.com
   ```

   The address is the operator account this team is owned by (`lab-team`), and `--to`
   resolves a member by name or email — never by `user_id`, which is a control-plane
   detail a rebuild would invalidate. The assignment is what puts the issue in the
   operator's **default board tab** (the `assignee_id` tab), so the pending decisions are
   where the operator already looks instead of something they have to search for. Four
   properties make it safe to do at the gate:

   - **It starts nothing.** A run is enqueued for an `agent` or `squad` assignee only
     (`WillEnqueueRun`); a member assignment records ownership and wakes nobody, so the
     agent that armed the gate does not wake itself with its own handoff.
   - **It does not disarm the gate.** The written rule stays armed: wakeups are disabled
     only when the issue moves to a terminal status (`StopClosedIssueWakeups`), and when
     the rule fires it starts a run for the agent it was armed for, whatever the issue's
     current assignee is. That is why the answer still reaches the agent that asked.
   - **It is idempotent.** An issue already assigned to the operator is left as it is;
     never bounce an issue back and forth to "refresh" the assignment.
   - **The handover is a loan, and the answer takes it back.** The comment→assignee
     routing wakes the issue's assigned **agent** only (`assigneeFallbackAgent` refuses a
     member assignee), so while the issue sits in the operator's hands a second comment on
     it reaches no run — unlike an issue assigned to an agent, where a follow-up question
     wakes its owner. The run that the approval starts therefore takes the issue back
     before it acts, and it takes it back without waking itself a second time:

     ```sh
     multica issue assign <issue-id> --to "<your agent name>" --no-start
     ```

     `--no-start` is not optional here: assigning an **agent** does enqueue a run, and that
     run would race the one already executing. If the resumed run reaches a new gate, the
     issue goes back to the operator, and it stays there until the answer arrives.

   One issue must **not** be handed over: **the parent that is still collecting delegated
   work.** The system `child_done` rule resolves its target when it fires, and a parent
   assigned to a *member* produces an inbox notification for that person and starts no run
   (section 2, "the parent must be an agent"). An orchestrator whose own issue is waiting on
   a stage would therefore stop being woken. Hand over the leaf that is waiting — the
   specialist's issue, Release's issue — never the parent with open children under it.

4. **The gate comment opens with a line anyone can recognise.** The first line of the
   comment is exactly

   ```
   WAITING FOR OPERATOR: <what is needed, in one sentence>
   ```

   followed by the detail of rule 2. The line exists so that the stop is legible while
   scanning the issue and its activity — before the comment is opened, and before the plan
   below it is read — and so that a board with ten issues on it shows at a glance which of
   them is the operator's move. It is a claim, not a decoration: it appears only on a run
   that has actually stopped with a gate armed, and the sentence after the colon names the
   decision the operator has to make ("approve the push of `restyle/teaching-web:master`",
   "say whether the new page replaces the old one"), never a summary of the work already
   done. A completion comment, a handoff to another agent and a failure the operator has
   no decision in do **not** carry the line — a signal that appears everywhere is not a
   signal.

   The line is the *scanner's* half of the gate: it says that there is a wait and on what,
   and it carries neither the options nor the consequences. The *reader's* half is the
   summary block of section 7.2, and section 7.3 states how the two relate — one
   question, stated once by each landmark, in the same words.

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
- **A handoff obeys section 7 like every other message.** The blocks are a budget, not a
  floor: each one carries what the receiver needs to act and nothing more. An issue
  description that has to be scrolled to reach `## Boundaries` is one the receiver will
  skim — and the part they skim is the boundary.

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
  `in_review` with the issue handed back to the operator — the same gate as section 3, used
  for a different reason, so it carries the same obligations: `in_review`, the issue assigned
  to the operator, the wakeup re-armed where the answer is what unblocks the work, and a
  first line of `WAITING FOR OPERATOR: <the decision>`. Name the decision the operator has to
  make; do not hand over the problem.
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
         posts the plan, arms the wakeup, hands the issue to the operator, stops
                                            → assignee: the operator, issue: in_review
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
- **The plan leads with the plan, and closes with the block.** Release's plan carries the gate
  sentence (section 3, rule 4) and, as its **last** thing, the block of section 7.2 — so the
  operator reads the whole chain in the order it executes and finishes on the question, with
  `IF NO:` as the final line of the comment.
- **Afterwards, the record moves, and Release is the one who moves it.** The wiki page, the
  memory entry and the parent issue are updated by their owners — not written by Release,
  who only reports what it observed — so once the release is verified, Release **opens those
  issues and assigns them** rather than reporting that they are now due (section 7.4.2).
  "Afterwards the record moves" is a delegation with an owner, not a note for the operator.

## 7. How a run writes: what changed, the detail, then the ask

Sections 1–6 say *what* a run has to say. This one says how it has to **read**, because a
message that is precise and unreadable has still failed. The operator, who reads all of them,
put it plainly: *"being precise is fine, but often I cannot tell what you are asking me."*
Four rules answer that, and they bind every message an agent posts — the run's final comment,
the gate comment, the description of a handoff.

### 7.1 The shape of the message: one line, the detail, then the block

The order is fixed, and it is the reader's order — evidence first, question last:

```
WAITING FOR OPERATOR: <the decision>     ← line 1, and only on a run that is at the gate
<one line: what changed, and what is now waiting>

<the detail: files, commands, evidence, what was left undone>

ASK:        <what this run wants now, in the plainest words available>
YOUR MOVE:  <accept | refuse | choose: A / B — recommend A>
IF YES:     <one line: what happens if the operator accepts, or takes the recommended option>
IF NO:      <one line: what happens if the operator refuses, or takes the other one>
```

- **The one line answers "what happened".** One line, the result — what was produced, and
  where it now stands. It is the line that survives being skimmed on a phone.
- **The block is last, and nothing follows it.** The message ends on the question: the last line
  written is `IF NO:`. The detail is what the reader falls back to *in order to understand* the
  ask, and a block stranded in the middle of a report is a block the operator has to notice is
  there. If there is something else to say, it goes **above** the block.
- **The detail is a section, not the message.** Keep it to what a reader would need in order
  to act and to check: the files touched, the commands and their real output, what was
  verified and what was not, what is still open. **Target ~800 characters** for the whole
  detail section. Go past it only when the facts genuinely do not fit — a release plan, a
  security finding, a design with three options — and then say *why* it is long, in one
  line at the top of the section. A long message is a legitimate answer to a large amount of
  news; it is not a legitimate answer to a small amount.
- **Short is a requirement, not a courtesy.** Precision is not length: `411 passed in 52.50 s`
  is one line, and it is more precise than a paragraph about the tests. The test is the
  reader, on one pass: after reading once, the operator can say what is being asked of them.
- **Result first; the journey is not part of the result.** The attempts, the dead ends and
  the files re-read along the way are not the operator's business — unless one of them
  changes the decision, and then it is one line.
- **Point at artifacts; do not paste them.** Section 4 gives the reason for a handoff (a
  pasted copy is stale the moment it is pasted) and it holds for the detail section too. The
  same goes for evidence: the command and its output, not a retelling of it.
- **Say a thing once.** An unverified claim, a boundary left uncrossed, a part of the
  deliverable not done — each is a line, stated plainly. Repetition is not emphasis.

### 7.2 The block — mandatory, in the four lines it is

The block above carries only the ask, the operator's move and the two outcomes. Four rules
bind its content:
- **It is a summary, not a conclusion.** The detail section above carries the work, the
  evidence and the plan; the block carries only the ask, the move and the two outcomes. A block
  that restates the body has failed at being a summary.
- **"Nothing to ask" is stated, not omitted.** A run that delivered and needs nothing still
  writes the block — `ASK: nothing` and `YOUR MOVE: none` — so the reader sees at a glance that
  the agent is not waiting on anything. A missing block is indistinguishable from a run that
  forgot to say what it wanted, which is the whole problem this section exists to fix.
- **Options come with a recommendation**, and one line of why. A menu without the agent's own
  view hands the thinking back to the operator, which is the opposite of what a specialist is
  for.
- **`IF YES` and `IF NO` are dropped only when the outcome is genuinely self-evident from the
  ask.** A gate with a blast radius always carries both: *what happens next* is what the
  operator is deciding about.

### 7.3 The opening line and the block: one question, two landmarks

A gate comment carries two landmarks, and they are not two questions:

| Landmark | Who reads it | What it carries |
| --- | --- | --- |
| `WAITING FOR OPERATOR: <the decision>` — the comment's **first** line (section 3, rule 4) | the operator scanning the board and the issue's activity, before any comment is opened | *that* there is a wait, and on what |
| the block (section 7.2) — the comment's **closing** landmark, below the detail | the operator who has opened the comment and has read to the end | *what to do about it*: accept, refuse or choose — and what each answer does |

They are kept distinct on purpose, and the rule that stops them from colliding is this:
**the block's `ASK:` line is the opening line's sentence, verbatim and minus the marker.** One
request, written once and shown twice — at the top, where a scanner sees it without opening
the comment; at the bottom, where the reader arrives having read the evidence and is deciding.
A paraphrase in the block is exactly what makes a comment look like it is asking two things; a
shared sentence makes it look like what it is.

Nothing else is repeated. The block does not summarise the detail, the opening line carries
neither the options nor the outcomes, and the detail section is touched by neither.

A run that is not waiting on the operator carries no opening line (a signal that appears
everywhere is not a signal) and still writes the block.

### 7.4 Three rules that keep the flow moving

These three are about what an agent does between messages, not about how a message reads. They
were added after the CV run of 2026-09-29, where each one cost the operator a round trip.

**1. A question is asked once.** A question with no answer stays **open** — it does not get
reformulated. When a later run reaches the same unanswered question, it does not re-ask it: it
states, in one line, that the question from run *n* is still open, and leaves it at that. The
question text is written once, on the comment that asked it; the reminder carries a pointer,
not a paraphrase.

- On TAZLAB-22 the agent asked at 20:08 «approvi il prototipo CV, pagina HTML o PDF?» and
  asked the same thing again, in the same words, at 20:32. Nothing had happened in between to
  change the question, so the second one was pure cost.
- **The test is whether anything changed.** New evidence, a new option, a changed artifact —
  any of those is a reason to write again, and then the new text says what changed. "No news"
  is not a reason.
- **A reminder is one line, once.** One mention of the standing question per run is enough.
  A second reminder in the same issue is nagging, and nagging is what a duplicated question
  looks like from the reader's side.

**2. Whoever finishes, delegates.** The moment an agent's own work is done and a known next
step exists — a release, a correction in another domain, a documentation update — **it opens
the issue and assigns it**, in the same run, and then says that it did. Coordination is work
like any other, and work that stops at the agent's own boundary is work the operator ends up
doing.

- Writing *"for publication you need a Release issue"*, or *"say the word and I will open it"*,
  or *"it needs an issue to the Job agent — unopened until you ask"*, is the failure. Those
  are three ways of handing the operator a task the agent was in a position to do itself.
- The test: **after the run, does the next step exist as an issue with an assignee?** If the
  answer is no, the run has not finished its job — whatever the status field says.
- The reverse also holds: **never delegate what you can settle yourself.** A question the
  operator asked *you* to answer directly is your work, not an issue.

**3. An announcement is not a request.** The block of section 7.2 is the only place a request
to the operator is written. Anything that merely *reports the state of the flow* — a handoff
opened, a gate reached, a release in progress, a sub-issue closed — goes in the one-line
summary or the detail section, never inside `ASK:`.

- TAZLAB-22 wrote `ASK: nothing — the gate is passed, the release is with Release and its
  approval request will reach you`. That is three facts about the pipeline, and none of them
  is a request; the operator had to work out that the `nothing` was genuine rather than a
  soft-pedalled demand. The same information is one line in the summary: *TAZLAB-26 opened to
  Release; it will ask you for the push.*
- **`ASK: nothing` plus `YOUR MOVE: none` is a complete and honest block** when the run really
  has nothing to ask — and the surrounding message must then be pure status, with no implied
  obligation in it.

## 8. Standing rules (all agents)

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
- **A stop is visible.** An issue waiting for the operator is assigned to the operator
  (`multica issue assign <id> --to roberto.tazzoli@gmail.com`), so it sits in the tab they
  read first, and the run's final comment opens with
  `WAITING FOR OPERATOR: <what is needed, in one sentence>`. Both halves are required: an
  issue nobody can find, with a reason written three paragraphs down, is a stall, not a
  handoff. The comment **ends** with the block of section 7.2 — the same sentence, plus what
  the operator is asked to do about it and what each answer does.
- **A message is: one line, the detail, then the block — and the block is the last thing
  written.** Section 7.1 fixes the order; the detail section is capped at ~800 characters unless
  the facts genuinely do not fit; and **zero characters follow the block**. The operator must not
  have to read twice to find the request, and must not have to hunt past a full report to reach
  it — and neither must they read to the end and keep scrolling to find it.
- **Ask once, delegate yourself, and never dress an announcement as a request.** Sections
  7.4.1–7.4.3: a question with no answer stays open and is pointed at, not repeated; a
  finished run that has a next step opens and assigns it; and the `ASK:` block carries
  requests only.
