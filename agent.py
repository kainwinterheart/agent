# =========================
# AGENT
# =========================

import logging
from typing import Optional

from schema_utils import schema_to_example
from utils import (
    load_session_id,
    run_codex,
    save_session_id,
)


class Agent:
    def __init__(
        self,
        name: str,
        role_prompt: str,
        schema: dict,
        subdir: str,
        *,
        ephemeral: bool = False,
        timeout: bool = None,
    ) -> None:
        self.name = name
        self.subdir = subdir
        self.session = None
        self.session_suffix = None
        self.role_prompt = role_prompt
        self.schema = schema
        self.ephemeral = ephemeral
        self.timeout = timeout

    @property
    def session_key(self) -> Optional[str]:
        if self.ephemeral:
            return None
        session_key = self.name
        if self.session_suffix:
            session_key += "@"
            session_key += self.session_suffix
        return session_key

    def run(self, input_text: str) -> str:
        if not self.session and not self.ephemeral:
            self.session = load_session_id(self.session_key, self.subdir)
        prompt = f"{input_text}\n\nReturn ONLY valid JSON."
        if not self.session:
            prompt = self.role_prompt + "\n\n" + prompt
        last_error = ""
        prev_session = self.session
        while True:
            try:
                out, self.session = run_codex(
                    self.session,
                    prompt + last_error,
                    self.schema,
                    self.ephemeral,
                    self.timeout,
                )
                if prev_session != self.session:
                    save_session_id(self.session_key, self.session, self.subdir)
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

Fix it. ONLY return valid JSON:
{schema_to_example(self.schema)}
"""
        return out

    def reset(self, session_suffix: str) -> None:
        if not session_suffix or session_suffix == self.session_suffix:
            raise ValueError("Session suffix must be changed upon agent reset")
        self.session = None
        self.session_suffix = session_suffix
