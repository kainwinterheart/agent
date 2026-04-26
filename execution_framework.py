# =========================
# EXECUTION FRAMEWORK
# =========================

import json
import time
from dataclasses import dataclass
from typing import Optional

from agent import Agent
from config import MAX_CODE_ITERS
from utils import log, nudge, run_json_agent


@dataclass
class Configuration:
    """Configuration for unified code execution framework."""

    initial_prompt_context: str
    coder_agent_ref: object
    review_agent_ref: object
    task: str
    plan: dict

    max_iterations: int = MAX_CODE_ITERS
    nsc: Optional[object] = None


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

    def execute(
        self,
        config: Configuration,
        subdir: list[str],
        invocation_id_prefix: str,
        watcher,
    ) -> str:
        coder_output, changes = collect_changes(
            config.max_iterations,
            watcher,
            config.coder_agent_ref,
            config.initial_prompt_context,
            f"{invocation_id_prefix}-coder-0",
            subdir,
        )
        all_outputs = [coder_output]
        all_changes = changes

        for iteration_count in range(config.max_iterations):
            log(
                "CODER EXECUTION",
                f"Iteration {iteration_count + 1}/{config.max_iterations}",
            )

            *past_changes, recent_changes = [v.get("changes", []) for v in all_outputs]
            if not recent_changes:
                recent_changes = all_outputs[-1]
            review_prompt = (
                f"TASK:\n{config.initial_prompt_context}\n"
                f"ATTEMPT: {iteration_count + 1}/{config.max_iterations}\n\n"
                f"MOST RECENT CHANGES:\n{json.dumps(recent_changes)}\n"
                + changes_prompt(changes)
                + "\n"
                f"changes from past iterations for context:\n{json.dumps(past_changes)}"
            )

            review = nudge(
                config.max_iterations,
                config.review_agent_ref,
                review_prompt,
                f"{invocation_id_prefix}-code-review-{iteration_count}",
                subdir,
                nsc=config.nsc,
            )[-1]

            if self.review_ok(review):
                log("REVIEW", "Code approved by reviewer")
                break

            if self.should_reset(review):
                log(
                    "SYSTEM",
                    f"Resetting coder agent context: {review.get('reset_reason', '')}",
                )

                config.coder_agent_ref.reset(
                    f"{invocation_id_prefix}-{iteration_count}"
                )

                coder_prompt = (
                    f"TASK:\n{config.task}\n"
                    f"PLAN:\n{json.dumps(config.plan)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(review)}\n\n"
                    "Re-implement from a clean context using the task, plan, and review feedback. "
                    "Do not assume prior implementation decisions are correct unless still justified."
                )
            else:
                coder_prompt = "FEEDBACK TO ADDRESS:\n" + json.dumps(review)

            coder_output, changes = collect_changes(
                config.max_iterations,
                watcher,
                config.coder_agent_ref,
                coder_prompt,
                f"{invocation_id_prefix}-coder-{iteration_count + 1}",
                subdir,
            )
            all_outputs.append(coder_output)
            all_changes.update(changes)

        return (
            "\n".join(
                [
                    f"<revision{v[0] + 1}>\n{v[1].get('summary', '')}\n</revision{v[0] + 1}>"
                    for v in enumerate(all_outputs)
                ]
            )
            + "\n"
            + changes_prompt(all_changes)
        )


def collect_changes(max_it, watcher, agent, prompt, invocation_id_prefix, subdir):
    watcher.wait()
    watcher.flush()
    results = nudge(
        max_it, agent, prompt, invocation_id_prefix, subdir, return_system_state=True
    )
    if not results[-1]["from_cache"]:
        time.sleep(10)
    return results[-1]["out"], watcher.flush()


def changes_prompt(changes: set[str]) -> str:
    if not changes:
        return "**AUTOMATED VERIFICATION FAILED: NO ACTUAL CHANGES DETECTED**\n"
    out = "AUTOMATED VERIFICATION DETECTED POTENTIAL CHANGES TO FOLLOWING FILES, COMPLETENESS MUST BE ASSESSED:\n"
    for name, state in sorted(changes.items(), key=lambda v: v[0]):
        out += f"* {state}: {name}\n"
    return out
