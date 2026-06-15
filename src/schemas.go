// =========================
// SCHEMAS
// =========================
package main

var nextSteps = map[string]interface{}{
	"type": "array",
	"items": map[string]interface{}{
		"type": "string",
		"description": `
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
`,
	},
}

var approvedSchema = map[string]interface{}{
	"approved": map[string]interface{}{
		"type": "boolean",
		"description": "true/false; " + MarshalJSON(`Approval is only valid if:
* All prior high-severity issues are either resolved or downgraded with explicit justification
* At least one paragraph explains why the design is now considered sound

CRITICAL: If the previous review contained high-severity issues, a transition to zero issues must include explicit justification for each.`),
	},
	"approved_confidence": map[string]interface{}{
		"type": "string",
		"enum": []string{"not_approved", "low", "medium", "high"},
	},
	"approved_reason": map[string]interface{}{
		"type":        "string",
		"description": "if approved=true - explain in detail why exactly has the approval been given, provide evidence; of approved=false - set this field to empty string",
	},
	"resolved_issues": map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type":        "string",
			"description": "Issues cannot be removed without explanation.\nIf an issue is no longer present - move it here, with justification.\n\nIf no issues were resolved - leave this array empty.",
		},
	},
}

var ARCH_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"architecture": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"overview": map[string]interface{}{
					"type":        "string",
					"description": "high-level design aligned with existing system",
				},
				"reviewer_notes": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "notes about alignment with current system and key tradeoffs; must include: 'Why not reuse existing system?' explanation for any new component",
					},
				},
				"components": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type":        "string",
								"description": "component name (existing or new)",
							},
							"responsibility": map[string]interface{}{
								"type":        "string",
								"description": "what it does",
							},
							"background": map[string]interface{}{
								"type":        "string",
								"description": "why this fits into the existing system; if the component is new: (1) why existing components cannot be reused, and (2) why introducing a new component is the simplest correct solution",
							},
						},
						"required": []string{"name", "responsibility", "background"},
					},
				},
				"data_flow": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "how data moves through EXISTING and new components",
					},
				},
				"tech_choices": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "choices that must align with current stack",
					},
				},
				"constraints": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "assumptions, limitations, and known gaps in system understanding",
					},
				},
			},
			"required": []string{"overview", "reviewer_notes", "components", "data_flow", "tech_choices", "constraints"},
		},
		"next_steps": nextSteps,
	},
	"required": []string{"architecture", "next_steps"},
}

var PLAN_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"plan": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "what will be implemented and how it integrates into the existing system",
				},
				"reviewer_notes": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "notes about assumptions on current codebase and structure",
					},
				},
				"files": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "relative/file.ext",
							},
							"purpose": map[string]interface{}{
								"type":        "string",
								"description": "what this file or modification does",
							},
							"background": map[string]interface{}{
								"type":        "string",
								"description": "why this belongs in this location in the existing system",
							},
						},
						"required": []string{"path", "purpose", "background"},
					},
				},
				"steps": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "integer",
								"description": "numeric step identifier",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "specific step tied to real file changes",
							},
						},
						"required": []string{"id", "description"},
					},
				},
			},
			"required": []string{"summary", "reviewer_notes", "files", "steps"},
		},
		"next_steps": nextSteps,
	},
	"required": []string{"plan", "next_steps"},
}

var CODER_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"changes": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "relative/file.ext",
					},
					"status": map[string]interface{}{
						"type": "string",
						"enum": []string{"modified", "created", "unchanged_blocked"},
					},
					"blocked_reason": map[string]interface{}{
						"type":        "string",
						"description": "required only when status is unchanged_blocked; set to empty string otherwise",
					},
					"brief_summary": map[string]interface{}{
						"type":        "string",
						"description": "what was actually changed and how it integrates with existing code",
					},
					"exists_after_change": map[string]interface{}{
						"type":        "boolean",
						"description": "true for created and modified files, false otherwise",
					},
					"diff_summary": map[string]interface{}{
						"type":        "string",
						"description": "describe the actual code change",
					},
				},
				"required": []string{"path", "status", "blocked_reason", "brief_summary", "exists_after_change", "diff_summary"},
			},
		},
		"summary": map[string]interface{}{
			"type":        "string",
			"description": "summary of all completed changes and how they fit into the existing system",
		},
		"reviewer_notes": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "notes about assumptions, blocked work, incomplete areas, missing context, or areas needing attention",
			},
		},
		"next_steps": nextSteps,
	},
	"required": []string{"changes", "summary", "reviewer_notes", "next_steps"},
}

