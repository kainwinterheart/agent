import json
import os


def trace(action: str, obj: dict) -> None:
    dest = os.getenv("AGENT_TRACE_FILE")
    if not dest:
        return
    with open(dest, "a") as fh:
        fh.write(json.dumps({"action": action, "details": obj}) + "\n")
