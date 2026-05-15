import json

next_steps = {
    "type": "array",
    "items": {
        "type": "string",
        "description": """
Ordered list of 1–5 concrete reasoning or analysis steps that are **required to complete this task within the agent’s role**, but have not yet been performed.

These steps represent only the following:
* Analytical steps (reasoning)
* Verification steps (code inspection, search, tracing)

Rules:
* Each step must be something this agent itself should do (not another agent or system)
* Do not propose implementation, external actions, or handoffs
* Do not expand scope beyond the assigned task
* Each step must address a specific gap, assumption, or incomplete part of the current output
* Prefer steps that clarify, verify, compute, or refine the existing solution
* Avoid vague phrasing (e.g., “think more”, “improve answer”)
* Do not repeat steps already completed

If no required reasoning steps are missing and the task is complete - leave this array empty.
""",
    },
}


approved = {
    "approved": {
        "type": "boolean",
        "description": "true/false; " + json.dumps("""
Approval is only valid if:
* All prior high-severity issues are either resolved or downgraded with explicit justification
* At least one paragraph explains why the design is now considered sound

CRITICAL: If the previous review contained high-severity issues, a transition to zero issues must include explicit justification for each.
            """.strip()),
    },
    "approved_confidence": {
        "type": "string",
        "enum": ["not_approved", "low", "medium", "high"],
    },
    "approved_reason": {
        "type": "string",
        "description": "if approved=true - explain in detail why exactly has the approval been given, provide evidence; of approved=false - set this field to empty string",
    },
    "resolved_issues": {
        "type": "array",
        "items": {
            "type": "string",
            "description": """
Issues cannot be removed without explanation.
If an issue is no longer present - move it here, with justification.

If no issues were resolved - leave this array empty.
                        """.strip(),
        },
    },
}

ARCH_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "architecture": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "overview": {
                    "type": "string",
                    "description": "high-level design aligned with existing system",
                },
                "reviewer_notes": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "notes about alignment with current system and key tradeoffs; must include: 'Why not reuse existing system?' explanation for any new component",
                    },
                },
                "components": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "additionalProperties": False,
                        "properties": {
                            "name": {
                                "type": "string",
                                "description": "component name (existing or new)",
                            },
                            "responsibility": {
                                "type": "string",
                                "description": "what it does",
                            },
                            "background": {
                                "type": "string",
                                "description": "why this fits into the existing system; if the component is new: (1) why existing components cannot be reused, and (2) why introducing a new component is the simplest correct solution",
                            },
                        },
                        "required": ["name", "responsibility", "background"],
                    },
                },
                "data_flow": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "how data moves through EXISTING and new components",
                    },
                },
                "tech_choices": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "choices that must align with current stack",
                    },
                },
                "constraints": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "assumptions, limitations, and known gaps in system understanding",
                    },
                },
            },
            "required": [
                "overview",
                "reviewer_notes",
                "components",
                "data_flow",
                "tech_choices",
                "constraints",
            ],
        },
        "next_steps": next_steps,
    },
    "required": ["architecture", "next_steps"],
}

PLAN_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "plan": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "what will be implemented and how it integrates into the existing system",
                },
                "reviewer_notes": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "notes about assumptions on current codebase and structure",
                    },
                },
                "files": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "additionalProperties": False,
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "relative/file.ext",
                            },
                            "purpose": {
                                "type": "string",
                                "description": "what this file or modification does",
                            },
                            "background": {
                                "type": "string",
                                "description": "why this belongs in this location in the existing system",
                            },
                        },
                        "required": ["path", "purpose", "background"],
                    },
                },
                "steps": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "additionalProperties": False,
                        "properties": {
                            "id": {
                                "type": "integer",
                                "description": "numeric step identifier",
                            },
                            "description": {
                                "type": "string",
                                "description": "specific step tied to real file changes",
                            },
                        },
                        "required": ["id", "description"],
                    },
                },
            },
            "required": ["summary", "reviewer_notes", "files", "steps"],
        },
        "next_steps": next_steps,
    },
    "required": ["plan", "next_steps"],
}