var ISSUE_ITEM = map[string]interface{}{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"severity": map[string]interface{}{
			"type": "string",
			"enum": []string{"low", "medium", "high"},
		},
		"severity_reason": map[string]interface{}{
			"type":        "string",
			"description": "short explanation of why current severity level was chosen",
		},
		"category": map[string]interface{}{
			"type":        "string",
			"description": "category of the issue (e.g., implementation, design, testing)",
		},
		"message": map[string]interface{}{
			"type":        "string",
			"description": "issue description",
		},
		"next_actions": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "specific actionable fix",
			},
		},
	},
	"required": []string{"severity", "severity_reason", "category", "message", "next_actions"},
}

func makeReviewSchema(category string) map[string]interface{} {
	return map[string]interface{}{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"type":                 "object",
		"additionalProperties": false,
		"properties": mergeMaps(approvedSchema, map[string]interface{}{
			"should_reset": map[string]interface{}{
				"type": "boolean",
			},
			"reset_reason": map[string]interface{}{
				"type":        "string",
				"description": "short explanation of why prior context is no longer trustworthy",
			},
			"issues": map[string]interface{}{
				"type":        "array",
				"items":       ISSUE_ITEM,
				"description": "list of issues found",
			},
		}),
		"required": []string{
			"approved", "approved_confidence", "approved_reason", "resolved_issues",
			"should_reset", "reset_reason", "issues",
		},
	}
}

func makeReviewSchemaWithNextSteps(category, reviewType string) map[string]interface{} {
	nextActionsDescription := ""
	switch reviewType {
	case "design": // agent/schemas.py:296-322
		nextActionsDescription = "\nOrdered list of 1–5 concrete, actionable instructions that the system design reviewer is issuing to the software architect agent.\n\nThese actions represent **external design work** that should be performed next to improve or complete the system architecture.\n\nRules:\n* Each action must be something the software architect can directly update or define in the system design\n* Use clear, imperative language (e.g., “Define…”, “Refine…”, “Add…”, “Clarify…”, “Restructure…”)\n* Focus on specific design elements (components, interfaces, data flows, constraints), not broad architectural goals\n* Do not include reasoning, justification, or analysis\n* Do not restate problems — only prescribe actions\n* Do not include internal thinking or validation steps (those belong in `next_steps`)\n* Avoid vague instructions (e.g., “improve scalability”, “make it better”)\n* Do not delegate beyond the software architect or introduce new roles\n* Each action should map to a concrete change or addition in the architecture\n\nIf no further design changes are required, return an empty array.\n"
	case "plan": // agent/schemas.py:373-400
		nextActionsDescription = "\nOrdered list of 1–5 concrete, actionable instructions that the implementation plan reviewer is issuing to the tech lead agent.\n\nThese actions represent **external planning work** that should be performed next to improve or complete the implementation plan.\n\nRules:\n* Each action must be something the tech lead can directly update in the implementation plan\n* Use clear, imperative language (e.g., “Break down…”, “Add…”, “Sequence…”, “Specify…”, “Adjust…”)\n* Focus on specific plan elements (task breakdowns, dependencies, sequencing, ownership, risks), not broad objectives\n* Do not include reasoning, justification, or analysis\n* Do not restate problems — only prescribe actions\n* Do not include internal thinking or validation steps (those belong in `next_steps`)\n* Avoid vague instructions (e.g., “improve plan”, “make it clearer”)\n* Do not delegate beyond the tech lead or introduce new roles\n* Each action should map to a concrete modification of the implementation plan\n\nIf no further planning changes are required, return an empty array.\n"
	case "code": // agent/schemas.py:450-477
		nextActionsDescription = "\nOrdered list of 1–5 concrete, actionable instructions that the code review agent is issuing to the coder agent.\n\nThese actions represent **external implementation work** that should be performed next to improve or complete the solution.\n\nRules:\n* Each action must be something the coder agent can directly implement or modify in code\n* Use clear, imperative language (e.g., “Refactor…”, “Add…”, “Fix…”, “Remove…”)\n* Focus on specific, localized changes rather than broad goals\n* Do not include reasoning, justification, or analysis\n* Do not restate problems — only prescribe actions\n* Do not include internal thinking or validation steps (those belong in `next_steps`)\n* Avoid vague instructions (e.g., “improve this”, “optimize code”)\n* Do not delegate beyond the coder agent or introduce new roles\n* Each action should map to a concrete change in the codebase\n\nIf no further code changes are required, return an empty array.\n"
	}
	props := mergeMaps(approvedSchema, map[string]interface{}{
		"should_reset": map[string]interface{}{
			"type": "boolean",
		},
		"reset_reason": map[string]interface{}{
			"type":        "string",
			"description": "short explanation of why prior context is no longer trustworthy",
		},
		"issues": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"severity": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"severity_reason": map[string]interface{}{
						"type":        "string",
						"description": "short explanation of why current severity level was chosen",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": category,
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "issue description",
					},
					"next_actions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": nextActionsDescription,
						},
					},
				},
				"required": []string{"severity", "severity_reason", "category", "message", "next_actions"},
			},
		},
		"next_steps": nextSteps,
	})
	return map[string]interface{}{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"type":                 "object",
		"additionalProperties": false,
		"properties":           props,
		"required": []string{
			"approved", "approved_confidence", "approved_reason", "resolved_issues",
			"should_reset", "reset_reason", "issues", "next_steps",
		},
	}
}

