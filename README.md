# Multi-Agent Orchestration System

A CLI orchestration system that turns high-level requests into either:

- structured investigative reports, or
- reviewed, validated code changes

The system uses multiple specialized agents to refine ambiguous requests, plan execution, review outputs, and verify results before completion.

---

## What It Does

You give the orchestrator a task description through stdin.

The system:

1. Refines the request into a clearer specification
2. Determines whether the task is:
   - an investigation task, or
   - an engineering task
3. Routes the task through the appropriate workflow
4. Produces either:
   - a research/report artifact, or
   - implemented code changes with review and validation

The goal is to make vague requests executable without requiring manually written specs or tightly scoped prompts.

---

## Example

### Input

```bash
python main.py exec << 'EOF'
Build a lightweight job queue with retry support and a Redis backend.
EOF
```

### Possible Workflow

- The request is clarified and refined
- The task is classified as engineering work
- The system decomposes the work into domains
- Agents design architecture and implementation plans
- Code is implemented and reviewed iteratively
- Final output is validated against the refined specification

Artifacts are written to:

```text
.agent-<timestamp>/
```

---

# Workflows

## Investigation Workflow

Used for research, analysis, auditing, or exploratory tasks where no code changes should be made.

Examples:

- Analyze an existing architecture
- Investigate performance bottlenecks
- Compare implementation strategies
- Produce migration recommendations
- Review a codebase for risks or inconsistencies

### Characteristics

- Read-only execution
- Multi-agent investigative planning
- Evidence collection and synthesis
- Structured report generation
- Review and fact-checking passes

Final output is a documentation/report artifact printed to stdout and stored in the session directory.

---

## Engineering Workflow

Used for implementation tasks that create or modify code.

Examples:

- Build a new subsystem
- Refactor existing components
- Add features
- Fix architectural problems
- Implement integrations

### Characteristics

- Task decomposition into independent work domains
- Architecture design and review cycles
- Implementation planning before coding
- Automated workspace change detection
- Iterative review and refinement loops
- Final semantic validation against the original request

After implementation, a final review pass verifies that the resulting system matches the intent of the original task.

---

# Design Goals

- Handle ambiguous, high-level requests
- Separate investigation work from implementation work
- Encourage iterative review instead of one-shot generation
- Keep orchestration visible and inspectable through artifacts
- Support long-running and resumable execution

---

# Requirements

## Python

- Python 3.12+

Create and activate a virtual environment, then install project dependencies.

---

## External Agent Runtime

The orchestrator depends on an external CLI agent runtime available on `PATH`.

This runtime is responsible for executing all agent tasks.

---

## Filesystem Watcher

Engineering workflows use a filesystem watcher for automated change detection during implementation and review cycles.

Install and run the required watcher daemon/service on the host system.

---

## Optional Container Runtime

- Docker or Podman

Can be used for isolated/containerized agent execution when configured.

---

## LLM Credentials

LLM API keys and provider configuration are managed externally through the agent runtime.

---

# Usage

## Execute a Task

```bash
python main.py exec << 'EOF'
Describe the system you want built or investigated.
EOF
```

The session ID is printed to stderr.

Artifacts are stored in:

```text
.agent-<timestamp>/
```

---

## Resume a Session

```bash
python main.py exec resume <session_id>
```

---

# Output Structure

Each execution creates a session directory containing orchestration artifacts such as:

- refined specifications
- plans
- architecture documents
- review outputs
- investigation reports
- implementation artifacts
- validation results

This makes execution traceable and resumable.

---

# Non-Goals

This project intentionally does not include:

- Web UI
- Interactive terminal UI beyond stdin/stdout/stderr
- Multi-user support
- Authentication/authorization
- Persistent databases
- CI/CD orchestration
- Managed API key infrastructure

Investigation workflows are strictly read-only and cannot modify source code, databases, or configuration files.

---

# Philosophy

The system is designed around the idea that high-quality autonomous execution requires:

- refinement before implementation
- specialization of responsibilities
- iterative review loops
- explicit validation against intent

Rather than relying on a single agent operating from a single prompt, the orchestrator treats execution as a staged process with verification at each layer.
