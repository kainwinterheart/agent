# Multi-Agent Orchestration System

A Go-based orchestration engine that turns high-level requests into reviewed,
validated code changes or evidence-grounded investigation reports.

Instead of a fixed pipeline, the system is **dynamic**: a single *workflow driver*
agent decides which short *subworkflow* to run next, based on the task and the
documents produced so far. Content agents communicate through **Markdown
documents on disk** — their contents are never parsed or schema-validated.
Control-flow decisions (the driver's choice, the loop decider's verdict) are
made by lightweight **decision agents** that respond in strictly validated
JSON (schema enforced at generation time and re-validated in the orchestrator).

---

## How It Works

1. You give the orchestrator a task description through stdin.
2. The **workflow driver** agent is invoked as a **multi-turn
   conversation**, one turn per driver iteration:
   - **Call 1 — read inputs**: a fresh LLM session (clean context) receives
     the driver's full state bundle — the original task, the full list of
     available subworkflows, the path to the artifact index (a
     system-maintained list of every document produced so far, grouped by
     subworkflow execution), and the driver's own decision history — and
     reads all the inputs. The runtime reports the session id of the call;
     the orchestrator keeps it **solely in memory** (it is never persisted
     to disk).
   - **Call 2 — analyze and save**: resuming that session, the driver
     analyzes the data with the intention of producing its decision and
     saves its complete analysis and reasoning to a pregenerated file under
     `analysis/`. The orchestrator verifies the file on disk and retries
     within the same session until it exists.
   - **Call 3 — respond**: resuming the same session, the driver returns
     the JSON decision. All retries (invalid JSON, missing analysis file)
     happen inside the session, so the driver keeps its reading and
     reasoning in context.
   If the runtime reports no session id, the turn degrades to
   self-contained one-shot calls (the pre-multi-turn behaviour).
