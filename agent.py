# =========================
# AGENT
# =========================

from utils import run_codex
import logging


class Agent:
    def __init__(self, name, role_prompt, *, ephemeral=False, timeout=None):
        self.name = name
        self.session = None
        self.role_prompt = role_prompt
        self.ephemeral = ephemeral
        self.timeout = timeout

    def run(self, input_text):
        prompt = f"{input_text}\n\nReturn ONLY valid JSON."
        if not self.session:
            prompt = self.role_prompt + '\n\n' + prompt
        while True:
            try:
                out, self.session = run_codex(self.session, prompt, self.ephemeral, self.timeout)
                break
            except KeyboardInterrupt:
                raise
            except:
                logging.exception(f"Failed to run {self.name}, retrying...")
        return out
