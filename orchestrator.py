
# =========================
# ORCHESTRATOR
# =========================

import json
import time
from agent import Agent
from prompts import (
    ARCH_PROMPT, PLAN_PROMPT, CODER_PROMPT,
    ARCH_REVIEW_PROMPT, PLAN_REVIEW_PROMPT, CODE_REVIEW_PROMPT,
    TECH_LEAD_FINAL_PROMPT, ARCH_FINAL_PROMPT,
    PRODUCT_MANAGER_PROMPT,
    SYSTEM_DECOMPOSITION_PROMPT, SYSTEM_DECOMPOSITION_REVIEW_PROMPT
)
from utils import run_json_agent, log, assert_not_empty, markdown_document_generator
from config import MAX_PLAN_ITERS, MAX_CODE_ITERS, MAX_TOP_ITERATIONS
from execution_framework import CodeExecutionFramework, Configuration


class Orchestrator:
    def __init__(self, task: str):
        self.task = task
        self.subdir = time.strftime('%Y-%m-%d_%H-%M-%S')

        self.product_manager = Agent(
            "product_manager",
            PRODUCT_MANAGER_PROMPT,
            timeout='15m'
        )

        self.arch = Agent("arch", ARCH_PROMPT, timeout='40m')
        self.tech_lead = Agent("tech_lead", PLAN_PROMPT, timeout='60m')
        self.coder = Agent("coder", CODER_PROMPT)

        self.arch_review = Agent(
            "arch_review",
            ARCH_REVIEW_PROMPT,
            ephemeral=True,
            timeout='30m'
        )
        self.plan_review = Agent(
            "plan_review",
            PLAN_REVIEW_PROMPT,
            ephemeral=True,
            timeout='30m'
        )
        self.code_review = Agent(
            "code_review",
            CODE_REVIEW_PROMPT,
            ephemeral=True,
            timeout='60m'
        )

        self.tech_lead_final = Agent(
            "tech_lead_final",
            TECH_LEAD_FINAL_PROMPT,
            ephemeral=True,
            timeout='60m'
        )
        self.arch_final = Agent(
            "arch_final",
            ARCH_FINAL_PROMPT,
            ephemeral=True,
            timeout='60m'
        )

        self.decomposition = Agent(
            "decomposition",
            SYSTEM_DECOMPOSITION_PROMPT,
            timeout='40m'
        )
        self.decomposition_review = Agent(
            "decomposition_review",
            SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
            ephemeral=True,
            timeout='30m'
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
        run_json_agent(
            self.product_manager,
            f"USER REQUEST:\n{self.task}\n\n"
            """TASK:\n
Refine this request into something engineering-ready.

Preserve the user's core intent exactly.

You should:
* identify missing requirements
* make reasonable assumptions
* define expected behavior
* tighten vague language
* add acceptance-oriented details
* avoid expanding scope beyond the likely intent

Do not ask questions back. Make the best product decisions you can.
            """
        )

        run_json_agent(
            self.product_manager,
            """
Review your own task specification.

Look for:
* ambiguity
* missing edge cases
* undefined behavior
* missing constraints
* places where engineering could misinterpret the task
* places where the scope may have drifted too far from the original user intent

Tighten the specification while preserving the original request.
            """,
        )

        rephrased_task = run_json_agent(
            self.product_manager,
            """
Review the task specification one final time.

Your goal is to ensure the specification is still tightly aligned with the original user request.

Remove:
* unnecessary scope expansion
* speculative features
* optional enhancements not clearly implied by the request
* implementation-level details
* defensive behavior not explicitly required

Keep:
* only requirements that are necessary to satisfy the user's intent
* only assumptions that are required to make the task implementable
* only edge cases that are realistic and important

Ensure the final specification is minimal, clear, and buildable.
            """,
        )

        markdown_document_generator(
            rephrased_task,
            'product_manager_final',
            [self.subdir]
        )
        return rephrased_task['task_specification']

    def run(self):
        root_task = self.pm_transformation_workflow()
        decomposition = self.decomposition_workflow(root_task)
        domains = decomposition.get('decomposition', {}).get('domains', [])
        final_feedback = None

        for domain_index, domain in enumerate(domains):
            self.arch.reset()
            self.tech_lead.reset()
            self.coder.reset()

            task = domain.get('architect_input')
            domain_id = domain.get('id', domain_index + 1)

            if not task:
                continue

            for iteration in range(MAX_TOP_ITERATIONS):
                log(
                    "ITERATION",
                    f"Starting iteration {iteration + 1}/{MAX_TOP_ITERATIONS}"
                    f" for domain {domain_index + 1}/{len(domains)}"
                )

                arch = self.architecture_design_phase(
                    final_feedback,
                    task,
                    domain_id
                )

                plan = self.plan_creation_phase(
                    arch,
                    task,
                    domain_id
                )

                code_summary = self.code_implementation_phase(
                    plan,
                    task
                )

                tech_lead_final_review = self.tech_lead_review_phase(
                    code_summary,
                    arch,
                    plan,
                    task
                )

                if not self.review_ok(tech_lead_final_review):
                    log(
                        "SYSTEM",
                        "Tech lead feedback received - revising implementation"
                    )

                    code_summary += '\n\n' + self.revision_loops(
                        tech_lead_review=tech_lead_final_review,
                        task=task,
                        plan=plan,
                        reset_coder=self.should_reset(tech_lead_final_review)
                    )

                final_feedback = run_json_agent(
                    self.arch_final,
                    f"TASK:\n{task}\n"
                    f"ARCHITECTURE:\n{json.dumps(arch)}\n"
                    f"PLAN:\n{json.dumps(plan)}\n"
                    f"CODE IMPLEMENTATION SUMMARY from each iteration:\n{code_summary}"
                )

                if self.review_ok(final_feedback):
                    break

    def decomposition_workflow(self, task: str) -> dict:
        decomposition_result = run_json_agent(
            self.decomposition,
            f"TASK:\n{task}"
        )

        assert_not_empty(decomposition_result, "DECOMPOSITION")

        for i in range(MAX_PLAN_ITERS):
            decomposition_review = run_json_agent(
                self.decomposition_review,
                f"TASK:\n{task}\n"
                f"ATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\n"
                f"DECOMPOSITION TO REVIEW:\n{json.dumps(decomposition_result)}"
            )

            if self.review_ok(decomposition_review):
                break

            if self.should_reset(decomposition_review):
                log(
                    "SYSTEM",
                    f"Resetting decomposition context: {decomposition_review.get('reset_reason', '')}"
                )

                self.decomposition.reset()

                revision_prompt = (
                    f"TASK:\n{task}\n"
                    f"PREVIOUS DECOMPOSITION:\n{json.dumps(decomposition_result)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(decomposition_review)}\n\n"
                    "Rebuild the decomposition from scratch using the original task and review feedback."
                )
            else:
                revision_prompt = (
                    f"REVISE DECOMPOSITION based on feedback:\n{json.dumps(decomposition_review)}"
                )

            decomposition_result = run_json_agent(
                self.decomposition,
                revision_prompt
            )

        decomposition_result.get('decomposition', {}).pop('reviewer_notes', None)
        markdown_document_generator(
            decomposition_result,
            'decomposition_final',
            [self.subdir]
        )

        return decomposition_result

    def architecture_design_phase(self, final_feedback: dict, task: str, domain_id: int) -> dict:
        if final_feedback:
            initial_prompt = (
                f"REVISE ARCHITECTURE based on feedback post implementation:\n{json.dumps(final_feedback)}"
            )
        else:
            initial_prompt = f"TASK:\n{task}"

        arch = run_json_agent(self.arch, initial_prompt)

        for i in range(MAX_PLAN_ITERS):
            arch_review = run_json_agent(
                self.arch_review,
                f"TASK:\n{task}\n"
                f"ATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\n"
                f"ARCHITECTURE TO REVIEW:\n{json.dumps(arch)}"
            )

            if self.review_ok(arch_review):
                break

            if self.should_reset(arch_review):
                log(
                    "SYSTEM",
                    f"Resetting architect context: {arch_review.get('reset_reason', '')}"
                )

                self.arch.reset()

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

            arch = run_json_agent(self.arch, revision_prompt)

        arch.get('architecture', {}).pop('reviewer_notes', None)
        markdown_document_generator(
            arch,
            'architecture_after_reviews',
            [self.subdir, str(domain_id)]
        )

        return arch

    def plan_creation_phase(self, arch: dict, task: str, domain_id: int) -> dict:
        plan = run_json_agent(
            self.tech_lead,
            f"TASK:\n{task}\nARCHITECTURE:\n{json.dumps(arch)}"
        )

        for i in range(MAX_PLAN_ITERS):
            plan_review = run_json_agent(
                self.plan_review,
                f"TASK:\n{task}\n"
                f"ATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\n"
                f"ARCHITECTURE:\n{json.dumps(arch)}\n"
                f"PLAN TO REVIEW:\n{json.dumps(plan)}"
            )

            if self.review_ok(plan_review):
                break

            if self.should_reset(plan_review):
                log(
                    "SYSTEM",
                    f"Resetting tech lead context: {plan_review.get('reset_reason', '')}"
                )

                self.tech_lead.reset()

                revision_prompt = (
                    f"TASK:\n{task}\n"
                    f"ARCHITECTURE:\n{json.dumps(arch)}\n"
                    f"PREVIOUS PLAN:\n{json.dumps(plan)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(plan_review)}\n\n"
                    "Rebuild the plan from scratch using the task, architecture, and review feedback."
                )
            else:
                revision_prompt = (
                    f"REVISE PLAN based on feedback:\n{json.dumps(plan_review)}"
                )

            plan = run_json_agent(self.tech_lead, revision_prompt)

        plan.get('plan', {}).pop('reviewer_notes', None)
        markdown_document_generator(
            plan,
            'tech_plan_after_reviews',
            [self.subdir, str(domain_id)]
        )

        return plan

    def code_implementation_phase(self, plan: dict, task: str) -> str:
        config = Configuration(
            initial_prompt_context=f"TASK:\n{task}\nPLAN:\n{json.dumps(plan)}",
            coder_agent_ref=self.coder,
            review_agent_ref=self.code_review,
            max_iterations=MAX_CODE_ITERS,
            task=task,
            plan=plan,
        )

        return CodeExecutionFramework().execute(config)

    def tech_lead_review_phase(self, code_summary: str, arch: dict, plan: dict, task: str) -> dict:
        return run_json_agent(
            self.tech_lead_final,
            f"TASK:\n{task}\n"
            f"ARCHITECTURE:\n{json.dumps(arch)}\n"
            f"PLAN:\n{json.dumps(plan)}\n"
            f"CODE IMPLEMENTATION SUMMARY from each iteration:\n{code_summary}"
        )

    def revision_loops(
        self,
        tech_lead_review: dict,
        task: str,
        plan: dict,
        reset_coder: bool = False
    ) -> str:
        if reset_coder:
            log(
                "SYSTEM",
                f"Resetting coder context: {tech_lead_review.get('reset_reason', '')}"
            )

            self.coder.reset()

            initial_prompt_context = (
                f"TASK:\n{task}\n"
                f"PLAN:\n{json.dumps(plan)}\n"
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

        return CodeExecutionFramework().execute(config)