var ARCH_REVIEW_SCHEMA = makeReviewSchemaWithNextSteps("design", "design")
var PLAN_REVIEW_SCHEMA = makeReviewSchemaWithNextSteps("implementation", "plan")
var CODE_REVIEW_SCHEMA = makeReviewSchemaWithNextSteps("implementation", "code")

var TECH_LEAD_FINAL_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": mergeMaps(approvedSchema, map[string]interface{}{
		"should_reset": map[string]interface{}{
			"type": "boolean",
		},
		"reset_reason": map[string]interface{}{
			"type":        "string",
			"description": "short explanation of why prior context is no longer trustworthy",
		},
		"issues": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"severity": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"severity_reason": map[string]interface{}{
						"type":        "string",
						"description": "short explanation of why current severity level was chosen",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "implementation",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "issue description",
					},
					"next_actions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "actionable fixes",
						},
					},
				},
				"required": []string{"severity", "severity_reason", "category", "message", "next_actions"},
			},
		},
		"next_steps": nextSteps,
	}),
	"required": []string{
		"approved", "approved_confidence", "approved_reason", "resolved_issues",
		"should_reset", "reset_reason", "issues", "next_steps",
	},
}

var ARCH_FINAL_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": mergeMaps(approvedSchema, map[string]interface{}{
		"should_reset": map[string]interface{}{
			"type": "boolean",
		},
		"reset_reason": map[string]interface{}{
			"type":        "string",
			"description": "short explanation of why prior context is no longer trustworthy",
		},
		"issues": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"severity": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"severity_reason": map[string]interface{}{
						"type":        "string",
						"description": "short explanation of why current severity level was chosen",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "design",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "issue description",
					},
					"next_actions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "actionable fixes",
						},
					},
				},
				"required": []string{"severity", "severity_reason", "category", "message", "next_actions"},
			},
		},
	}),
	"required": []string{
		"approved", "approved_confidence", "approved_reason", "resolved_issues",
		"should_reset", "reset_reason", "issues",
	},
}

var PRODUCT_MANAGER_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"task_specification": map[string]interface{}{
			"type":        "string",
			"description": "fully refined and engineering-ready task description",
		},
		"original_input_preserved": map[string]interface{}{
			"type":        "boolean",
			"description": "true",
		},
		"clarifications_made": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "explicit assumptions, refinements, missing requirements filled in, and interpretation decisions",
			},
		},
		"files": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "if user request refers to any files - extract file names/pathes, and put them here as is",
			},
		},
		"proper_nouns": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "if user request refers to any proper nouns which are *not* file names/pathes - extract them, and put them here as is",
			},
		},
		"facts": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "if user request states specific facts - extract them, and put them here",
			},
		},
	},
	"required": []string{
		"task_specification", "original_input_preserved", "clarifications_made",
		"files", "proper_nouns", "facts",
	},
}

