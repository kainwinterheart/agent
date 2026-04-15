# =========================
# ORCHESTRATOR
# =========================

import json
import re
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
        
        # Initialize Product Manager agent as first step
        self.product_manager = Agent("product_manager", PRODUCT_MANAGER_PROMPT, timeout='15m')
        
        # Initialize reviewers
        self.arch_review = Agent("arch_review", ARCH_REVIEW_PROMPT, ephemeral=True, timeout='30m')
        self.plan_review = Agent("plan_review", PLAN_REVIEW_PROMPT, ephemeral=True, timeout='30m')
        self.code_review = Agent("code_review", CODE_REVIEW_PROMPT, ephemeral=True, timeout='60m')
        
        # Initialize final reviewers
        self.tech_lead_final = Agent("tech_lead_final", TECH_LEAD_FINAL_PROMPT, ephemeral=True, timeout='60m')
        self.arch_final = Agent("arch_final", ARCH_FINAL_PROMPT, ephemeral=True, timeout='60m')
        
        # Initialize decomposition agents
        self.decomposition = Agent("decomposition", SYSTEM_DECOMPOSITION_PROMPT, timeout='40m')
        self.decomposition_review = Agent("decomposition_review", SYSTEM_DECOMPOSITION_REVIEW_PROMPT, ephemeral=True, timeout='30m')

    def reset(self) -> None:
        # Initialize domain-specific agents
        self.arch = Agent("arch", ARCH_PROMPT, timeout='40m')
        self.tech_lead = Agent("tech_lead", PLAN_PROMPT, timeout='60m')
        self.coder = Agent("coder", CODER_PROMPT)

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

        markdown_document_generator(rephrased_task, 'product_manager_final', self.subdir)
        return rephrased_task['task_specification']

    def run(self):
        """Main workflow execution with comprehensive error handling."""
        # Execute PM transformation first (synchronous blocking step)
        root_task = self.pm_transformation_workflow()
        
        # Execute decomposition workflow to get architect_inputs for each domain
        decomposition = self.decomposition_workflow(root_task)
        domains = decomposition.get('decomposition', {}).get('domains', [])
        final_feedback = None
        
        for domain_index, domain in enumerate(domains):
            self.reset()
            task = domain.get('architect_input')
            if not task:
                continue
            for iteration in range(MAX_TOP_ITERATIONS):  # MAX_TOP_ITERERS
                log("ITERATION", f"Starting iteration {iteration + 1}/{MAX_TOP_ITERATIONS} for domain {domain_index + 1}/{len(domains)}")
                
                # Phase 1: Architecture design per-domain
                arch = self.architecture_design_phase(final_feedback, task)
                
                # Phase 2: Plan creation per-domain
                plan = self.plan_creation_phase(arch, task)
                
                # Phase 3: Code implementation per-domain
                code_summary = self.code_implementation_phase(plan, task)
                
                # Phase 4: Tech lead review per-domain with domain-specific context
                tech_lead_final_review = self.tech_lead_review_phase(code_summary, arch, plan, task)
                
                if not self.review_ok(tech_lead_final_review):
                    log("SYSTEM", "Tech lead feedback received - revising implementation")
                    code_summary += '\n\n' + self.revision_loops(tech_lead_final_review)
                
                # Final architect review per-domain with domain-specific context
                final_feedback = run_json_agent(
                    self.arch_final,
                    f"TASK:\n{task}\nARCHITECTURE:\n{json.dumps(arch)}\nPLAN:\n{json.dumps(plan)}\nCODE IMPLEMENTATION SUMMARY from each iteration:\n{code_summary}"
                )
            
                if self.review_ok(final_feedback):
                    break

    def decomposition_workflow(self, task: str) -> list:
        """Execute decomposition workflow between PM and Architect phases.
        
        Returns:
            tuple: (valid_domains list, decomposition_content dict)
        """
        # Invoke Decomposition Agent with PM output as input
        decomposition_result = run_json_agent(
            self.decomposition,
            f"TASK:\n{task}"
        )
        
        assert_not_empty(decomposition_result, "DECOMPOSITION")
        
        # Review iteration until acceptance criteria met
        for i in range(MAX_PLAN_ITERS):
            decomposition_review = run_json_agent(
                self.decomposition_review,
                f"TASK:\n{task}\nATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\nDECOMPOSITION TO REVIEW:\n{json.dumps(decomposition_result)}"
            )
            
            # Check acceptance criteria: successful review pass with no rejection and valid structure
            if self.review_ok(decomposition_review):
                log("SYSTEM", "Decomposition approved by decomposition reviewer")
                break
            
            # Re-invoke Decomposition Agent based on feedback
            decomposition_result = run_json_agent(
                self.decomposition,
                f"REVISE DECOMPOSITION based on feedback:\n{json.dumps(decomposition_review)}"
            )
        
        assert_not_empty(decomposition_result, "DECOMPOSITION_REVIEWED")
        
        markdown_document_generator(decomposition_result, 'decomposition_final', self.subdir)
        return decomposition_result

    def architecture_design_phase(self, final_feedback: dict, task: str) -> dict:
        """Phase 1: Architecture design with review per-domain.
        
        Parameters:
            final_feedback (dict): Feedback from previous iteration for revision
            architect_inputs (list): List of domain-specific architect_input strings to use as context
        """
        arch = run_json_agent(
            self.arch,
            f"TASK:\n{task}" if not final_feedback else f"REVISE ARCHITECTURE based on feedback post implementation:\n{json.dumps(final_feedback)}"
        )
        
        for i in range(MAX_PLAN_ITERS):  # MAX_PLAN_ITERS
            assert_not_empty(arch, "ARCHITECTURE")
            arch_review = run_json_agent(
                self.arch_review,
                f"TASK:\n{task}\nATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\nARCHITECTURE TO REVIEW:\n{json.dumps(arch)}"
            )
            
            if self.review_ok(arch_review):
                log("SYSTEM", "Architecture approved by architect")
                break
            
            arch = run_json_agent(
                self.arch,
                f"REVISE ARCHITECTURE based on feedback:\n{json.dumps(arch_review)}"
            )
        
        markdown_document_generator(arch, 'architecture_after_reviews', self.subdir)
        return arch

    def plan_creation_phase(self, arch: dict, task: str) -> dict:
        """Phase 2: Plan creation with review per-domain.
        
        Parameters:
            arch (dict): Architecture design output from previous phase
            architect_inputs (list): List of domain-specific architect_input strings to use as context
        """
        plan = run_json_agent(
            self.tech_lead,
            f"TASK:\n{task}\nARCHITECTURE:\n{json.dumps(arch)}"
        )
        
        for i in range(MAX_PLAN_ITERS):  # MAX_PLAN_ITERS
            assert_not_empty(plan, "PLAN")
            plan_review = run_json_agent(
                self.plan_review,
                f"TASK:\n{task}\nATTEMPT: {i + 1}/{MAX_PLAN_ITERS}\nARCHITECTURE:\n{json.dumps(arch)}\nPLAN TO REVIEW:\n{json.dumps(plan)}"
            )
            
            if self.review_ok(plan_review):
                log("SYSTEM", "Plan approved by tech lead")
                break
            
            plan = run_json_agent(
                self.tech_lead,
                f"REVISE PLAN based on feedback:\n{json.dumps(plan_review)}"
            )
        
        markdown_document_generator(plan, 'tech_plan_after_reviews', self.subdir)
        return plan
        
    def code_implementation_phase(self, plan: dict, task: str) -> str:
        """Phase 3: Code implementation with review per-domain.
        
        Parameters:
            plan (dict): Implementation plan from previous phase
            architect_inputs (list): List of domain-specific architect_input strings to use as context
        """
        # Create configuration for unified execution with domain-specific context
        config = Configuration(
            initial_prompt_context=f"TASK:\n{task}\nPLAN:\n{json.dumps(plan)}",
            coder_agent_ref=self.coder,
            review_agent_ref=self.code_review,
            max_iterations=MAX_CODE_ITERS,
        )
        
        # Execute unified framework
        return CodeExecutionFramework().execute(config)

    def tech_lead_review_phase(self, code_summary: str, arch: dict, plan: dict, task: str) -> dict:
        """Phase 4: Tech lead final review with domain-specific context.
        
        Parameters:
            code_summary (str): Summary of code implementation for review
            arch (dict): Architecture design - used in prompt construction with json.dumps(arch)
            plan (dict): Implementation plan - used in prompt construction with json.dumps(plan)
            architect_inputs (list): List of domain-specific architect_input strings for context (optional, defaults to rephrased_task)
        """
        tech_lead_final_review = run_json_agent(
            self.tech_lead_final,
            f"TASK:\n{task}\nARCHITECTURE:\n{json.dumps(arch)}\nPLAN:\n{json.dumps(plan)}\nCODE IMPLEMENTATION SUMMARY from each iteration:\n{code_summary}"
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
