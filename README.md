# Multi-Agent Orchestration System

A Go-based orchestration engine that turns high-level requests into structured
investigative reports or reviewed, validated code changes using multiple
specialized AI agents.

---

## What It Does

You give the orchestrator a task description through stdin.

The system:

1. Refines the request into a clearer specification
2. Classifies the task as **investigation** or **engineering**
3. Routes it through the appropriate workflow
4. Coordinates specialized agents to produce a final artifact

The goal is to make vague, high-level requests executable without requiring
manually written specs.

---

## Agent Roles

The orchestrator coordinates these specialized agents:

| Role | Responsibility |
|------|----------------|
| **Product Manager** | Refines ambiguous requests, expands scope |
| **Architect** | Designs system architecture and integration plan |
| **Tech Lead** | Creates implementation plans and decomposes domains |
| **Coder** | Implements code changes |
| **Arch Review** | Reviews architectural decisions |
| **Plan Review** | Validates implementation plans |
| **Code Review** | Reviews and approves code changes |

---

## Workflow Types

### Investigation Workflow

Read-only research and analysis. Agents collect evidence, synthesize findings,
and produce structured reports. No file modifications are permitted.

### Engineering Workflow

Implementation tasks. Features:

- **Domain decomposition** — work split into independent domains with
  topological ordering
- **Iterative coder+review loop** — code is implemented and reviewed
  iteratively until approved
- **Automated change detection** — filesystem watcher detects actual changes
  made by agents
- **Final validation** — the system verifies the result matches the original
  intent

---

## Requirements

- **Go 1.26.4** (requires `GOEXPERIMENT=jsonv2`)
- **An LLM runtime** — configured externally through the agent service
- **wman script** — `AC_WMAN_PATH` must point to `wman/wman.sh`

Set these environment variables before running:

```bash
export GOEXPERIMENT=jsonv2
export AC_WMAN_PATH=path/to/wman.sh
```

---

## Usage

### Execute a Task

```bash
go run ./src/ exec << 'EOF'
Describe the system you want built or investigated.
EOF
```

The session ID is printed to stderr.

### Resume a Session

```bash
go run ./src/ exec resume <session_id>
```

### Tracing

Set `AGENT_TRACE_FILE` to enable structured event logging:

```bash
AGENT_TRACE_FILE=/tmp/trace.json go run ./src/ exec << 'EOF'
...
EOF
```

---

## Output

Each execution creates a session directory:

```text
.agent-2026-06-15_14-30-00/
├── 2026-06-15_14-30-00-task.txt    # Original task
├── .state/                         # Agent response cache
│   ├── ProductManager_xxx.out      # Cached responses
│   └── ProductManager_xxx.in       # Input prompts
└── ...                             # Intermediate artifacts
```

Artifacts include refined specifications, plans, architecture documents,
review outputs, investigation reports, implementation artifacts, and
validation results.

---

## Design Goals

- Handle ambiguous, high-level requests
- Separate investigation work from implementation work
- Encourage iterative review instead of one-shot generation
- Keep orchestration visible and inspectable through artifacts
- Support long-running and resumable execution

---

## Non-Goals

This project intentionally does not include:

- Web UI
- Interactive terminal UI beyond stdin/stdout/stderr
- Multi-user support
- Authentication/authorization
- Persistent databases
- CI/CD orchestration
- Managed API key infrastructure

Investigation workflows are strictly read-only and cannot modify source code,
databases, or configuration files.

---

## Philosophy

The system is designed around the idea that high-quality autonomous execution
requires:

- **Refinement before implementation** — ambiguous requests are clarified
- **Specialization of responsibilities** — different agents for different tasks
- **Iterative review loops** — code and plans are reviewed repeatedly
- **Explicit validation against intent** — final output is checked against the
  original request

Rather than relying on a single agent operating from a single prompt, the
orchestrator treats execution as a staged process with verification at each
layer.
