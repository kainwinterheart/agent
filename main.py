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

import os
import random
import sys
import time

from orchestrator import Orchestrator

# =========================
# CLI
# =========================


def read_stdin():
    data = []
    while True:
        chunk = sys.stdin.read(4096)
        if not chunk:
            break
        data.append(chunk)
    return "".join(data).strip()


def main():
    random.seed(time.time())
    ts = time.strftime("%Y-%m-%d_%H-%M-%S")

    # Manual argument parsing for predictable positional handling
    args = sys.argv[1:]

    if args and args[0] == "exec":
        args = args[1:]
    elif args and args[0] in ("-h", "--help"):
        print("usage: main.py exec [resume <session_id>]", file=sys.stderr)
        sys.exit(0)
    else:
        print("usage: main.py exec [resume <session_id>]", file=sys.stderr)
        sys.exit(1)

    is_resume = False
    session_id = None

    if args and args[0] == "resume":
        is_resume = True
        args = args[1:]
        if not args:
            print("Missing session_id", file=sys.stderr)
            sys.exit(1)
        session_id = args[0]

    subdir = session_id if is_resume else f".agent-{ts}"

    # Print session id to stderr (same prefix as codex/claudex tools)
    print("session id: " + subdir, file=sys.stderr)

    task = None
    if is_resume:
        for f in os.listdir(subdir):
            if f.endswith("-task.txt"):
                with open(os.path.join(subdir, f), "r") as f:
                    task = f.read()
                break
    else:
        task = read_stdin()
    if not task:
        raise AssertionError("Task content must be provided")
    if not is_resume:
        os.makedirs(subdir, exist_ok=True)
        task_file = os.path.join(subdir, f"{ts}-task.txt")
        with open(task_file, "w") as f:
            f.write(task)

    orch = Orchestrator(task, subdir)
    orch.run()


if __name__ == "__main__":
    main()
