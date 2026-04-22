next_steps = {
    "type": "array",
    "items": {
        "type": "string",
        "description": """
Ordered list of 1–5 concrete reasoning or analysis steps that are **required to complete this task within the agent’s role**, but have not yet been performed.

These steps represent **missing internal work**, not future execution or delegation.

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
    },
    "required": ["plan"],
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
        "approved": {
            "type": "boolean",
        },
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
    "required": ["approved", "should_reset", "reset_reason", "issues", "next_steps"],
}

PLAN_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "approved": {
            "type": "boolean",
        },
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
    "required": ["approved", "should_reset", "reset_reason", "issues", "next_steps"],
}

CODE_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "approved": {
            "type": "boolean",
        },
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
    "required": ["approved", "should_reset", "reset_reason", "issues", "next_steps"],
}

TECH_LEAD_FINAL_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "approved": {
            "type": "boolean",
        },
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
    },
    "required": ["approved", "should_reset", "reset_reason", "issues"],
}

ARCH_FINAL_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "approved": {
            "type": "boolean",
        },
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
    "required": ["approved", "should_reset", "reset_reason", "issues"],
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
        "next_steps": next_steps,
    },
    "required": [
        "task_specification",
        "original_input_preserved",
        "clarifications_made",
        "files",
        "proper_nouns",
        "facts",
        "next_steps",
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

PM_REVIEW_SCHEMA = {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "approved": {"type": "boolean", "description": "true"},
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
    "required": ["approved", "should_reset", "reset_reason", "issues"],
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
        "approved": {
            "type": "boolean",
        },
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
    "required": ["approved", "should_reset", "reset_reason", "issues"],
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