var PM_SYNTHESIZER_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"task_specification": map[string]interface{}{
			"type":        "string",
			"description": "final refined engineering-ready task description",
		},
		"selected_candidate": map[string]interface{}{
			"type":        "integer",
			"description": "1",
		},
		"selection_reason": map[string]interface{}{
			"type":        "string",
			"description": "why this candidate was closest to the correct interpretation",
		},
		"rejected_candidates": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"candidate": map[string]interface{}{
						"type":        "integer",
						"description": "candidate number",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "why this interpretation was rejected",
					},
				},
				"required": []string{"candidate", "reason"},
			},
		},
		"common_requirements": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "requirements most candidates agree on",
			},
		},
		"candidate_disagreements": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "common points of disagreement between most candidates",
			},
		},
		"speculative_expansions": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "what candidates clearly speculate on",
			},
		},
		"missing_but_necessary_details": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "details not commonly found in candidate interpretations that are clearly required",
			},
		},
		"clarifications_made": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "final assumptions and interpretation decisions",
			},
		},
		"files": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "if user request refers to any files - extract file names/pathes, and put them here as is",
			},
		},
		"proper_nouns": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "if user request refers to any proper nouns which are *not* file names/pathes - extract them, and put them here as is",
			},
		},
		"facts": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "if user request states specific facts - extract them, and put them here",
			},
		},
	},
	"required": []string{
		"task_specification", "selected_candidate", "selection_reason",
		"rejected_candidates", "common_requirements", "candidate_disagreements",
		"speculative_expansions", "missing_but_necessary_details",
		"clarifications_made", "files", "proper_nouns", "facts",
	},
}

var PM_EXPANSION_CLEANUP_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"lines": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "identical to an item you were provided, but without references to candidate numbers",
			},
		},
	},
	"required": []string{"lines"},
}

var NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"lines": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "exploratory steps ONLY, preserved as-is",
			},
		},
	},
	"required": []string{"lines"},
}

var PM_REVIEW_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": mergeMaps(approvedSchema, map[string]interface{}{
		"should_reset": map[string]interface{}{
			"type":        "boolean",
			"description": "False",
		},
		"reset_reason": map[string]interface{}{
			"type":        "string",
			"description": "",
		},
		"issues": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"severity": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"severity_reason": map[string]interface{}{
						"type":        "string",
						"description": "short explanation of why current severity level was chosen",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "product",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "issue description",
					},
					"next_actions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "specific actionable fix",
						},
					},
				},
				"required": []string{"severity", "severity_reason", "category", "message", "next_actions"},
			},
		},
	}),
	"required": []string{
		"approved", "approved_confidence", "approved_reason", "resolved_issues",
		"should_reset", "reset_reason", "issues",
	},
}

var SYSTEM_DECOMPOSITION_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"decomposition": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "high-level explanation of the architecture decomposition strategy",
				},
				"reviewer_notes": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"domains": map[string]interface{}{
					"type":        "array",
					"description": "architecture domains forming a DAG",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "unique-identifier-of-this-domain",
							},
							"name": map[string]interface{}{
								"type": "string",
							},
							"category": map[string]interface{}{
								"type": "string",
								"enum": []string{
									"Foundation",
									"Core Business Logic",
									"Data & Persistence",
									"External Integrations",
									"User Experience",
									"Platform Infrastructure",
									"Cross-Domain Integration",
									"Validation & Acceptance",
								},
							},
							"responsibility": map[string]interface{}{
								"type":        "string",
								"description": "single ownership responsibility of this domain",
							},
							"scope": map[string]interface{}{
								"type":        "string",
								"description": "what architecture problem this domain owns",
							},
							"constraints": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
							"upstream_dependencies": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
								"description": "unique-identifier-of-a-dependency-domain; ids of domains that produce required artifacts",
							},
							"consumed_artifacts": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]interface{}{
										"artifact_name": map[string]interface{}{
											"type": "string",
										},
										"producer_domain_id": map[string]interface{}{
											"type":        "string",
											"description": "unique-identifier-of-a-producer-domain; id of a domain that is expected to produce this artifact",
										},
										"purpose": map[string]interface{}{
											"type":        "string",
											"description": "why this artifact is required",
										},
									},
									"required": []string{"artifact_name", "producer_domain_id", "purpose"},
								},
							},
							"produced_artifacts": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]interface{}{
										"artifact_name": map[string]interface{}{
											"type": "string",
										},
										"artifact_type": map[string]interface{}{
											"type": "string",
										},
										"purpose": map[string]interface{}{
											"type": "string",
										},
										"expected_content": map[string]interface{}{
											"type": "string",
										},
									},
									"required": []string{"artifact_name", "artifact_type", "purpose", "expected_content"},
								},
							},
							"expected_architecture_outcomes": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
							"domain_specification": map[string]interface{}{
								"type":        "string",
								"description": "bounded architecture problem statement describing responsibility, scope, constraints, available upstream artifacts, and expected outputs. Must NOT contain architecture designs, APIs, schemas, algorithms, implementation steps, or technical mechanisms. If already completed, set to empty string.",
							},
						},
						"required": []string{
							"id", "name", "category", "responsibility", "scope",
							"constraints", "upstream_dependencies", "consumed_artifacts",
							"produced_artifacts", "expected_architecture_outcomes",
							"domain_specification",
						},
					},
				},
				"integration_ownership": map[string]interface{}{
					"type":        "array",
					"description": "explicit ownership of cross-domain integration responsibilities",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"capability": map[string]interface{}{
								"type": "string",
							},
							"owner_domain_id": map[string]interface{}{
								"type":        "string",
								"description": "unique-identifier-of-an-owner-domain; id of a domain that owns this integration",
							},
							"integration_artifacts": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
						},
						"required": []string{"capability", "owner_domain_id", "integration_artifacts"},
					},
				},
				"global_risks": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{
				"summary", "reviewer_notes", "domains",
				"integration_ownership", "global_risks",
			},
		},
		"next_steps": nextSteps,
	},
	"required": []string{"decomposition", "next_steps"},
}

