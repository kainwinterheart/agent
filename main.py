# Multi-Agent Codex CLI Orchestrator v4.1 (Hardened, Sequential)
# =================================================================
# Improvements over v4:
# - STRICT JSON enforcement with auto-repair loop
# - Fail-fast on empty/invalid outputs
# - Strong prompts (no prose, JSON only)
# - Always include TASK + CONTEXT in every call
# - Workspace-aware coder (must create files)
# - Safer test runner fallback (pytest optional)
# - Better logging (raw + parsed)
# - Simple repo awareness (file tree passed to coder)

import argparse
import os
import time

from orchestrator import Orchestrator

# =========================
# CLI
# =========================

if __name__ == "__main__":
    ts = time.strftime("%Y-%m-%d_%H-%M-%S")
    parser = argparse.ArgumentParser(description="Multi-Agent Codex CLI Orchestrator")
    parser.add_argument("task", nargs="...", help="Task description")
    parser.add_argument(
        "--dir",
        type=str,
        default=f".agent-{ts}",
        help="path for the state store",
    )

    args = parser.parse_args()
    task = " ".join(args.task) if args.task else ""

    if task:
        os.makedirs(args.dir, exist_ok=True)
        task_file = os.path.join(args.dir, f"{ts}-task.txt")
        with open(task_file, "w") as f:
            f.write(task)

    orch = Orchestrator(task, args.dir)
    orch.run()