CODER_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "changes": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "path": {"type": "string", "description": "relative/file.ext"},
                    "status": {
                        "type": "string",
                        "enum": ["modified", "created", "unchanged_blocked"],
                    },
                    "blocked_reason": {
                        "type": "string",
                        "description": "required only when status is unchanged_blocked; set to empty string otherwise",
                    },
                    "brief_summary": {
                        "type": "string",
                        "description": "what was actually changed and how it integrates with existing code",
                    },
                    "exists_after_change": {
                        "type": "boolean",
                        "description": "true for created and modified files, false otherwise",
                    },
                    "diff_summary": {
                        "type": "string",
                        "description": "describe the actual code change",
                    },
                },
                "required": [
                    "path",
                    "status",
                    "brief_summary",
                    "blocked_reason",
                    "exists_after_change",
                    "diff_summary",
                ],
            },
        },
        "summary": {
            "type": "string",
            "description": "summary of all completed changes and how they fit into the existing system",
        },
        "reviewer_notes": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "notes about assumptions, blocked work, incomplete areas, missing context, or areas needing attention",
            },
        },
        "next_steps": next_steps,
    },
    "required": ["changes", "summary", "reviewer_notes", "next_steps"],
}

ARCH_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "design"},
                    "message": {"type": "string", "description": "issue description"},
                    "next_actions": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": """
Ordered list of 1–5 concrete, actionable instructions that the system design reviewer is issuing to the software architect agent.

These actions represent **external design work** that should be performed next to improve or complete the system architecture.

Rules:
* Each action must be something the software architect can directly update or define in the system design
* Use clear, imperative language (e.g., “Define…”, “Refine…”, “Add…”, “Clarify…”, “Restructure…”)
* Focus on specific design elements (components, interfaces, data flows, constraints), not broad architectural goals
* Do not include reasoning, justification, or analysis
* Do not restate problems — only prescribe actions
* Do not include internal thinking or validation steps (those belong in `next_steps`)
* Avoid vague instructions (e.g., “improve scalability”, “make it better”)
* Do not delegate beyond the software architect or introduce new roles
* Each action should map to a concrete change or addition in the architecture

If no further design changes are required, return an empty array.
""",
                        },
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
        },
        "next_steps": next_steps,
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
        "next_steps",
    ],
}

PLAN_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "implementation"},
                    "message": {"type": "string", "description": "issue description"},
                    "next_actions": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": """
Ordered list of 1–5 concrete, actionable instructions that the implementation plan reviewer is issuing to the tech lead agent.

These actions represent **external planning work** that should be performed next to improve or complete the implementation plan.

Rules:
* Each action must be something the tech lead can directly update in the implementation plan
* Use clear, imperative language (e.g., “Break down…”, “Add…”, “Sequence…”, “Specify…”, “Adjust…”)
* Focus on specific plan elements (task breakdowns, dependencies, sequencing, ownership, risks), not broad objectives
* Do not include reasoning, justification, or analysis
* Do not restate problems — only prescribe actions
* Do not include internal thinking or validation steps (those belong in `next_steps`)
* Avoid vague instructions (e.g., “improve plan”, “make it clearer”)
* Do not delegate beyond the tech lead or introduce new roles
* Each action should map to a concrete modification of the implementation plan

If no further planning changes are required, return an empty array.
""",
                        },
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
        },
        "next_steps": next_steps,
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
        "next_steps",
    ],
}

CODE_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "implementation"},
                    "message": {"type": "string", "description": "issue description"},
                    "next_actions": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": """
Ordered list of 1–5 concrete, actionable instructions that the code review agent is issuing to the coder agent.

These actions represent **external implementation work** that should be performed next to improve or complete the solution.

Rules:
* Each action must be something the coder agent can directly implement or modify in code
* Use clear, imperative language (e.g., “Refactor…”, “Add…”, “Fix…”, “Remove…”)
* Focus on specific, localized changes rather than broad goals
* Do not include reasoning, justification, or analysis
* Do not restate problems — only prescribe actions
* Do not include internal thinking or validation steps (those belong in `next_steps`)
* Avoid vague instructions (e.g., “improve this”, “optimize code”)
* Do not delegate beyond the coder agent or introduce new roles
* Each action should map to a concrete change in the codebase