var SYSTEM_DECOMPOSITION_REVIEW_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": mergeMaps(approvedSchema, map[string]interface{}{
		"should_reset": map[string]interface{}{
			"type": "boolean",
		},
		"reset_reason": map[string]interface{}{
			"type":        "string",
			"description": "short explanation of why prior context is no longer trustworthy",
		},
		"issues": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"severity": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"severity_reason": map[string]interface{}{
						"type":        "string",
						"description": "short explanation of why current severity level was chosen",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "decomposition",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "description of the issue",
					},
					"next_actions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "specific actionable fix",
						},
					},
				},
				"required": []string{"severity", "severity_reason", "category", "message", "next_actions"},
			},
			"description": "list of issues found",
		},
	}),
	"required": []string{
		"approved", "approved_confidence", "approved_reason", "resolved_issues",
		"should_reset", "reset_reason", "issues",
	},
}

var DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"text": map[string]interface{}{
			"type":        "string",
			"description": "only the transformed text, with no explanations or commentary",
		},
	},
	"required": []string{"text"},
}

var INVESTIGATION_CLASSIFIER_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"type": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"investigation", "engineering"},
			"description": "classification type: 'investigation' for exploratory/research tasks, 'engineering' for implementation tasks",
		},
		"reasoning": map[string]interface{}{
			"type":        "string",
			"description": "brief explanation of why this task was classified as investigation or engineering",
		},
	},
	"required": []string{"type", "reasoning"},
}

var INVESTIGATOR_PLAN_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"workstreams": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "unique-identifier-of-this-workstream",
					},
					"dependencies": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "unique-identifier-of-a-dependency-workstream; if the workstream has no dependencies - leave this array blank",
						},
					},
					"objective": map[string]interface{}{
						"type":        "string",
						"description": "clear statement of what this workstream aims to determine",
					},
					"data_sources": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "specific logs, metrics, files, or systems to examine",
						},
					},
					"hypotheses": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "testable hypotheses to investigate",
						},
					},
					"investigation_methods": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "specific methods or techniques for gathering evidence",
						},
					},
					"expected_deliverables": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":        "string",
							"description": "what this workstream should produce",
						},
					},
				},
				"required": []string{
					"id", "dependencies", "objective",
					"data_sources", "hypotheses", "investigation_methods",
					"expected_deliverables",
				},
			},
		},
	},
	"required": []string{"workstreams"},
}

