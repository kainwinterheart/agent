# =========================
# AGENT
# =========================

import json
import logging
import random
import time
from typing import Optional

import prompts
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
        self.role_prompt = f"<role>\n{role_prompt}\n</role>"
        self.schema = schema
        self.ephemeral = ephemeral
        self.timeout = timeout
        self.last_correct_response = None

    @property
    def session_key(self) -> Optional[str]:
        if self.ephemeral and not self.session_suffix:
            self.reset()
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
            next_prompt = prompt + last_error
            if self.session or self.last_correct_response:
                next_prompt += "\n\n"
                if self.last_correct_response:
                    next_prompt += f"PREVIOUS RESPONSE: {json.dumps(self.last_correct_response)}\n\n"
                next_prompt += prompts.FOLLOWUP
            try:
                out, self.session = run_codex(
                    self.name,
                    self.session,
                    next_prompt,
                    self.schema,
                    self.timeout,
                )
                if prev_session != self.session and not self.ephemeral:
                    save_session_id(self.session_key, self.session, self.subdir)
                break
            except KeyboardInterrupt:
                raise
            except Exception as e:
                logging.exception(f"Failed to run {self.name}, retrying...")
                last_error = f"""\n\n
<warning>
**Previous attempt to read your response for the current task failed**:
<error>
{e}
</error>

Output MUST be valid JSON only:
{schema_to_example(self.schema)}
</warning>
"""
        return out

    def reset(self, session_suffix: Optional[str] = None) -> None:
        if self.ephemeral:
            if session_suffix:
                raise ValueError(
                    "Session suffix can't be specified for ephemeral agents"
                )
            session_suffix = f"{time.time()}-{random.random() * time.time()}"
        else:
            if not session_suffix or session_suffix == self.session_suffix:
                raise ValueError(
                    "Session suffix must be changed upon non-ephemeral agent reset"
                )
        self.session = None
        self.session_suffix = session_suffix
        self.last_correct_response = None
