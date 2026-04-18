ARCH_SCHEMA = """
{
  "architecture": {
    "overview": "high-level design aligned with existing system",
    "reviewer_notes": [
        "notes about alignment with current system and key tradeoffs",
        "must include: 'Why not reuse existing system?' explanation for any new component"
    ],
    "components": [
      {
        "name": "component name (existing or new)",
        "responsibility": "what it does",
        "background": "why this fits into the existing system; if the component is new: (1) why existing components cannot be reused, and (2) why introducing a new component is the simplest correct solution"
      }
    ],
    "data_flow": [
      "how data moves through EXISTING and new components"
    ],
    "tech_choices": [
      "choices that must align with current stack"
    ],
    "constraints": [
      "assumptions, limitations, and known gaps in system understanding"
    ]
  }
}
"""

PLAN_SCHEMA = """
{
  "plan": {
    "summary": "what will be implemented and how it integrates into the existing system",
    "reviewer_notes": [
        "notes about assumptions on current codebase and structure"
    ],
    "files": [
      {
        "path": "relative/file.ext",
        "purpose": "what this file or modification does",
        "background": "why this belongs in this location in the existing system"
      }
    ],
    "steps": [
      {
        "id": 1,
        "description": "specific step tied to real file changes"
      }
    ]
  }
}
"""

CODER_SCHEMA = """
{
"changes": [
{
"path": "relative/file.ext",
"status": "modified/created/unchanged_blocked",
"brief_summary": "what was actually changed and how it integrates with existing code"
}
],
"summary": "summary of all completed changes and how they fit into the existing system",
"reviewer_notes": [
"notes about assumptions, blocked work, incomplete areas, missing context, or areas needing attention"
]
}
"""

ARCH_REVIEW_SCHEMA = """
{
"approved": true/false,
"should_reset": true/false,
"reset_reason": "short explanation of why prior context is no longer trustworthy",
"issues": [
{
"severity": "low/high",
"category": "design",
"message": "issue description",
"next_actions": ["actionable fixes"]
}
]
}
"""

PLAN_REVIEW_SCHEMA = """
{
  "approved": true/false,
  "should_reset": true/false,
  "reset_reason": "short explanation of why prior context is no longer trustworthy",
  "issues": [
    {
      "severity": "low/high",
      "category": "implementation",
      "message": "issue description",
      "next_actions": ["actionable fixes"]
    }
  ]
}
"""

CODE_REVIEW_SCHEMA = """
{
"approved": true/false,
"should_reset": true/false,
"reset_reason": "short explanation of why prior context is no longer trustworthy",
"issues": [
{
"severity": "low/high",
"category": "implementation",
"message": "issue description",
"next_actions": [
"specific actionable fix"
]
}
]
}
"""

TECH_LEAD_FINAL_SCHEMA = """
{
  "approved": true/false,
  "should_reset": true/false,
  "reset_reason": "short explanation of why prior context is no longer trustworthy",
  "issues": [
    {
      "severity": "low/high",
      "category": "implementation",
      "message": "issue description",
      "next_actions": ["actionable fixes"]
    }
  ]
}
"""

ARCH_FINAL_SCHEMA = """
{
  "approved": true/false,
  "should_reset": true/false,
  "reset_reason": "short explanation of why prior context is no longer trustworthy",
  "issues": [
    {
      "severity": "low/high",
      "category": "design",
      "message": "issue description",
      "next_actions": ["actionable fixes"]
    }
  ]
}
"""

PRODUCT_MANAGER_SCHEMA = """
{
  "task_specification": "fully refined and engineering-ready task description",
  "original_input_preserved": true,
  "clarifications_made": [
    "explicit assumptions, refinements, missing requirements filled in, and interpretation decisions"
  ],
  "files": [
    "if user request refers to any files - extract file names/pathes, and put them here as is"
  ],
  "proper_nouns": [
    "if user request refers to any proper nouns which are *not* file names/pathes - extract them, and put them here as is"
  ],
  "facts": [
    "if user request states specific facts - extract them, and put them here"
  ]
}
"""

PM_SYNTHESIZER_SCHEMA = """
{
  "task_specification": "final refined engineering-ready task description",
  "selected_candidate": 1,
  "selection_reason": "why this candidate was closest to the correct interpretation",
  "rejected_candidates": [
    {
      "candidate": 2,
      "reason": "why this interpretation was rejected"
    }
  ],
  "common_requirements": ["requirements most candidates agree on"],
  "candidate_disagreements": ["common points of disagreement between most candidates"],
  "speculative_expansions": ["what candidates clearly speculate on"],
  "missing_but_necessary_details": ["details not commonly found in candidate interpretations that are clearly required"],
  "clarifications_made": [
    "final assumptions and interpretation decisions"
  ],
  "files": [
    "if user request refers to any files - extract file names/pathes, and put them here as is"
  ],
  "proper_nouns": [
    "if user request refers to any proper nouns which are *not* file names/pathes - extract them, and put them here as is"
  ],
  "facts": [
    "if user request states specific facts - extract them, and put them here"
  ]
}
"""

PM_REVIEW_SCHEMA = """
{
  "approved": true,
  "should_reset": false,
  "reset_reason": "",
  "issues": [
    {
      "severity": "low/high",
      "category": "product",
      "message": "issue description",
      "next_actions": [
        "specific actionable fix"
      ]
    }
  ]
}
"""

SYSTEM_DECOMPOSITION_SCHEMA = """
{
"decomposition": {
"summary": "high-level explanation of how the request was broken down into implementation domains",
"reviewer_notes": [
"notes about decomposition decisions, coupling concerns, assumptions, and why certain areas were grouped or separated"
],
"domains": [
{
"id": 1,
"name": "short domain name",
"scope": "clear description of what belongs in this domain",
"includes": [
"specific responsibility",
"specific component",
"specific subsystem"
],
"excludes": [
"related responsibility intentionally handled elsewhere"
],
"dependencies": [
"name of prerequisite domain"
],
"reasoning": "why this domain is separated, why it is cohesive, and why it should be implemented at this stage",
"architect_input": "fully scoped architecture request for this domain, written so it can be passed directly to a software architect without additional processing"
}
],
"integration_order": [
"ordered list of domain names representing recommended execution sequence"
],
"global_risks": [
"cross-domain risk, coupling issue, sequencing concern, or area requiring careful validation"
]
}
}
"""

SYSTEM_DECOMPOSITION_REVIEW_SCHEMA = """
{
"approved": true/false,
"should_reset": true/false,
"reset_reason": "short explanation of why prior context is no longer trustworthy",
"issues": [
{
"severity": "low/high",
"category": "decomposition",
"message": "description of the issue",
"next_actions": [
"specific actionable fix"
]
}
]
}
"""