If no further code changes are required, return an empty array.
""",
                        },
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
        },
        "next_steps": next_steps,
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
        "next_steps",
    ],
}

TECH_LEAD_FINAL_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "implementation"},
                    "message": {"type": "string", "description": "issue description"},
                    "next_actions": {
                        "type": "array",
                        "items": {"type": "string", "description": "actionable fixes"},
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
        },
        "next_steps": next_steps,
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
        "next_steps",
    ],
}

ARCH_FINAL_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "design"},
                    "message": {"type": "string", "description": "issue description"},
                    "next_actions": {
                        "type": "array",
                        "items": {"type": "string", "description": "actionable fixes"},
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
        },
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
    ],
}

PRODUCT_MANAGER_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "task_specification": {
            "type": "string",
            "description": "fully refined and engineering-ready task description",
        },
        "original_input_preserved": {"type": "boolean", "description": "true"},
        "clarifications_made": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "explicit assumptions, refinements, missing requirements filled in, and interpretation decisions",
            },
        },
        "files": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "if user request refers to any files - extract file names/pathes, and put them here as is",
            },
        },
        "proper_nouns": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "if user request refers to any proper nouns which are *not* file names/pathes - extract them, and put them here as is",
            },
        },
        "facts": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "if user request states specific facts - extract them, and put them here",
            },
        },
    },
    "required": [
        "task_specification",
        "original_input_preserved",
        "clarifications_made",
        "files",
        "proper_nouns",
        "facts",
    ],
}

PM_SYNTHESIZER_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "task_specification": {
            "type": "string",
            "description": "final refined engineering-ready task description",
        },
        "selected_candidate": {"type": "integer", "description": "1"},
        "selection_reason": {
            "type": "string",
            "description": "why this candidate was closest to the correct interpretation",
        },
        "rejected_candidates": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "candidate": {"type": "integer", "description": "candidate number"},
                    "reason": {
                        "type": "string",
                        "description": "why this interpretation was rejected",
                    },
                },
                "required": ["candidate", "reason"],
            },
        },
        "common_requirements": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "requirements most candidates agree on",
            },
        },
        "candidate_disagreements": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "common points of disagreement between most candidates",
            },
        },
        "speculative_expansions": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "what candidates clearly speculate on",
            },
        },
        "missing_but_necessary_details": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "details not commonly found in candidate interpretations that are clearly required",
            },
        },
        "clarifications_made": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "final assumptions and interpretation decisions",
            },
        },
        "files": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "if user request refers to any files - extract file names/pathes, and put them here as is",
            },
        },
        "proper_nouns": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "if user request refers to any proper nouns which are *not* file names/pathes - extract them, and put them here as is",
            },
        },
        "facts": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "if user request states specific facts - extract them, and put them here",
            },
        },
    },
    "required": [
        "task_specification",
        "selected_candidate",
        "selection_reason",
        "rejected_candidates",
        "common_requirements",
        "candidate_disagreements",
        "speculative_expansions",
        "missing_but_necessary_details",
        "clarifications_made",
        "files",
        "proper_nouns",
        "facts",
    ],
}

PM_EXPANSION_CLEANUP_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "lines": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "identical to an item you were provided, but without references to candidate numbers",
            },
        },
    },
    "required": ["lines"],
}

NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "lines": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "exploratory steps ONLY, preserved as-is",
            },
        },
    },
    "required": ["lines"],
}

PM_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {"type": "boolean", "description": "False"},
        "reset_reason": {"type": "string", "description": ""},
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "product"},
                    "message": {"type": "string", "description": "issue description"},
                    "next_actions": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": "specific actionable fix",
                        },
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
        },
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
    ],
}

SYSTEM_DECOMPOSITION_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "decomposition": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "high-level explanation of how the request was broken down into implementation domains",
                },
                "reviewer_notes": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "notes about decomposition decisions, coupling concerns, assumptions, and why certain areas were grouped or separated",
                    },
                },
                "domains": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "additionalProperties": False,
                        "properties": {
                            "id": {
                                "type": "integer",
                                "description": "numeric domain identifier",
                            },
                            "name": {
                                "type": "string",
                                "description": "short domain name",
                            },
                            "scope": {
                                "type": "string",
                                "description": "clear description of what belongs in this domain",
                            },
                            "includes": {
                                "type": "array",
                                "items": {
                                    "type": "string",
                                    "description": "specific responsibility, specific component, specific subsystem",
                                },
                            },
                            "excludes": {
                                "type": "array",
                                "items": {
                                    "type": "string",
                                    "description": "related responsibility intentionally handled elsewhere",
                                },
                            },
                            "dependencies": {
                                "type": "array",
                                "items": {
                                    "type": "string",
                                    "description": "name of prerequisite domain",
                                },
                            },
                            "reasoning": {
                                "type": "string",
                                "description": "why this domain is separated, why it is cohesive, and why it should be implemented at this stage",
                            },
                            "architect_input": {
                                "type": "string",
                                "description": "fully scoped architecture request for this domain, written so it can be passed directly to a software architect without additional processing; in case this domain has been confirmed fully complete - set this field to empty string.",
                            },
                        },
                        "required": [
                            "id",
                            "name",
                            "scope",
                            "includes",
                            "excludes",
                            "dependencies",
                            "reasoning",
                            "architect_input",
                        ],
                    },
                    "description": "list of decomposition domains",
                },
                "integration_order": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "ordered list of domain names representing recommended execution sequence",
                    },
                },
                "global_risks": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "cross-domain risk, coupling issue, sequencing concern, or area requiring careful validation",
                    },
                },
            },
            "required": [
                "summary",
                "reviewer_notes",
                "domains",
                "integration_order",
                "global_risks",
            ],
        },
        "next_steps": next_steps,
    },
    "required": ["decomposition", "next_steps"],
}

SYSTEM_DECOMPOSITION_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "severity": {
                        "type": "string",
                        "enum": ["low", "medium", "high"],
                    },
                    "severity_reason": {
                        "type": "string",
                        "description": "short explanation of why current severity level was chosen",
                    },
                    "category": {"type": "string", "description": "decomposition"},
                    "message": {
                        "type": "string",
                        "description": "description of the issue",
                    },
                    "next_actions": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": "specific actionable fix",
                        },
                    },
                },
                "required": [
                    "severity",
                    "severity_reason",
                    "category",
                    "message",
                    "next_actions",
                ],
            },
            "description": "list of issues found",
        },
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
    ],
}

DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "text": {
            "type": "string",
            "description": "only the transformed text, with no explanations or commentary",
        },
    },
    "required": ["text"],
}

INVESTIGATION_CLASSIFIER_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "type": {
            "type": "string",
            "enum": ["investigation", "engineering"],
            "description": "classification type: 'investigation' for exploratory/research tasks, 'engineering' for implementation tasks",
        },
        "reasoning": {
            "type": "string",
            "description": "brief explanation of why this task was classified as investigation or engineering",
        },
    },
    "required": ["type", "reasoning"],
}

INVESTIGATOR_PLAN_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "workstreams": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "objective": {
                        "type": "string",
                        "description": "clear statement of what this workstream aims to determine",
                    },
                    "data_sources": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": "specific logs, metrics, files, or systems to examine",
                        },
                    },
                    "hypotheses": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": "testable hypotheses to investigate",
                        },
                    },
                    "investigation_methods": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": "specific methods or techniques for gathering evidence",
                        },
                    },
                    "expected_deliverables": {
                        "type": "array",
                        "items": {
                            "type": "string",
                            "description": "what this workstream should produce",
                        },
                    },
                },
                "required": [
                    "objective",
                    "data_sources",
                    "hypotheses",
                    "investigation_methods",
                    "expected_deliverables",
                ],
            },
        },
    },
    "required": ["workstreams"],
}

INVESTIGATOR_FINDINGS_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "workstream_objective": {
            "type": "string",
            "description": "which workstream objective this finding addresses",
        },
        "conclusions": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "findings derived from evidence",
            },
        },
        "supporting_evidence": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "evidence_type": {
                        "type": "string",
                        "description": "category of evidence (e.g., log_entry, metric_value, code_snippet, user_report)",
                    },
                    "evidence_description": {
                        "type": "string",
                        "description": "detailed description of the evidence",
                    },
                    "source_reference": {
                        "type": "string",
                        "description": "reference to the source where this evidence was found",
                    },
                },
                "required": [
                    "evidence_type",
                    "evidence_description",
                    "source_reference",
                ],
            },
            "description": "specific evidence backing each conclusion",
        },
        "confidence_level": {
            "type": "string",
            "enum": ["low", "medium", "high"],
            "description": "confidence in the conclusions based on evidence quality and quantity",
        },
        "unanswered_questions": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "questions that remain after investigation",
            },
        },
    },
    "required": [
        "workstream_objective",
        "conclusions",
        "supporting_evidence",
        "confidence_level",
        "unanswered_questions",
    ],
}

INVESTIGATION_REPORT_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "executive_summary": {
            "type": "string",
            "description": "high-level summary of investigation findings for non-technical stakeholders",
        },
        "root_cause_analysis": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "primary_cause": {
                    "type": "string",
                    "description": "the single most important underlying cause",
                },
                "contributing_factors": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "secondary factors that contributed to the issue",
                    },
                },
                "evidence_trail": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "description": "step in the evidence chain",
                    },
                    "description": "logical chain of evidence connecting root cause to symptoms",
                },
            },
            "required": ["primary_cause", "contributing_factors", "evidence_trail"],
        },
        "timeline_reconstruction": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "timestamp": {
                        "type": "string",
                        "description": "when the event occurred",
                    },
                    "event": {
                        "type": "string",
                        "description": "what happened at that time",
                    },
                },
                "required": ["timestamp", "event"],
            },
            "description": "chronological account of events leading to and surrounding the issue",
        },
        "customer_impact_assessment": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "affected_users": {
                    "type": "string",
                    "description": "estimate of how many users/customers were impacted",
                },
                "severity": {
                    "type": "string",
                    "description": "overall severity of customer impact",
                },
                "duration": {
                    "type": "string",
                    "description": "how long the impact lasted",
                },
            },
            "required": ["affected_users", "severity", "duration"],
        },
        "correlation_findings": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "observation": {
                        "type": "string",
                        "description": "what correlation was observed",
                    },
                    "correlation_strength": {
                        "type": "string",
                        "enum": ["weak", "moderate", "strong"],
                        "description": "how confident we are in this correlation",
                    },
                    "causal_claim": {
                        "type": "string",
                        "description": "whether this correlation implies causation and to what extent",
                    },
                },
                "required": ["observation", "correlation_strength", "causal_claim"],
            },
            "description": "correlations discovered between variables or events",
        },
        "hypothesis_test_results": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "hypothesis": {
                        "type": "string",
                        "description": "the hypothesis that was tested",
                    },
                    "test_performed": {
                        "type": "string",
                        "description": "description of the test method used",
                    },
                    "result": {
                        "type": "string",
                        "enum": ["confirmed", "refuted", "inconclusive"],
                        "description": "outcome of the hypothesis test",
                    },
                    "conclusion": {
                        "type": "string",
                        "description": "interpretation of the result",
                    },
                },
                "required": ["hypothesis", "test_performed", "result", "conclusion"],
            },
            "description": "results of hypothesis testing during the investigation",
        },
        "known_gaps_and_unknowns": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "areas where evidence is incomplete or findings are uncertain",
            },
            "description": "list of known gaps and unknowns",
        },
        "recommendations": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "priority": {
                        "type": "string",
                        "enum": ["low", "medium", "high", "critical"],
                        "description": "urgency of the recommendation",
                    },
                    "action": {
                        "type": "string",
                        "description": "what should be done",
                    },
                    "rationale": {
                        "type": "string",
                        "description": "why this recommendation follows from the findings",
                    },
                },
                "required": ["priority", "action", "rationale"],
            },
            "description": "actionable recommendations based on findings",
        },
    },
    "required": [
        "executive_summary",
        "root_cause_analysis",
        "timeline_reconstruction",
        "customer_impact_assessment",
        "correlation_findings",
        "hypothesis_test_results",
        "known_gaps_and_unknowns",
        "recommendations",
    ],
}

ISSUE_ITEM = {
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "severity": {
            "type": "string",
            "enum": ["low", "medium", "high"],
        },
        "severity_reason": {
            "type": "string",
            "description": "short explanation of why current severity level was chosen",
        },
        "category": {
            "type": "string",
            "description": "category of the issue (e.g., implementation, design, testing)",
        },
        "message": {"type": "string", "description": "issue description"},
        "next_actions": {
            "type": "array",
            "items": {
                "type": "string",
                "description": "specific actionable fix",
            },
        },
    },
    "required": [
        "severity",
        "severity_reason",
        "category",
        "message",
        "next_actions",
    ],
}

BASE_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        **approved,
        "should_reset": {
            "type": "boolean",
        },
        "reset_reason": {
            "type": "string",
            "description": "short explanation of why prior context is no longer trustworthy",
        },
        "issues": {
            "type": "array",
            "items": ISSUE_ITEM,
            "description": "list of issues found",
        },
        "next_steps": next_steps,
    },
    "required": [
        "approved",
        "resolved_issues",
        "approved_confidence",
        "approved_reason",
        "should_reset",
        "reset_reason",
        "issues",
        "next_steps",
    ],
}

GAP_ANALYSIS_REVIEW_SCHEMA = dict(BASE_REVIEW_SCHEMA)

FACT_CHECKING_REVIEW_SCHEMA = dict(BASE_REVIEW_SCHEMA)

STRUCTURAL_REVIEW_SCHEMA = dict(BASE_REVIEW_SCHEMA)
