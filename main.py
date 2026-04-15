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

import sys
from orchestrator import Orchestrator

# =========================
# CLI
# =========================

if __name__ == "__main__":
    task = " ".join(sys.argv[1:])
    if not task:
        print("Usage: python main.py 'task'")
        exit(1)

    orch = Orchestrator(task)
    orch.run()
