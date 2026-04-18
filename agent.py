# =========================
# AGENT
# =========================

from utils import run_codex
import json
import logging


class Agent:
    def __init__(self, name: str, role_prompt: str, schema: str, *, ephemeral: bool=False, timeout: bool=None) -> None:
        self.name = name
        self.session = None
        self.role_prompt = role_prompt
        self.schema = json.loads(schema)
        self.ephemeral = ephemeral
        self.timeout = timeout

    def run(self, input_text: str) -> str:
        prompt = f"{input_text}\n\nReturn ONLY valid JSON."
        if not self.session:
            prompt = self.role_prompt + '\n\n' + prompt
        last_error = ""
        while True:
            try:
                out, self.session = run_codex(
                    self.session,
                    prompt + last_error,
                    json.dumps(self.schema),
                    self.ephemeral,
                    self.timeout,
                )
                break
            except KeyboardInterrupt:
                raise
            except Exception as e:
                logging.exception(f"Failed to run {self.name}, retrying...")
                last_error = f"""\n\n
**Previous attempt to read your response for the current task failed**:
```
{e}
```

Fix it.
"""
        return out

    def reset(self) -> None:
        self.session = None
