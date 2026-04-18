
# =========================
# ORCHESTRATOR
# =========================

import json
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Optional

import prompts
import schemas
from agent import Agent
from utils import run_json_agent, log, assert_not_empty, markdown_document_generator
from config import MAX_PLAN_ITERS, MAX_CODE_ITERS, MAX_TOP_ITERATIONS
from execution_framework import CodeExecutionFramework, Configuration


class Orchestrator:
    def __init__(self, task: str):
        self.task = task
        self.subdir = time.strftime('%Y-%m-%d_%H-%M-%S')

        self.product_manager = Agent(
            "product_manager",
            prompts.PRODUCT_MANAGER_PROMPT,
            schemas.PRODUCT_MANAGER_SCHEMA,
            ephemeral=True,
            timeout='30m'
        )
        self.pm_synth = Agent(
            "pm_synth",
            prompts.PM_SYNTHESIZER_PROMPT,
            schemas.PM_SYNTHESIZER_SCHEMA,
            timeout='30m'
        )
        self.pm_review = Agent(
            "pm_review",
            prompts.PM_REVIEW_PROMPT,
            schemas.PM_REVIEW_SCHEMA,
            ephemeral=True,
            timeout='30m'
        )

        self.arch = Agent(
            "arch",
            prompts.ARCH_PROMPT,
            schemas.ARCH_SCHEMA,
            timeout='40m'
        )
        self.tech_lead = Agent(
            "tech_lead",
            prompts.PLAN_PROMPT,
            schemas.PLAN_SCHEMA,
            timeout='60m'
        )
        self.coder = Agent(
            "coder",
            prompts.CODER_PROMPT,
            schemas.CODER_SCHEMA
        )

        self.arch_review = Agent(
            "arch_review",
            prompts.ARCH_REVIEW_PROMPT,
            schemas.ARCH_REVIEW_SCHEMA,
            ephemeral=True,
            timeout='30m'
        )
        self.plan_review = Agent(
            "plan_review",
            prompts.PLAN_REVIEW_PROMPT,
            schemas.PLAN_REVIEW_SCHEMA,
            ephemeral=True,
            timeout='30m'
        )
        self.code_review = Agent(
            "code_review",
            prompts.CODE_REVIEW_PROMPT,
            schemas.CODE_REVIEW_SCHEMA,
            ephemeral=True,
            timeout='60m'
        )

        self.tech_lead_final = Agent(
            "tech_lead_final",
            prompts.TECH_LEAD_FINAL_PROMPT,
            schemas.TECH_LEAD_FINAL_SCHEMA,
            ephemeral=True,
            timeout='60m'
        )
        self.arch_final = Agent(
            "arch_final",
            prompts.ARCH_FINAL_PROMPT,
            schemas.ARCH_FINAL_SCHEMA,
            ephemeral=True,
            timeout='60m'
        )

        self.decomposition = Agent(
            "decomposition",
            prompts.SYSTEM_DECOMPOSITION_PROMPT,
            schemas.SYSTEM_DECOMPOSITION_SCHEMA,
            timeout='40m'
        )
        self.decomposition_review = Agent(
            "decomposition_review",
            prompts.SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
            schemas.SYSTEM_DECOMPOSITION_REVIEW_SCHEMA,
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
        candidates = []
        with ThreadPoolExecutor(max_workers=3) as pool:
            futures = []
            for prompt in [
                "Bias toward minimal scope and preserving the literal user request.",
                "Bias toward UX completeness, validation rules, and expected user behavior.",
                "Bias toward implementation simplicity and minimal engineering risk.",
                "Bias toward edge cases, state transitions, and failure scenarios.",
                "Bias toward preserving existing system behavior and minimizing changes to current workflows.",
                "Bias toward permissions, roles, ownership boundaries, and access control behavior.",
                "Bias toward data model implications, persistence behavior, lifecycle management, and state consistency.",
                "Bias toward API behavior, input/output contracts, validation, and error handling.",
                "Bias toward reporting, auditability, notifications, logging, and observability requirements.",
                "Bias toward backward compatibility, migration concerns, rollout safety, and minimizing disruption to existing users.",
                "Bias toward operational concerns such as performance, scalability, concurrency, and long-term maintainability.",
                "Bias toward identifying the smallest possible implementation that still fully satisfies the request.",
                "Bias toward identifying where the request may be overcomplicated, unnecessary, or better solved through a smaller existing workflow change instead of a new feature.",
                "Bias toward preserving only what is explicitly stated by the user. Avoid assumptions unless absolutely necessary.",
                "Bias toward identifying the user's likely business goal and ensuring the specification solves that goal with the smallest possible feature set.",
            ]:
                future = pool.submit(
                    run_json_agent,
                    self.product_manager,
                    f"USER REQUEST:\n{self.task}\n\n"
                    f"TASK:\nProduce a focused engineering-ready specification.\n{prompt}"
                )
                futures.append(future)
            for future in as_completed(futures):
                candidates.append(future.result())

        choices = "\n".join([f"CANDIDATE {i + 1}:\n{json.dumps(v)}\n" for i, v in enumerate(candidates)])
        rephrased_task = run_json_agent(
            self.pm_synth,
            f"ORIGINAL USER REQUEST:\n{self.task}\n\n{choices}"
            """TASK:
Select the single best interpretation of the original user request.
Preserve only the minimum assumptions necessary.
Reject speculative scope expansion.
            """
        )
        for iteration in range(MAX_PLAN_ITERS):
            review = run_json_agent(
                self.pm_review,
                f"ORIGINAL USER REQUEST:\n{self.task}\n\n"
                f"SYNTHESIZED SPECIFICATION:\n{json.dumps(rephrased_task)}\n\n"
                f"ATTEMPT: {iteration + 1}/{MAX_PLAN_ITERS}\n\n"
                """TASK:
Review whether the synthesized specification correctly preserves the original user intent.
Reject only if the specification is ambiguous, speculative, internally inconsistent, or over-expanded.
                """
            )

            if self.review_ok(review):
                break

            if self.should_reset(review):
                log(
                    "SYSTEM",
                    f"Resetting PM Synthesizer context: {review.get('reset_reason', '')}"
                )

                self.pm_synth.reset()

                revision_prompt = (
                    f"ORIGINAL USER REQUEST:\n{self.task}\n\n{choices}"
                    f"PREVIOUS SYNTHESIZED SPECIFICATION:\n{json.dumps(rephrased_task)}\n"
                    f"REVIEW FEEDBACK:\n{json.dumps(review)}\n"
                    """TASK:
Revise the synthesized specification to address the review feedback while preserving the original user intent and keeping the scope minimal.
                    """
                )
            else:
                revision_prompt = (
                    f"REVISE SYNTHESIZED SPECIFICATION based on feedback:\n{json.dumps(review)}"
                )

            rephrased_task = run_json_agent(self.pm_synth, revision_prompt)

        markdown_document_generator(
            rephrased_task,
            'product_manager_final',
            [self.subdir]
        )
        out = rephrased_task['task_specification']
        files = rephrased_task.get('files')
        if isinstance(files, list) and files:
            out += "\n\nMentioned files:\n"
            for file in files:
                out += f"* {file}\n"
        proper_nouns = rephrased_task.get('proper_nouns')
        if isinstance(proper_nouns, list) and proper_nouns:
            out += "\n\nMentioned proper nouns:\n"
            for proper_noun in proper_nouns:
                out += f"* {proper_noun}\n"
        facts = rephrased_task.get('facts')
        if isinstance(facts, list) and facts:
            out += "\n\nStated facts:\n"
            for fact in facts:
                out += f"* {fact}\n"
        return out

    def run(self):
        root_task = self.pm_transformation_workflow()
        decomposition = self.decomposition_workflow(root_task)
        domains = decomposition.get('decomposition', {}).get('domains', [])

        for domain_index, domain in enumerate(domains):
            self.arch.reset()
            self.tech_lead.reset()
            self.coder.reset()
            final_feedback = None

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
                code_summaries = [code_summary]
                tech_lead_final_review = None

                for iteration in range(MAX_TOP_ITERATIONS):
                    tech_lead_final_review = self.tech_lead_review_phase(
                        code_summary,
                        arch,
                        plan,
                        task,
                        tech_lead_final_review,
                    )

                    if self.review_ok(tech_lead_final_review):
                        break

                    log(
                        "SYSTEM",
                        "Tech lead feedback received - revising implementation"
                    )

                    code_summary = self.revision_loops(
                        tech_lead_review=tech_lead_final_review,
                        task=task,
                        plan=plan,
                        reset_coder=self.should_reset(tech_lead_final_review)
                    )
                    code_summaries.append(code_summary)

                final_feedback = run_json_agent(
                    self.arch_final,
                    f"TASK:\n{task}\n"
                    f"ARCHITECTURE:\n{json.dumps(arch)}\n"
                    f"PLAN:\n{json.dumps(plan)}\n"
                    f"CODE IMPLEMENTATION SUMMARY from each iteration:\n"
                    f"{'\n\n'.join(code_summaries)}"
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

    def tech_lead_review_phase(self, code_summary: str, arch: dict, plan: dict, task: str, tech_lead_final_review: Optional[dict]) -> dict:
        extra_prompt = ""
        if tech_lead_final_review:
            extra_prompt = f"PREVIOUS FEEDBACK:\n{json.dumps(tech_lead_final_review)}\n"
        return run_json_agent(
            self.tech_lead_final,
            f"TASK:\n{task}\n"
            f"ARCHITECTURE:\n{json.dumps(arch)}\n"
            f"PLAN:\n{json.dumps(plan)}\n"
            f"{extra_prompt}"
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