3. The driver's decision is a **strictly validated JSON response**: which
   subworkflow to run next (or `finish`), why, and the task for that
   execution — the concrete goal that becomes the execution's section heading
   in the artifact index. The decision is written rationale-first, so the
   analysis precedes the transition. The orchestrator persists the validated
   decision under `decisions/`. The state bundle also states the authoritative run
   state (`RUN STATE: BOOTSTRAP` when no subworkflow has completed yet, so
   the driver's first decision is the `spec` subworkflow) and a
   `finish` in bootstrap state is rejected at validation time — the run
   cannot be complete before it has started — with the driver asked again.
4. The chosen **subworkflow** runs: a small fixed group of closely tied agents
   (a producer and its reviewers). Each agent writes its final output as a
   Markdown document to its own pregenerated, unique file path. The system
   validates only that the file exists and is non-empty; document contents are
   not parsed or schema-validated.
5. After each round, the **loop decider** agent is invoked as a **multi-turn
   conversation** (the same pattern as the driver, without the reassessment):
   a fresh session reads this execution's section of the artifact index and
   the round's documents; resuming that session, it analyzes the round and
   saves its analysis and reasoning to a file under `analysis/`; and only
   then, still in the same session, returns a **strictly validated JSON
   verdict**: repeat the round or stop. The loop is unbounded — it repeats as
   long as the decider orders another round.
6. Every agent receives **only the path to the artifact index** — no document
   paths are injected into any prompt. The index is a structured Markdown
   document maintained by the code (never by an LLM): one section per
   subworkflow execution, headed by the driver's task for that execution,
   listing every produced document with a description of its kind and the
   agent that produced it. Each agent identifies the documents to read from
   the index based on its role and the task at hand.
7. Control returns to the driver, which adapts the next step to the results.
   This repeats until the driver responds with `finish` and confirms it;
   there is no iteration budget.
8. A `finish` response never terminates the run by itself. The orchestrator
   sends the driver's full response back to it in a mandatory **finish
   reassessment** — a fourth call that **reuses the session acquired by the
   turn's read-inputs call**, so the driver second-guesses its decision with
   everything it read and analyzed still in context: the driver either
   confirms the `finish` (the run ends, and the confirmation rationale
   becomes the run outcome) or picks the subworkflow that was actually
   needed (which runs as the turn's decision).
   The reassessment response uses an explicit `decision` field
   (`confirm_finish` / `select_subworkflow`), and the orchestrator rejects
   confirmations that are structurally impossible — the same rule that
   applies to the regular turn. Both decisions are persisted under
   `decisions/`.

## Subworkflows

| id | agents | produces |
|----|--------|----------|
| `spec` | product manager + spec reviewer | task specification + review verdict |
| `classify` | classifier | investigation-vs-engineering classification |
| `decompose` | decomposer + decomposition reviewer | domain decomposition + review verdict |
| `architect` | architect + architecture reviewer | architecture + review verdict |
| `plan` | tech lead (planner) + plan reviewer | implementation plan + review verdict |
| `implement` | coder + code reviewer | implementation report + code review verdict |
| `tech_lead_final` | tech lead | final integration review (verdict) |
| `arch_final` | architect | final architecture review (verdict) |
| `investigate_plan` | investigator planner + quality reviewer + structural reviewer | investigation plan + review verdicts |
| `investigate` | investigator executor + fact checker + gap analyst | workstream findings + review verdicts |
| `synthesize` | synthesis agent + consistency reviewer | final investigation report + review verdict |

The `implement` subworkflow additionally feeds the code reviewer an automated
disk-change detection block (via the `wman` watcher, when available).

## Agent Roles

| Role | Kind | Responsibility |
|------|------|----------------|
| **Workflow Driver** | decision agent (JSON) | Picks the next subworkflow; a `finish` must survive the mandatory reassessment |
| **Loop Decider** | decision agent (JSON) | Reads a subworkflow's round documents; decides whether to repeat the round |
| **Product Manager** | Refines the request into an engineering-ready specification |
| **Classifier** | Determines investigation vs. engineering |
| **Decomposer** | Splits the task into bounded domains with integration ownership |
| **Architect** | Designs system architecture for a scope |
| **Tech Lead** | Writes concrete implementation plans |
| **Coder** | Implements approved plans in the repository |
| **Investigators** | Plan and execute bounded investigation workstreams |
| **Synthesizer** | Consolidates findings into the final report |
| **Reviewers** | Independent per-role reviewers that gate revision loops |

Content agents are ephemeral (fresh LLM sessions): every
durable fact flows through the artifact files, never through conversation
state. The workflow driver and the loop decider are the exceptions: each of
their turns is a multi-turn conversation whose session id is acquired from
the runtime and kept **solely in memory** for the duration of the turn — it is never written
to disk, so a process restart simply starts the next turn from a clean
session.

Role prompts live in `pkg/loader/prompts/`. Each content-agent prompt defines a
predefined Markdown document structure whose section labels mirror the original
schema field names for that role; no document content is ever machine-parsed
(the reviewer verdict line in `pkg/loader/prompts/shared/review_verdict.txt` is
for the loop decider to read, not for the orchestrator). Decision-agent prompts
carry their JSON schema plus an example response, and the runtime enforces the
schema via `--output-schema` before the orchestrator strictly re-validates the
response.

---

## Requirements

- **Go 1.26.4** (requires `GOEXPERIMENT=jsonv2`)
- **An LLM runtime** — the `codex` CLI wrapper (see `codex/`, `claudex/`)
- **wman script (optional)** — automated change detection for code reviews;
  enabled automatically when `wman/wman.sh` (or `AC_WMAN_PATH`) is available

```bash
export GOEXPERIMENT=jsonv2
export AC_WMAN_PATH=path/to/wman.sh   # optional
```

---

## Usage

### Execute a Task

```bash
go run ./src/ exec << 'TASK'
Describe the system you want built or investigated.
TASK
```

At run start the static wiring (agents, subworkflows, input/output graph) is
recorded in the session trace at `.state/static_definitions.md`.

The session ID is printed to stderr.

### Resume a Session

```bash
go run ./src/ exec resume <session_id>
```

Resume continues from the persisted workflow state: the artifact index and
decision history are rebuilt, an interrupted subworkflow execution is folded
into the history, and the driver picks up where it left off.

### Dump the Static Definitions

```bash
go run ./src/ static-defs [path]    # prints to stdout, or writes to path
```

The static definitions live in `src/definitions.go`: every agent (name, role
prompt, timeout, input document types, output document types) and every
subworkflow (name, description, ordered agents). `go test` verifies the
resulting graph: every declared input has a producer, every non-terminal
output is consumed, and every agent is used.

### Tracing

Set `AGENT_TRACE_FILE` to enable structured event logging (zstd-compressed JSON
lines: driver decisions, agent invocations, retries, verdicts, watcher events):

```bash
AGENT_TRACE_FILE=/tmp/trace.json.zstd go run ./src/ exec << 'TASK'
...
TASK
```

---

## Output

Each execution creates a session directory:

```text
.agent-2026-09-16_21-30-00/
├── 2026-09-16_21-30-00-task.txt   # Original task
├── SUMMARY.md                     # Outcome, artifact index, decision history
├── .state/
│   └── workflow_state.json        # Authoritative run state (resume point)
├── artifacts/                     # Agent documents (Markdown)
│   ├── INDEX.md                   # System-maintained artifact index
│   ├── 002-classify_investigation_classifier.md
│   ├── 004-spec_product_manager.md
│   ├── 005-spec_pm_review.md
│   └── ...
├── decisions/                     # Driver decision documents
│   ├── 001-driver.md
│   ├── 009-driver_finish_reassessment.md
│   └── ...
└── analysis/                      # Per-turn analysis documents
    ├── 002-driver.md
    ├── 003-loop_decider.md
    └── ...
```

Artifact file names carry a monotonically increasing sequence number, so every
pregenerated path is unique within a run, including across resumes.

---

## Design Goals

- **Dynamic control flow** — the driver adapts the workflow to the task at hand
  instead of following a hardcoded sequence
- **File-based agent communication** — agents exchange documents on disk, not
  inline JSON; downstream agents are told what each file is and instructed to
  study it
- **Short, independent subworkflows** — each is 1-3 closely tied agents; the
  unit the driver composes
- **Minimal output validation** — file existence and non-zero size; reviewer
  verdicts are the only machine-parsed control-flow token
- **Resumability** — state persists after every driver turn
- **Visible orchestration** — decisions, artifacts, and traces are inspectable
  on disk

## Non-Goals

- Web UI or interactive TUI beyond stdin/stdout/stderr
- Multi-user support, authentication/authorization
- Persistent databases, CI/CD orchestration, managed API key infrastructure

Investigation workflows are strictly read-only and cannot modify source code,
databases, or configuration files.

---

## Philosophy

High-quality autonomous execution requires that control flow be a *judgment*,
not a *script*. The orchestrator therefore encodes only the small, reliable
building blocks (subworkflows, file contracts, bounded review loops, state
persistence) and delegates sequencing to a driver agent that sees the actual
artifacts. Verification stays mechanical where it is cheap (files exist,
verdicts parse, changes detected on disk) and agentic where it is hard
(reviewers inspect the repository and the documents).
