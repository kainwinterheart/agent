# =========================
# ORCHESTRATOR
# =========================

import json
import re
from agent import Agent
from prompts import (
    ARCH_PROMPT, PLAN_PROMPT, CODER_PROMPT,
    ARCH_REVIEW_PROMPT, PLAN_REVIEW_PROMPT, CODE_REVIEW_PROMPT,
    TECH_LEAD_FINAL_PROMPT, ARCH_FINAL_PROMPT,
    PRODUCT_MANAGER_PROMPT
)
from utils import run_json_agent, log, assert_not_empty, markdown_document_generator, enforce_retention_policy
from config import MAX_PLAN_ITERS, MAX_CODE_ITERS, MAX_TOP_ITERATIONS
from execution_framework import CodeExecutionFramework, Configuration


class Orchestrator:
    def __init__(self, task: str):
        self.task = task
        
        # Initialize Product Manager agent as first step
        self.product_manager = Agent("product_manager", PRODUCT_MANAGER_PROMPT, timeout='15m')
        
        # Initialize agents
        self.arch = Agent("arch", ARCH_PROMPT, timeout='40m')
        self.tech_lead = Agent("tech_lead", PLAN_PROMPT, timeout='60m')
        self.coder = Agent("coder", CODER_PROMPT)
        
        # Initialize reviewers
        self.arch_review = Agent("arch_review", ARCH_REVIEW_PROMPT, ephemeral=True, timeout='30m')
        self.plan_review = Agent("plan_review", PLAN_REVIEW_PROMPT, ephemeral=True, timeout='30m')
        self.code_review = Agent("code_review", CODE_REVIEW_PROMPT, ephemeral=True, timeout='60m')
        
        # Initialize final reviewers
        self.tech_lead_final = Agent("tech_lead_final", TECH_LEAD_FINAL_PROMPT, ephemeral=True, timeout='60m')
        self.arch_final = Agent("arch_final", ARCH_FINAL_PROMPT, ephemeral=True, timeout='60m')

    def review_ok(self, review: dict) -> bool:
        """Check if review passed all checks."""
        approved = review.get("approved", False)
        
        # Log review status
        log("REVIEW STATUS", f"approved={approved}")
        
        # Check for high severity issues
        issues = review.get("issues", [])
        for issue in issues:
            if issue.get("severity") == "high":
                return False
        
        return approved

    def pm_transformation_workflow(self):
        run_json_agent(
            self.product_manager,
            f"USER REQUEST:\n{self.task}\n\n"
            """TASK:\n
Refine this request into something engineering-ready.

Preserve the user's core intent exactly.

You should:
- identify missing requirements
- make reasonable assumptions
- define expected behavior
- tighten vague language
- add acceptance-oriented details
- avoid expanding scope beyond the likely intent

Do not ask questions back. Make the best product decisions you can.
            """
        )

        run_json_agent(
            self.product_manager,
            """
Review your own task specification.

Look for:
- ambiguity
- missing edge cases
- undefined behavior
- missing constraints
- places where engineering could misinterpret the task
- places where the scope may have drifted too far from the original user intent

Tighten the specification while preserving the original request.
            """,
        )

        self.rephrased_task = run_json_agent(
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

        markdown_document_generator(self.rephrased_task, 'product_manager_final')
        enforce_retention_policy('product_manager_final')

    def run(self):
        """Main workflow execution with comprehensive error handling."""
        # Execute PM transformation first (synchronous blocking step)
        self.pm_transformation_workflow()
        
        final_feedback = None
        
        for iteration in range(MAX_TOP_ITERATIONS):  # MAX_TOP_ITERERS
            log("ITERATION", f"Starting iteration {iteration + 1}/{MAX_TOP_ITERATIONS}")
            
            # Phase 1: Architecture design - consume exclusively PM-produced task_specification
            arch = self.architecture_design_phase(final_feedback)
            
            # Phase 2: Plan creation
            plan = self.plan_creation_phase(arch)
            
            # Phase 3: Code implementation
            code_summary = self.code_implementation_phase(plan)
            
            # Phase 4: Tech lead review
            tech_lead_final_review = self.tech_lead_review_phase(code_summary, arch, plan)
            
            if not self.review_ok(tech_lead_final_review):
                log("SYSTEM", "Tech lead feedback received - revising implementation")
                code_summary += '\n\n' + self.revision_loops(tech_lead_final_review)
            
            # Final architect review
            final_feedback = run_json_agent(
                self.arch_final,
                f"TASK:\n{self.rephrased_task['task_specification']}\nARCHITECTURE:\n{json.dumps(arch)}\nPLAN:\n{json.dumps(plan)}\nCODE IMPLEMENTATION SUMMARY from each iteration:\n{code_summary}"
            )

            if self.review_ok(final_feedback):
                break

    def architecture_design_phase(self, final_feedback: dict) -> dict:
        """Phase 1: Architecture design with review."""
        arch = run_json_agent(
            self.arch,
            f"REVISE ARCHITECTURE based on feedback post implementation:\n{json.dumps(final_feedback)}" if final_feedback else f"TASK:\n{self.rephrased_task['task_specification']}"
        )
        
        for i in range(MAX_PLAN_ITERS):  # MAX_PLAN_ITERS
            assert_not_empty(arch, "ARCHITECTURE")
            arch_review = run_json_agent(
                self.arch_review,
                f"TASK:\n{self.rephrased_task['task_specification']}\nATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\nARCHITECTURE TO REVIEW:\n{json.dumps(arch)}"
            )
            
            if self.review_ok(arch_review):
                log("SYSTEM", "Architecture approved by architect")
                break
            
            arch = run_json_agent(
                self.arch,
                f"REVISE ARCHITECTURE based on feedback:\n{json.dumps(arch_review)}"
            )
        
        markdown_document_generator(arch, 'architecture_after_reviews')
        enforce_retention_policy('architecture_after_reviews')
        return arch

    def plan_creation_phase(self, arch: dict) -> dict:
        """Phase 2: Plan creation with review."""
        plan = run_json_agent(
            self.tech_lead,
            f"TASK:\n{self.rephrased_task['task_specification']}\nARCHITECTURE:\n{json.dumps(arch)}"
        )
        
        for i in range(MAX_PLAN_ITERS):  # MAX_PLAN_ITERS
            assert_not_empty(plan, "PLAN")
            plan_review = run_json_agent(
                self.plan_review,
                f"TASK:\n{self.rephrased_task['task_specification']}\nATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\nARCHITECTURE:\n{json.dumps(arch)}\nPLAN TO REVIEW:\n{json.dumps(plan)}"
            )
            
            if self.review_ok(plan_review):
                log("SYSTEM", "Plan approved by tech lead")
                break
            
            plan = run_json_agent(
                self.tech_lead,
                f"REVISE PLAN based on feedback:\n{json.dumps(plan_review)}"
            )
        
        markdown_document_generator(plan, 'tech_plan_after_reviews')
        enforce_retention_policy('tech_plan_after_reviews')
        return plan
        
    def code_implementation_phase(self, plan: dict) -> str:
        """Phase 3: Code implementation with review using unified execution framework."""
        # Create configuration for unified execution
        config = Configuration(
            initial_prompt_context=f'TASK:\n{self.rephrased_task["task_specification"]}\nPLAN:\n{json.dumps(plan)}',
            coder_agent_ref=self.coder,
            review_agent_ref=self.code_review,
            max_iterations=MAX_CODE_ITERS,
        )
        
        # Execute unified framework
        return CodeExecutionFramework().execute(config)

    def tech_lead_review_phase(self, code_summary: str, arch: dict, plan: dict) -> dict:
        """Phase 4: Tech lead final review.
        
        Parameters:
            code_summary (str): Summary of code implementation for review
            arch (dict): Architecture design - used in prompt construction with json.dumps(arch)
            plan (dict): Implementation plan - used in prompt construction with json.dumps(plan)
        """
        # Note: arch and plan parameters are currently unused but kept for future extensibility
        tech_lead_final_review = run_json_agent(
            self.tech_lead_final,
            f"TASK:\n{self.rephrased_task['task_specification']}\nARCHITECTURE:\n{json.dumps(arch)}\nPLAN:\n{json.dumps(plan)}\nCODE IMPLEMENTATION SUMMARY from each iteration:\n{code_summary}"
        )
        
        assert_not_empty(tech_lead_final_review, "TECH_LEAD_FINAL")
        return tech_lead_final_review

    def revision_loops(self, tech_lead_review: dict) -> str:
        # Create configuration for unified execution with additional constraints
        config = Configuration(
            initial_prompt_context=f'TECH LEAD FEEDBACK TO ADDRESS:\n{json.dumps(tech_lead_review)}',
            coder_agent_ref=self.coder,
            review_agent_ref=self.code_review,
            max_iterations=MAX_CODE_ITERS,
        )
        
        # Execute unified framework
        return CodeExecutionFramework().execute(config)