var INVESTIGATOR_FINDINGS_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"workstream_objective": map[string]interface{}{
			"type":        "string",
			"description": "which workstream objective this finding addresses",
		},
		"conclusions": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "findings derived from evidence",
			},
		},
		"supporting_evidence": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"evidence_type": map[string]interface{}{
						"type":        "string",
						"description": "category of evidence (e.g., log_entry, metric_value, code_snippet, user_report)",
					},
					"evidence_description": map[string]interface{}{
						"type":        "string",
						"description": "detailed description of the evidence",
					},
					"source_reference": map[string]interface{}{
						"type":        "string",
						"description": "reference to the source where this evidence was found",
					},
				},
				"required": []string{"evidence_type", "evidence_description", "source_reference"},
			},
			"description": "specific evidence backing each conclusion",
		},
		"confidence_level": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"low", "medium", "high"},
			"description": "confidence in the conclusions based on evidence quality and quantity",
		},
		"unanswered_questions": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "questions that remain after investigation",
			},
		},
	},
	"required": []string{
		"workstream_objective", "conclusions",
		"supporting_evidence", "confidence_level", "unanswered_questions",
	},
}

var INVESTIGATION_REPORT_SCHEMA = map[string]interface{}{
	"$schema":              "http://json-schema.org/draft-07/schema#",
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]interface{}{
		"executive_summary": map[string]interface{}{
			"type":        "string",
			"description": "high-level summary of investigation findings for non-technical stakeholders",
		},
		"root_cause_analysis": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"primary_cause": map[string]interface{}{
					"type":        "string",
					"description": "the single most important underlying cause",
				},
				"contributing_factors": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "secondary factors that contributed to the issue",
					},
				},
				"evidence_trail": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "step in the evidence chain",
					},
					"description": "logical chain of evidence connecting root cause to symptoms",
				},
			},
			"required": []string{"primary_cause", "contributing_factors", "evidence_trail"},
		},
		"timeline_reconstruction": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{
						"type":        "string",
						"description": "when the event occurred",
					},
					"event": map[string]interface{}{
						"type":        "string",
						"description": "what happened at that time",
					},
				},
				"required": []string{"timestamp", "event"},
			},
			"description": "chronological account of events leading to and surrounding the issue",
		},
		"customer_impact_assessment": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"affected_users": map[string]interface{}{
					"type":        "string",
					"description": "estimate of how many users/customers were impacted",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "overall severity of customer impact",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "how long the impact lasted",
				},
			},
			"required": []string{"affected_users", "severity", "duration"},
		},
		"correlation_findings": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"observation": map[string]interface{}{
						"type":        "string",
						"description": "what correlation was observed",
					},
					"correlation_strength": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"weak", "moderate", "strong"},
						"description": "how confident we are in this correlation",
					},
					"causal_claim": map[string]interface{}{
						"type":        "string",
						"description": "whether this correlation implies causation and to what extent",
					},
				},
				"required": []string{"observation", "correlation_strength", "causal_claim"},
			},
			"description": "correlations discovered between variables or events",
		},
		"hypothesis_test_results": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"hypothesis": map[string]interface{}{
						"type":        "string",
						"description": "the hypothesis that was tested",
					},
					"test_performed": map[string]interface{}{
						"type":        "string",
						"description": "description of the test method used",
					},
					"result": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"confirmed", "refuted", "inconclusive"},
						"description": "outcome of the hypothesis test",
					},
					"conclusion": map[string]interface{}{
						"type":        "string",
						"description": "interpretation of the result",
					},
				},
				"required": []string{"hypothesis", "test_performed", "result", "conclusion"},
			},
			"description": "results of hypothesis testing during the investigation",
		},
		"known_gaps_and_unknowns": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":        "string",
				"description": "areas where evidence is incomplete or findings are uncertain",
			},
			"description": "list of known gaps and unknowns",
		},
		"recommendations": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "urgency of the recommendation",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "what should be done",
					},
					"rationale": map[string]interface{}{
						"type":        "string",
						"description": "why this recommendation follows from the findings",
					},
				},
				"required": []string{"priority", "action", "rationale"},
			},
			"description": "actionable recommendations based on findings",
		},
	},
	"required": []string{
		"executive_summary", "root_cause_analysis", "timeline_reconstruction",
		"customer_impact_assessment", "correlation_findings",
		"hypothesis_test_results", "known_gaps_and_unknowns", "recommendations",
	},
}

var GAP_ANALYSIS_REVIEW_SCHEMA = makeReviewSchema("gap analysis")
var FACT_CHECKING_REVIEW_SCHEMA = makeReviewSchema("fact checking")
var STRUCTURAL_REVIEW_SCHEMA = makeReviewSchema("structural")
var INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA = makeReviewSchema("plan quality")
var SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA = makeReviewSchema("synthesis consistency")
