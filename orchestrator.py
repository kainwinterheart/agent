# =========================
# ORCHESTRATOR
# =========================

import json
import os
from typing import Optional

import prompts
import schemas
from agent import Agent
from config import MAX_CODE_ITERS, MAX_PLAN_ITERS, MAX_TOP_ITERATIONS
from execution_framework import CodeExecutionFramework, Configuration
from utils import (
    assert_not_empty,
    log,
    markdown_document_generator,
    nudge,
    run_json_agent,
)
from wman import WatchmanBackgroundWatcher


class Orchestrator:
    def __init__(self, task: str, subdir: str):
        self.task = wrap_text(task)
        self.subdir = subdir

        self.product_manager = Agent(
            "product_manager",
            prompts.PRODUCT_MANAGER_PROMPT,
            schemas.PRODUCT_MANAGER_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="30m",
        )
        self.pm_synth = Agent(
            "pm_synth",
            prompts.PM_SYNTHESIZER_PROMPT,
            schemas.PM_SYNTHESIZER_SCHEMA,
            self.subdir,
            timeout="30m",
        )
        self.pm_expansion_cleanup = Agent(
            "pm_expansion_cleanup",
            prompts.PM_EXPANSION_CLEANUP_PROMPT,
            schemas.PM_EXPANSION_CLEANUP_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="10m",
        )
        self.pm_review = Agent(
            "pm_review",
            prompts.PM_REVIEW_PROMPT,
            schemas.PM_REVIEW_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="30m",
        )

        self.design_cleanup = Agent(
            "design_cleanup",
            prompts.DESIGN_TO_IMPLEMENT_PHRASING_PROMPT,
            schemas.DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="10m",
        )

        self.arch = Agent(
            "arch", prompts.ARCH_PROMPT, schemas.ARCH_SCHEMA, self.subdir, timeout="40m"
        )
        self.tech_lead = Agent(
            "tech_lead",
            prompts.PLAN_PROMPT,
            schemas.PLAN_SCHEMA,
            self.subdir,
            timeout="60m",
        )
        self.coder = Agent(
            "coder",
            prompts.CODER_PROMPT,
            schemas.CODER_SCHEMA,
            self.subdir,
            timeout="180m",
        )

        self.arch_review = Agent(
            "arch_review",
            prompts.ARCH_REVIEW_PROMPT,
            schemas.ARCH_REVIEW_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="30m",
        )
        self.plan_review = Agent(
            "plan_review",
            prompts.PLAN_REVIEW_PROMPT,
            schemas.PLAN_REVIEW_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="30m",
        )
        self.code_review = Agent(
            "code_review",
            prompts.CODE_REVIEW_PROMPT,
            schemas.CODE_REVIEW_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="60m",
        )

        self.tech_lead_final = Agent(
            "tech_lead_final",
            prompts.TECH_LEAD_FINAL_PROMPT,
            schemas.TECH_LEAD_FINAL_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="60m",
        )
        self.arch_final = Agent(
            "arch_final",
            prompts.ARCH_FINAL_PROMPT,
            schemas.ARCH_FINAL_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="60m",
        )

        self.decomposition = Agent(
            "decomposition",
            prompts.SYSTEM_DECOMPOSITION_PROMPT,
            schemas.SYSTEM_DECOMPOSITION_SCHEMA,
            self.subdir,
            timeout="40m",
        )
        self.decomposition_review = Agent(
            "decomposition_review",
            prompts.SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
            schemas.SYSTEM_DECOMPOSITION_REVIEW_SCHEMA,
            self.subdir,
            ephemeral=True,
            timeout="30m",
        )

    def review_ok(self, review: dict) -> bool:
        approved = review.get("approved", False)

        log("REVIEW STATUS", f"approved={approved}")

        issues = review.get("issues", [])
        for issue in issues:
            if issue.get("severity") == "high":
                return False

        return approved

    def should_reset(self, review: dict) -> bool:
        return bool(review.get("should_reset", False))

    def pm_transformation_workflow(self):
        candidates = []
        for idx, prompt in [
            (0, "Bias toward minimal scope and preserving the literal user request."),
            (
                1,
                "Bias toward UX completeness, validation rules, and expected user behavior.",
            ),
            (2, "Bias toward implementation simplicity and minimal engineering risk."),
            (3, "Bias toward edge cases, state transitions, and failure scenarios."),
            (
                4,
                "Bias toward preserving existing system behavior and minimizing changes to current workflows.",
            ),
            (
                5,
                "Bias toward permissions, roles, ownership boundaries, and access control behavior.",
            ),
            (
                6,
                "Bias toward data model implications, persistence behavior, lifecycle management, and state consistency.",
            ),
            (
                7,
                "Bias toward API behavior, input/output contracts, validation, and error handling.",
            ),
            (
                8,
                "Bias toward reporting, auditability, notifications, logging, and observability requirements.",
            ),
            (
                9,
                "Bias toward backward compatibility, migration concerns, rollout safety, and minimizing disruption to existing users.",
            ),
            (
                10,
                "Bias toward operational concerns such as performance, scalability, concurrency, and long-term maintainability.",
            ),
            (
                11,
                "Bias toward identifying the smallest possible implementation that still fully satisfies the request.",
            ),
            (
                12,
                "Bias toward identifying where the request may be overcomplicated, unnecessary, or better solved through a smaller existing workflow change instead of a new feature.",
            ),
            (
                13,
                "Bias toward preserving only what is explicitly stated by the user. Avoid assumptions unless absolutely necessary.",
            ),
            (
                14,
                "Bias toward identifying the user's likely business goal and ensuring the specification solves that goal with the smallest possible feature set.",
            ),
        ]:
            candidate = nudge(
                100,
                self.product_manager,
                f"USER REQUEST:\n{self.task}\n\n"
                f"TASK:\nProduce a focused engineering-ready specification.\n{prompt}",
                f"pm-spec-candidate-{idx}",
                [self.subdir],
            )[-1]

            candidates.append(candidate)

        choices = "\n".join(
            [f"CANDIDATE {i + 1}:\n{json.dumps(v)}\n" for i, v in enumerate(candidates)]
        )
        rephrased_task = run_json_agent(
            self.pm_synth,
            f"ORIGINAL USER REQUEST:\n{self.task}\n\n{choices}"
            "TASK:\nSelect the single best interpretation of the original user request.Preserve only the minimum assumptions necessary.Reject speculative scope expansion.",
            "pm-spec-synthesis-0",
            [self.subdir],
        )

        for iteration in range(MAX_PLAN_ITERS):
            review = run_json_agent(
                self.pm_review,
                f"ORIGINAL USER REQUEST:\n{self.task}\n\n"
                f"SYNTHESIZED SPECIFICATION:\n{json.dumps(rephrased_task)}\n\n"
                f"ATTEMPT: {iteration + 1}/{MAX_PLAN_ITERS}\n\n"
                "TASK:\nReview whether the synthesized specification correctly preserves the original user intent.Reject only if the specification is ambiguous, speculative, internally inconsistent, or over-expanded.",
                f"pm-spec-review-{iteration}",
                [self.subdir],
            )

            if self.review_ok(review):
                break

            if self.should_reset(review):
                log(
                    "SYSTEM",
                    f"Resetting PM Synthesizer context: {review.get('reset_reason', '')}",
                )

                self.pm_synth.reset(str(iteration))

                revision_prompt = (
                    f"ORIGINAL USER REQUEST:\n{self.task}\n\n{choices}"
                    f"PREVIOUS SYNTHESIZED SPECIFICATION:\n{json.dumps(rephrased_task)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(review)}\n"
                    """TASK:
Revise the synthesized specification to address the review feedback while preserving the original user intent and keeping the scope minimal.
                    """
                )
            else:
                revision_prompt = f"REVISE SYNTHESIZED SPECIFICATION based on feedback:\n{json.dumps(review)}"

            rephrased_task = run_json_agent(
                self.pm_synth,
                revision_prompt,
                "pm-spec-synthesis-{iteration + 1}",
                [self.subdir],
            )

        speculative_expansions = rephrased_task.get("speculative_expansions")
        if isinstance(speculative_expansions, list) and speculative_expansions:
            while True:
                clean_speculative_expansions = run_json_agent(
                    self.pm_expansion_cleanup,
                    "INPUT JSON:\n" + json.dumps({"lines": speculative_expansions}),
                    "pm-expansion-cleanup",
                    [self.subdir],
                ).get("lines", [])

                if len(clean_speculative_expansions) == len(speculative_expansions):
                    break
            rephrased_task["speculative_expansions"] = clean_speculative_expansions

        markdown_document_generator(
            rephrased_task, "product_manager_final", [self.subdir]
        )
        out = rephrased_task["task_specification"]
        files = rephrased_task.get("files")
        if isinstance(files, list) and files:
            out += "\n\nMentioned files:\n"
            for file in files:
                out += f"* {file}\n"
        proper_nouns = rephrased_task.get("proper_nouns")
        if isinstance(proper_nouns, list) and proper_nouns:
            out += "\n\nMentioned proper nouns:\n"
            for proper_noun in proper_nouns:
                out += f"* {proper_noun}\n"
        facts = rephrased_task.get("facts")
        if isinstance(facts, list) and facts:
            out += "\n\nStated facts:\n"
            for fact in facts:
                out += f"* {fact}\n"
        missing_but_necessary_details = rephrased_task.get(
            "missing_but_necessary_details"
        )
        if (
            isinstance(missing_but_necessary_details, list)
            and missing_but_necessary_details
        ):
            out += "\n\nAdditional considerations:\n"
            for missing_but_necessary_detail in missing_but_necessary_details:
                out += f"* {missing_but_necessary_detail}\n"
        speculative_expansions = rephrased_task.get("speculative_expansions")
        if isinstance(speculative_expansions, list) and speculative_expansions:
            out += "\n\nOut of scope:\n"
            for speculative_expansion in speculative_expansions:
                out += f"* {speculative_expansion}\n"
        return out

    def run(self):
        self.watcher = WatchmanBackgroundWatcher(self.subdir)
        self.watcher.start(os.getcwd())
        try:
            return self._run()
        finally:
            self.watcher.stop()

    def _run(self):
        root_task = wrap_text(self.pm_transformation_workflow())

        decomposition = self.decomposition_workflow(root_task)

        domains = decomposition.get("decomposition", {}).get("domains", [])

        for domain_index, domain in enumerate(domains):
            task = domain.get("architect_input")
            self.domain_id = domain.get("id", domain_index + 1)

            session_suffix = f"d{self.domain_id}-start"
            self.arch.reset(session_suffix)
            self.tech_lead.reset(session_suffix)
            self.coder.reset(session_suffix)
            final_feedback = None

            if not task:
                continue

            coder_task = run_json_agent(
                self.design_cleanup,
                f"INPUT TEXT:\n{task}",
                f"d{self.domain_id}-design-cleanup",
                [self.subdir, str(self.domain_id)],
            )["text"]

            task = wrap_text(task)
            coder_task = wrap_text(coder_task)

            for iteration in range(MAX_TOP_ITERATIONS):
                log(
                    "ITERATION",
                    f"Starting iteration {iteration + 1}/{MAX_TOP_ITERATIONS}"
                    f" for domain {domain_index + 1}/{len(domains)}",
                )

                arch = self.architecture_design_phase(
                    final_feedback,
                    task,
                    f"d{self.domain_id}-arch-{iteration}",
                )

                plan = self.plan_creation_phase(
                    arch,
                    coder_task,
                    f"d{self.domain_id}-plan-{iteration}",
                )

                code_summary = self.code_implementation_phase(
                    plan,
                    coder_task,
                    f"d{self.domain_id}-impl-{iteration}",
                )

                code_summaries = [code_summary]
                tech_lead_final_review = None

                for tl_iteration in range(MAX_TOP_ITERATIONS):
                    tech_lead_final_review = self.tech_lead_review_phase(
                        code_summary,
                        arch,
                        plan,
                        coder_task,
                        tech_lead_final_review,
                        f"d{self.domain_id}-tl-review-{iteration}-{tl_iteration}",
                    )

                    if self.review_ok(tech_lead_final_review):
                        break

                    log(
                        "SYSTEM",
                        "Tech lead feedback received - revising implementation",
                    )

                    code_summary = self.revision_loops(
                        tech_lead_final_review,
                        coder_task,
                        plan,
                        f"d{self.domain_id}-impl-revision-{iteration}-{tl_iteration}",
                        reset_coder=self.should_reset(tech_lead_final_review),
                    )

                    code_summaries.append(code_summary)

                merged_code_summaries = "\n".join(
                    map(
                        lambda v: f"<summary{v[0] + 1}>\n{v[1]}\n</summary{v[0] + 1}>",
                        enumerate(code_summaries),
                    )
                )
                final_feedback = run_json_agent(
                    self.arch_final,
                    f"TASK:\n{task}\n"
                    f"ARCHITECTURE:\n{json.dumps(arch)}\n"
                    f"APPROVED IMPLEMENTATION PLAN:\n{json.dumps(plan)}\n"
                    "<aggregate_implementation_summary>\n"
                    f"{merged_code_summaries}\n"
                    "</aggregate_implementation_summary>\n",
                    f"d{self.domain_id}-arch-final-review-{iteration}",
                    [self.subdir, str(self.domain_id)],
                )

                if self.review_ok(final_feedback):
                    break

    def decomposition_workflow(self, task: str) -> dict:
        decomposition_result = nudge(
            100, self.decomposition, f"TASK:\n{task}", "decomposition-0", [self.subdir]
        )[-1]

        assert_not_empty(decomposition_result, "DECOMPOSITION")

        for i in range(MAX_PLAN_ITERS):
            decomposition_review = run_json_agent(
                self.decomposition_review,
                f"TASK:\n{task}\n"
                f"ATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\n"
                f"DECOMPOSITION TO REVIEW:\n{json.dumps(decomposition_result)}",
                f"decomposition-review-{i}",
                [self.subdir],
            )

            if self.review_ok(decomposition_review):
                break

            if self.should_reset(decomposition_review):
                log(
                    "SYSTEM",
                    f"Resetting decomposition context: {decomposition_review.get('reset_reason', '')}",
                )

                self.decomposition.reset(str(i))

                revision_prompt = (
                    f"TASK:\n{task}\n"
                    f"PREVIOUS DECOMPOSITION:\n{json.dumps(decomposition_result)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(decomposition_review)}\n\n"
                    "Rebuild the decomposition from scratch using the original task and review feedback."
                )
            else:
                revision_prompt = f"REVISE DECOMPOSITION based on feedback:\n{json.dumps(decomposition_review)}"

            decomposition_result = nudge(
                100,
                self.decomposition,
                revision_prompt,
                f"decomposition-{i + 1}",
                [self.subdir],
            )[-1]

        decomposition_result.get("decomposition", {}).pop("reviewer_notes", None)
        markdown_document_generator(
            decomposition_result, "decomposition_final", [self.subdir]
        )

        return decomposition_result

    def architecture_design_phase(
        self,
        final_feedback: dict,
        task: str,
        invocation_id_prefix: str,
    ) -> dict:
        if final_feedback:
            initial_prompt = f"REVISE ARCHITECTURE based on feedback post implementation:\n{json.dumps(final_feedback)}"
        else:
            initial_prompt = f"TASK:\n{task}"

        extra_prompt = "\nKeep architecture focused on component boundaries, ownership, and interactions. Avoid naming concrete functions, methods, language constructs, or exact code statements unless they are architecturally significant."
        arch = nudge(
            100,
            self.arch,
            initial_prompt + extra_prompt,
            f"{invocation_id_prefix}-0",
            [self.subdir, str(self.domain_id)],
        )[-1]

        for i in range(MAX_PLAN_ITERS):
            arch_review = nudge(
                100,
                self.arch_review,
                f"TASK:\n{task}\n"
                f"ATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\n"
                f"ARCHITECTURE TO REVIEW:\n{json.dumps(arch)}",
                f"{invocation_id_prefix}-review-{i}",
                [self.subdir, str(self.domain_id)],
            )[-1]

            if self.review_ok(arch_review):
                break

            if self.should_reset(arch_review):
                log(
                    "SYSTEM",
                    f"Resetting architect context: {arch_review.get('reset_reason', '')}",
                )

                self.arch.reset(f"{invocation_id_prefix}-{i}")

                revision_prompt = (
                    f"TASK:\n{task}\n"
                    f"PREVIOUS ARCHITECTURE:\n{json.dumps(arch)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(arch_review)}\n\n"
                    "Rebuild the architecture from scratch using the task and review feedback."
                )
            else:
                revision_prompt = (
                    f"REVISE ARCHITECTURE based on feedback:\n{json.dumps(arch_review)}"
                )

            arch = nudge(
                100,
                self.arch,
                revision_prompt + extra_prompt,
                f"{invocation_id_prefix}-{i + 1}",
                [self.subdir, str(self.domain_id)],
            )[-1]

        arch.get("architecture", {}).pop("reviewer_notes", None)
        markdown_document_generator(
            arch, "architecture_after_reviews", [self.subdir, str(self.domain_id)]
        )

        return arch

    def plan_creation_phase(
        self,
        arch: dict,
        task: str,
        invocation_id_prefix: str,
    ) -> dict:
        extra_prompt = "\nPrefer concrete file-level changes, but avoid embedding exact code snippets unless the task is trivial and the code itself is the clearest representation of the change."
        plan = run_json_agent(
            self.tech_lead,
            f"TASK:\n{task}\nAPPROVED ARCHITECTURE:\n{json.dumps(arch)}" + extra_prompt,
            f"{invocation_id_prefix}-0",
            [self.subdir, str(self.domain_id)],
        )

        for i in range(MAX_PLAN_ITERS):
            plan_review = nudge(
                100,
                self.plan_review,
                f"TASK:\n{task}\n"
                f"ATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\n"
                f"APPROVED ARCHITECTURE:\n{json.dumps(arch)}\n"
                f"PLAN TO REVIEW:\n{json.dumps(plan)}",
                f"{invocation_id_prefix}-review-{i}",
                [self.subdir, str(self.domain_id)],
            )[-1]

            if self.review_ok(plan_review):
                break

            if self.should_reset(plan_review):
                log(
                    "SYSTEM",
                    f"Resetting tech lead context: {plan_review.get('reset_reason', '')}",
                )

                self.tech_lead.reset(f"{invocation_id_prefix}-{i}")

                revision_prompt = (
                    f"TASK:\n{task}\n"
                    f"APPROVED ARCHITECTURE:\n{json.dumps(arch)}\n"
                    f"PREVIOUS PLAN:\n{json.dumps(plan)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(plan_review)}\n\n"
                    "Rebuild the plan from scratch using the task, architecture, and review feedback."
                )
            else:
                revision_prompt = (
                    f"REVISE PLAN based on feedback:\n{json.dumps(plan_review)}"
                )

            plan = run_json_agent(
                self.tech_lead,
                revision_prompt + extra_prompt,
                f"{invocation_id_prefix}-{i + 1}",
                [self.subdir, str(self.domain_id)],
            )

        plan.get("plan", {}).pop("reviewer_notes", None)
        markdown_document_generator(
            plan, "tech_plan_after_reviews", [self.subdir, str(self.domain_id)]
        )

        return plan

    def code_implementation_phase(
        self,
        plan: dict,
        task: str,
        invocation_id_prefix: str,
    ) -> str:
        config = Configuration(
            initial_prompt_context=(
                f"TASK:\n{task}\nAPPROVED IMPLEMENTATION PLAN:\n{json.dumps(plan)}\n\n"
                "Implement the approved plan exactly as written.\n"
                "The plan has already been reviewed and approved.\n"
                "Do not question whether planned file creation or modification should occur."
            ),
            coder_agent_ref=self.coder,
            review_agent_ref=self.code_review,
            max_iterations=MAX_CODE_ITERS,
            task=task,
            plan=plan,
        )

        return CodeExecutionFramework().execute(
            config,
            [self.subdir, str(self.domain_id)],
            invocation_id_prefix,
            self.watcher,
        )

    def tech_lead_review_phase(
        self,
        code_summary: str,
        arch: dict,
        plan: dict,
        task: str,
        tech_lead_final_review: Optional[dict],
        invocation_id: str,
    ) -> dict:
        extra_prompt = ""
        if tech_lead_final_review:
            extra_prompt = f"PREVIOUS FEEDBACK:\n{json.dumps(tech_lead_final_review)}\n"
        return run_json_agent(
            self.tech_lead_final,
            f"TASK:\n{task}\n"
            f"APPROVED ARCHITECTURE:\n{json.dumps(arch)}\n"
            f"APPROVED IMPLEMENTATION PLAN:\n{json.dumps(plan)}\n"
            f"{extra_prompt}"
            "<aggregate_implementation_summary>\n"
            f"{code_summary}\n"
            "</aggregate_implementation_summary>\n",
            invocation_id,
            [self.subdir, str(self.domain_id)],
        )

    def revision_loops(
        self,
        tech_lead_review: dict,
        task: str,
        plan: dict,
        invocation_id_prefix: str,
        reset_coder: bool = False,
    ) -> str:
        if reset_coder:
            log(
                "SYSTEM",
                f"Resetting coder context: {tech_lead_review.get('reset_reason', '')}",
            )

            self.coder.reset(f"{invocation_id_prefix}-start")

            initial_prompt_context = (
                f"TASK:\n{task}\n"
                f"APPROVED IMPLEMENTATION PLAN:\n{json.dumps(plan)}\n"
                f"TECH LEAD FEEDBACK:\n{json.dumps(tech_lead_review)}\n\n"
                "Re-implement from a clean context using the task, plan, and tech lead feedback."
            )
        else:
            initial_prompt_context = (
                f"TECH LEAD FEEDBACK TO ADDRESS:\n{json.dumps(tech_lead_review)}"
            )

        config = Configuration(
            initial_prompt_context=initial_prompt_context,
            coder_agent_ref=self.coder,
            review_agent_ref=self.code_review,
            max_iterations=MAX_CODE_ITERS,
            task=task,
            plan=plan,
        )

        return CodeExecutionFramework().execute(
            config,
            [self.subdir, str(self.domain_id)],
            invocation_id_prefix,
            self.watcher,
        )


def wrap_text(text: str) -> str:
    return f"<text>\n{text}\n</text>"
