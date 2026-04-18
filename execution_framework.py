
# =========================
# EXECUTION FRAMEWORK
# =========================

from dataclasses import dataclass
import json

from agent import Agent
from config import MAX_CODE_ITERS
from utils import run_json_agent, log


@dataclass
class Configuration:
    """Configuration for unified code execution framework."""

    initial_prompt_context: str
    coder_agent_ref: object
    review_agent_ref: object
    task: str
    plan: dict

    max_iterations: int = MAX_CODE_ITERS


class CodeExecutionFramework:
    """Unified execution framework for coder+review loop."""

    @staticmethod
    def review_ok(review: dict) -> bool:
        """Check if review passed all checks."""
        approved = review.get("approved", False)

        log("REVIEW STATUS", f"approved={approved}")

        issues = review.get("issues", [])
        for issue in issues:
            if issue.get("severity") == "high":
                return False

        return approved

    @staticmethod
    def should_reset(review: dict) -> bool:
        """Check whether the review recommends resetting coder context."""
        return bool(review.get("should_reset", False))

    def execute(self, config: Configuration) -> str:
        coder_output = run_json_agent(
            config.coder_agent_ref,
            config.initial_prompt_context
        )
        all_outputs = [coder_output]

        for iteration_count in range(config.max_iterations):
            log(
                "CODER EXECUTION",
                f"Iteration {iteration_count + 1}/{config.max_iterations}"
            )

            *past_changes, recent_changes = [v.get("changes", []) for v in all_outputs]
            if not recent_changes:
                recent_changes = all_outputs[-1]
            review_prompt = (
                f"TASK:\n{config.initial_prompt_context}\n"
                f"ATTEMPT: {iteration_count + 1}/{config.max_iterations}\n\n"
                f"MOST RECENT CHANGES:\n{json.dumps(recent_changes)}\n\n"
                f"changes from past iterations for context:\n{json.dumps(past_changes)}"
            )

            review = run_json_agent(
                config.review_agent_ref,
                review_prompt
            )

            if self.review_ok(review):
                log("REVIEW", "Code approved by reviewer")
                break

            if self.should_reset(review):
                log(
                    "SYSTEM",
                    f"Resetting coder agent context: {review.get('reset_reason', '')}"
                )

                config.coder_agent_ref.reset()

                coder_prompt = (
                    f"TASK:\n{config.task}\n"
                    f"PLAN:\n{json.dumps(config.plan)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(review)}\n\n"
                    "Re-implement from a clean context using the task, plan, and review feedback. "
                    "Do not assume prior implementation decisions are correct unless still justified."
                )
            else:
                coder_prompt = (
                    "FEEDBACK TO ADDRESS:\n" + json.dumps(review)
                )

            coder_output = run_json_agent(
                config.coder_agent_ref,
                coder_prompt
            )
            all_outputs.append(coder_output)

        return '\n\n'.join([
            v.get('summary', '')
            for v in all_outputs
        ])
