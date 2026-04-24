# Multi-Agent Codex CLI Orchestrator

A Python system that turns a plain-English task description into implemented code by coordinating several specialized AI agents — each with its own role, prompt, and validation schema — inside a disciplined pipeline.

Think of it as an **AI-powered software delivery factory**: you hand it a task, and it goes through planning, architecture, decomposition, coding, and review phases to produce working code in your repo.

---

## How It Works (High-Level)

```
User Task
    │
    ▼
┌─────────────────────────────────────────────────┐
│  Phase 1 ─ Product Management                   │
│  · Break down the raw request into a structured │
│    spec using multiple reasoning angles         │
│  · Synthesize, clean up, and review the spec    │
│  · Convert design-language prompts into         │
│    implementation-language prompts              │
└─────────────────┬───────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────┐
│  Phase 2 ─ System Decomposition                 │
│  · Split the problem into independent domains   │
│  · Review and iterate on the decomposition      │
└─────────────────┬───────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────┐
│  Phase 3 ─ Architecture                         │
│  · Design architecture per domain               │
│  · Review each architecture                     │
│  · Convert design → implementation instructions │
└─────────────────┬───────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────┐
│  Phase 4 ─ Implementation Plan                  │
│  · Tech lead turns architecture into a          │
│    step-by-step, file-level plan                │
│  · Review and iterate on the plan               │
└─────────────────┬───────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────┐
│  Phase 5 ─ Code Execution                       │
│  · Coder agent writes actual code changes       │
│  · Review agent inspects changes each iteration │
│  · Repeats until reviewer approves or max       │
│    iterations reached                           │
└─────────────────┬───────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────┐
│  Phase 6 ─ Final Review                         │
│  · Tech lead + architect review the final code  │
│  · Optional revision loops if feedback requires │
│    a full re-implementation                     │
└─────────────────────────────────────────────────┘
```

Every output between phases is **JSON-only**, validated against a strict schema, and cached on disk so reruns can skip work that hasn't changed. If validation fails, the agent gets the error message and retries automatically.

---

## Business Goals of This System

1. **Automate software delivery.** Turn human-readable task descriptions into real code changes without manual engineering handoff.

2. **Enforce quality through multi-stage review.** Each phase has its own reviewer agent that can reject output, request changes, or force a full context reset — mimicking how senior engineers review each other's work.

3. **Ground everything in the actual codebase.** Agents must inspect the existing repository before proposing architecture, plans, or code. The system actively discourages inventing new components when existing ones will do.

4. **Keep human effort minimal.** You give it a task description and point it at your repo directory. Everything else — decomposing requirements, designing, planning, coding, reviewing — is automated.

---

## External Dependencies

| Dependency | Purpose |
|---|---|
| **[OpenAI Codex CLI](https://github.com/openai/codex)** (`codex` binary) | The underlying AI agent that actually reads prompts and writes code. Every "agent" in the orchestrator is just a named configuration of Codex CLI sessions. |
| **[Watchman](https://github.com/facebook/watchman)** | File-system watching (via `pywatchman`) so the system can detect which files the coder agent creates or modifies during execution. |
| Python 3.12+ (with packages in `requirements.txt`) | Runtime. Only needs standard library + `jsonschema`, `pywatchman`. |

The system does **not** contain any AI models itself — it is purely an orchestration layer on top of Codex CLI.

---

## Quick Start Guide

### 1. Install the Codex CLI

Make sure you have OpenAI's Codex CLI installed and authenticated:

```bash
# Install via npm (adjust if you use another method)
npm install -g @openai/codex

# Verify it works
codex --version
```

### 2. Set Up the Project

```bash
cd agent
pip install -r requirements.txt   # or: python -m venv venv && source venv/bin/activate && pip install -r requirements.txt
```

Ensure `watchman` is installed for file-watching:

```bash
# macOS
brew install watchman

# Ubuntu / Debian
sudo apt install watchman
```

### 3. Run the Orchestrator

```bash
python main.py "Add a login page with email and password" --dir .agent-workspace
```

This will:
- Create a working directory (`.agent-workspace` by default, timestamped)
- Launch the full pipeline (PM → Decomposition → Architecture → Planning → Coding → Review)
- Log progress to stderr
- Save intermediate artifacts (JSON outputs + generated Markdown docs) into `.agent-workspace/`

---

## Usage Example

```bash
# Point the orchestrator at an existing project directory
python main.py "Refactor the auth module to use JWT tokens instead of session cookies" \
    --dir .agent-jwt-migration
```

The orchestrator will:

1. **Analyze your repo** (it reads the file tree and passes it to agents for context)
2. **Generate a task spec** — what exactly needs to happen, from multiple angles (UX, edge cases, existing system behavior, etc.)
3. **Decompose into domains** — e.g., "token storage," "API middleware," "session migration"
4. **Design architecture per domain** — how each piece fits
5. **Create a step-by-step plan** — which files to create or modify, in what order
6. **Execute code changes** — the coder agent actually writes/edits files, with review loops until approved
7. **Final review** — tech lead and architect do a last pass

All intermediate artifacts are saved as JSON and as human-readable Markdown in:

```
.agent-jwt-migration/
├── 2026-04-23_14-30-00-task.txt           ← your original task
├── .state/                                ← cached agent outputs + sessions
│   ├── .sessions/                         ← persistent Codex session IDs
│   └── ...                                ← per-invocation JSON caches
└── document_stores/                       ← formatted Markdown docs
    ├── *_product_manager_final.md
    ├── *_decomposition_after_reviews.md
    ├── *_architecture_after_reviews.md
    └── *_tech_plan_after_reviews.md
```

---

## What This System Is Not (Out of Scope)

- **A standalone AI model.** It does not ship or train any LLMs. It only orchestrates calls to OpenAI's Codex CLI.

- **A general-purpose task manager.** The pipeline is fixed: PM → Decomposition → Architecture → Planning → Coding → Review. There is no concept of user-defined workflows, arbitrary branching, or custom phases.

- **A CI/CD or testing framework.** It does not run tests, deploy code, or manage environments. It produces code changes only. If you want to validate those changes, that's your responsibility.

- **A multi-language / multi-repo system.** It operates on a single directory at a time. There is no concept of cross-repo refactoring, monorepo coordination across independent repos, or language-specific tooling (linters, formatters) beyond what Codex CLI itself does.

- **A user-facing application.** This is a command-line orchestrator with no UI, no API server, no authentication, and no persistent state between unrelated invocations beyond agent session caching within a single `--dir`.

- **Guaranteed correctness.** Agents can produce incorrect or incomplete code. The review loop helps, but the system has no automated test execution — if the reviewer is wrong about something, that bug ships. Always verify output in a staging environment before production use.
