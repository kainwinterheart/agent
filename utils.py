# =========================
# UTIL
# =========================

import asyncio
import json
import logging
import os
import re
import subprocess
import sys
import time
from contextlib import ExitStack
from tempfile import NamedTemporaryFile, TemporaryFile, mkstemp
from typing import Optional

import jsonschema

import prompts
from schema_utils import schema_to_example


def log(step, msg):
    sys.stderr.write(f">> [{time.strftime('%Y-%m-%d %H:%M:%S')}] [{step}] {msg}\n")


async def run_codex_async(
    agent_name: str,
    sess_id: Optional[str],
    prompt: str,
    schema: dict,
    timeout: Optional[str] = None,
) -> tuple[str, Optional[str]]:
    cmd_args = []
    if timeout:
        cmd_args.extend(["timeout", "-s", "9", timeout])
    cmd_args.extend(["codex", "exec"])

    with ExitStack() as stack:
        if not os.environ.get("AC_AGENT_NO_SCHEMA"):
            schema_file = stack.enter_context(NamedTemporaryFile(delete_on_close=False))
            schema_file.write(json.dumps(schema, indent=2).encode("utf-8"))
            schema_file.close()
            cmd_args.append("--output-schema")
            cmd_args.append(schema_file.name)
        if sess_id:
            cmd_args.extend(["resume", sess_id])
        prompt_file = stack.enter_context(TemporaryFile(buffering=0))
        prompt_file.write(prompt.encode("utf-8"))
        prompt_file.seek(0)
        process = await asyncio.create_subprocess_exec(
            *cmd_args,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            stdin=prompt_file,
            env={**os.environ, "AC_AGENT_NAME": agent_name},
        )

        task_a = asyncio.create_task(read_stderr_stream_task(process))
        task_b = asyncio.create_task(read_stdout_buffer_task(process))

        try:
            stderr, stdout = await asyncio.gather(task_a, task_b)
        finally:
            if process.returncode is None:
                process.kill()
                await process.wait()

    if match := re.search(r"session id:\s*(\S*)", stderr):
        sess_id = match.group(1)

    stdout = stdout.strip()
    if not stdout:
        raise RuntimeError("Empty output, likely timeout issue")
    return stdout, sess_id


def run_codex(
    agent_name: str,
    session: Optional[str],
    prompt: str,
    schema: dict,
    timeout: Optional[str] = None,
) -> tuple[str, Optional[str]]:
    """Sync wrapper for async run_codex_async - maintains backward compatibility."""
    return asyncio.run(run_codex_async(agent_name, session, prompt, schema, timeout))


def extract_json(text: str):
    start = text.find("{")
    end = text.rfind("}") + 1
    if start != -1 and end != -1:
        return text[start:end]
    raise ValueError("Text does not contain JSON object")


def atomic_write(path: str, content: str) -> None:
    """Write content to a temporary file in the same directory, then atomically rename."""
    dir_path = os.path.dirname(path)
    if dir_path:
        os.makedirs(dir_path, exist_ok=True)
    fd, tmp_path = mkstemp(dir=dir_path, suffix=".tmp")
    try:
        with os.fdopen(fd, "w") as f:
            f.write(content)
        os.replace(tmp_path, path)
    except Exception:
        os.unlink(tmp_path)
        raise


def run_json_agent(
    agent,
    input_text,
    invocation_id: str,
    subdir: list[str],
    return_system_state: bool = False,
):
    raw = None
    updated = False
    # Cache check
    cache_file = os.path.join(
        build_path(subdir, ".state"), f"{agent.name}_{invocation_id}.out"
    )
    if os.path.exists(cache_file):
        log(invocation_id, f"Using cached response: {cache_file}")
        with open(cache_file, "r") as f:
            raw = f.read()

    if raw is None:
        atomic_write(
            os.path.join(
                build_path(subdir, ".state"), f"{agent.name}_{invocation_id}.in"
            ),
            input_text,
        )
        raw = agent.run(input_text)
        updated = True
    else:  # XXX
        try:
            x = json.loads(extract_json(raw), strict=False)
            kludged = False
            if "approved" in x and "approved_confidence" not in x:
                x["approved_confidence"] = "low"
                x["approved_reason"] = "backfill"
                x["resolved_issues"] = []
                kludged = True
            if (
                "tech_lead_final" == agent.name or "plan" in x
            ) and "next_steps" not in x:
                x["next_steps"] = []
                kludged = True
            if kludged:
                raw = json.dumps(x)
        except:
            pass
    while True:
        if updated:
            atomic_write(cache_file, raw)
        try:
            out = json.loads(extract_json(raw), strict=False)
            jsonschema.validate(out, agent.schema)
            if agent.ephemeral:
                agent.reset()
            else:
                agent.last_correct_response = out
            if return_system_state:
                return {"out": out, "from_cache": not updated}
            else:
                return out
        except KeyboardInterrupt:
            raise
        except Exception as e:
            msg = ""
            if isinstance(e, jsonschema.exceptions.ValidationError):
                path = ".".join(map(str, e.path))
                if path:
                    msg += f"Error within {path}: "
                msg += e.message
            else:
                msg += str(e)
            updated = True
            raw = agent.run(f"""
{agent.resume_prompt}

<feedback>
Your previous output failed JSON validation:
<error>
{msg}
</error>

Output MUST be valid JSON only:
{schema_to_example(agent.schema)}
</feedback>
""")


def assert_not_empty(obj, step):
    if not obj:
        raise RuntimeError(f"{step} produced empty output")


def markdown_document_generator(
    content: dict, stage_name: str, subdir: list[str]
) -> None:
    # Generate timestamp for filename
    timestamp = time.strftime("%Y-%m-%d_%H-%M-%S")
    filename = f"{timestamp}_{stage_name}.md"
    target_dir = build_path(subdir, "document_stores")
    if os.path.exists(target_dir):
        for f in os.listdir(target_dir):
            if f.endswith(f"_{stage_name}.md"):
                filepath = os.path.join(target_dir, f)
                log("MARKDOWN", f"{filepath} already exists")
                return filepath
    filepath = os.path.join(target_dir, filename)

    # Build markdown sections based on stage type
    markdown_content = ""

    if stage_name == "product_manager_final":
        actual_content = content
        markdown_content += f"# Task Specification\n\n{actual_content.get('task_specification', 'N/A')}\n\n"
        files = actual_content.get("files")
        if isinstance(files, list) and files:
            markdown_content += "## Mentioned files\n\n"
            for file in files:
                markdown_content += f"- {file}\n\n"
        proper_nouns = actual_content.get("proper_nouns")
        if isinstance(proper_nouns, list) and proper_nouns:
            markdown_content += "## Mentioned proper nouns\n\n"
            for proper_noun in proper_nouns:
                markdown_content += f"- {proper_noun}\n\n"
        facts = actual_content.get("facts")
        if isinstance(facts, list) and facts:
            markdown_content += "## Stated facts\n\n"
            for fact in facts:
                markdown_content += f"- {fact}\n\n"
        missing_but_necessary_details = actual_content.get(
            "missing_but_necessary_details"
        )
        if (
            isinstance(missing_but_necessary_details, list)
            and missing_but_necessary_details
        ):
            markdown_content += "## Additional considerations\n\n"
            for missing_but_necessary_detail in missing_but_necessary_details:
                markdown_content += f"- {missing_but_necessary_detail}\n\n"
        speculative_expansions = actual_content.get("speculative_expansions")
        if isinstance(speculative_expansions, list) and speculative_expansions:
            markdown_content += "## Out of scope\n\n"
            for speculative_expansion in speculative_expansions:
                markdown_content += f"- {speculative_expansion}\n\n"
    elif stage_name == "decomposition_final":
        actual_content = content.get("decomposition", {})
        markdown_content += "# Decomposition\n\n"

        if "domains" in actual_content:
            markdown_content += "## Domains\n\n"
            domains = actual_content["domains"]
            if isinstance(domains, list):
                for domain_index, domain in enumerate(domains):
                    if isinstance(domain, dict):
                        id_val = domain.get("id", domain_index + 1)
                        if architect_input := domain.get("architect_input"):
                            markdown_content += f"### Domain {id_val}\n\n"
                            markdown_content += f"{architect_input}\n\n"
    elif stage_name == "architecture_after_reviews":
        actual_content = content.get("architecture", {})
        # Architecture stage uses overview
        markdown_content += "# Architecture\n\n"
        markdown_content += "## Overview\n\n"
        markdown_content += f"{actual_content.get('overview', 'N/A')}\n\n"

        if "components" in actual_content:
            markdown_content += "## Components\n\n"
            components = actual_content["components"]
            if isinstance(components, list):
                for component in components:
                    if isinstance(component, dict):
                        name = component.get("name", "")
                        responsibility = component.get("responsibility", "N/A")
                        background = component.get("background", "N/A")
                        if responsibility == "N/A" or background == "N/A":
                            log(
                                "MARKDOWN",
                                f"Missing fields in component dict: falling back to defaults",
                            )
                        markdown_content += f"### {name}\n\n"
                        markdown_content += f"**Responsibility**: {responsibility}\n\n"
                        markdown_content += f"**Background**: {background}\n\n"

        if "data_flow" in actual_content:
            markdown_content += "## Data Flow\n\n"
            data_flow = actual_content["data_flow"]
            if isinstance(data_flow, list):
                for flow in data_flow:
                    markdown_content += f"- {flow}\n\n"

        if "tech_choices" in actual_content:
            markdown_content += "## Tech Choices\n\n"
            tech_choices = actual_content["tech_choices"]
            if isinstance(tech_choices, list):
                for choice in tech_choices:
                    markdown_content += f"- {choice}\n\n"

        if "constraints" in actual_content:
            markdown_content += "## Constraints\n\n"
            constraints = actual_content["constraints"]
            if isinstance(constraints, list):
                for constraint in constraints:
                    markdown_content += f"- {constraint}\n\n"
    elif stage_name == "tech_plan_after_reviews":
        actual_content = content.get("plan", {})
        # Plan stage uses summary instead of overview
        markdown_content += "# Implementation plan\n\n"
        markdown_content += "## Summary\n\n"
        markdown_content += f"{actual_content.get('summary', 'N/A')}\n\n"

        if "files" in actual_content:
            markdown_content += "## Files\n\n"
            files = actual_content["files"]
            if isinstance(files, list):
                for file_item in files:
                    if isinstance(file_item, dict):
                        path = file_item.get("path", "")
                        purpose = file_item.get("purpose", "N/A")
                        background = file_item.get("background", "N/A")
                        markdown_content += f"### {path}\n\n"
                        markdown_content += f"**Purpose**: {purpose}\n\n"
                        markdown_content += f"**Background**: {background}\n\n"

        if "steps" in actual_content:
            markdown_content += "## Steps\n\n"
            steps = actual_content["steps"]
            if isinstance(steps, list):
                for step in steps:
                    if isinstance(step, dict):
                        id_val = step.get("id", "")
                        description = step.get("description", "N/A")
                        markdown_content += f"### Step {id_val}\n\n"
                        markdown_content += f"{description}\n\n"

    elif stage_name == "investigation_plan":
        workstreams = content.get("workstreams", [])
        markdown_content += "# Investigation Plan\n\n"
        for N, ws in enumerate(workstreams, 1):
            markdown_content += f"## Workstream {N}\n\n"
            objective = ws.get("objective", "")
            if objective:
                markdown_content += f"{objective}\n\n"
            data_sources = ws.get("data_sources", [])
            if data_sources:
                markdown_content += "Data Sources:\n"
                for ds in data_sources:
                    markdown_content += f"* {ds}\n"
                markdown_content += "\n"
            hypotheses = ws.get("hypotheses", [])
            if hypotheses:
                markdown_content += "Hypotheses:\n"
                for h in hypotheses:
                    markdown_content += f"* {h}\n"
                markdown_content += "\n"
            methods = ws.get("investigation_methods", [])
            if methods:
                markdown_content += "Investigation Methods:\n"
                for m in methods:
                    markdown_content += f"* {m}\n"
                markdown_content += "\n"
            deliverables = ws.get("expected_deliverables", [])
            if deliverables:
                markdown_content += "Expected Deliverables:\n"
                for d in deliverables:
                    markdown_content += f"* {d}\n"
                markdown_content += "\n"

    elif stage_name.startswith("investigation_workstream_"):
        markdown_content += "# Investigation Findings\n\n"
        objective = content.get("workstream_objective", "")
        if objective:
            markdown_content += f"{objective}\n\n"
        conclusions = content.get("conclusions", [])
        if conclusions:
            markdown_content += "Conclusions:\n"
            for c in conclusions:
                markdown_content += f"* {c}\n"
            markdown_content += "\n"
        evidence = content.get("supporting_evidence", [])
        if evidence:
            markdown_content += "Supporting Evidence:\n"
            for item in evidence:
                if isinstance(item, dict):
                    etype = item.get("evidence_type", "")
                    edesc = item.get("evidence_description", "")
                    sref = item.get("source_reference", "")
                    markdown_content += f"* **[{etype}]** {edesc} (ref: {sref})\n"
                else:
                    markdown_content += f"* {item}\n"
            markdown_content += "\n"
        confidence = content.get("confidence_level", "")
        if confidence:
            markdown_content += f"Confidence Level: {confidence}\n\n"
        unanswered = content.get("unanswered_questions", [])
        if unanswered:
            markdown_content += "Unanswered Questions:\n"
            for q in unanswered:
                markdown_content += f"* {q}\n"
            markdown_content += "\n"

    elif stage_name == "investigation_report_final":
        markdown_content += "# Investigation Report\n\n"
        exec_summary = content.get("executive_summary", "")
        if exec_summary:
            markdown_content += f"{exec_summary}\n\n"

        rca = content.get("root_cause_analysis")
        if rca and isinstance(rca, dict):
            markdown_content += "## Root Cause Analysis\n\n"
            primary = rca.get("primary_cause")
            if primary:
                markdown_content += f"### Primary Cause\n\n{primary}\n\n"
            contrib = rca.get("contributing_factors", [])
            if contrib:
                markdown_content += "### Contributing Factors\n\n"
                for f in contrib:
                    markdown_content += f"* {f}\n"
                markdown_content += "\n"
            trail = rca.get("evidence_trail", [])
            if trail:
                markdown_content += "### Evidence Trail\n\n"
                for item in trail:
                    markdown_content += f"* {item}\n"
                markdown_content += "\n"

        timeline = content.get("timeline_reconstruction", [])
        if timeline:
            markdown_content += "## Timeline Reconstruction\n\n"
            for entry in timeline:
                if isinstance(entry, dict):
                    ts = entry.get("timestamp", "")
                    evt = entry.get("event", "")
                    markdown_content += f"### {ts}\n\n{evt}\n\n"

        impact = content.get("customer_impact_assessment")
        if impact and isinstance(impact, dict):
            markdown_content += "## Customer Impact Assessment\n\n"
            affected = impact.get("affected_users")
            if affected:
                markdown_content += f"### Affected Users\n\n{affected}\n\n"
            severity = impact.get("severity")
            if severity:
                markdown_content += f"### Severity\n\n{severity}\n\n"
            duration = impact.get("duration")
            if duration:
                markdown_content += f"### Duration\n\n{duration}\n\n"

        correlations = content.get("correlation_findings", [])
        if correlations:
            markdown_content += "## Correlation Findings\n\n"
            for item in correlations:
                if isinstance(item, dict):
                    obs = item.get("observation", "")
                    strength = item.get("correlation_strength", "")
                    causal = item.get("causal_claim", "")
                    markdown_content += f"* **Observation**: {obs}\n"
                    if strength:
                        markdown_content += f"  **Strength**: {strength}\n"
                    if causal:
                        markdown_content += f"  **Causal Claim**: {causal}\n"
                    markdown_content += "\n"

        hyp_results = content.get("hypothesis_test_results", [])
        if hyp_results:
            markdown_content += "## Hypothesis Test Results\n\n"
            for item in hyp_results:
                if isinstance(item, dict):
                    hyp = item.get("hypothesis", "")
                    test = item.get("test_performed", "")
                    result = item.get("result", "")
                    conclusion = item.get("conclusion", "")
                    markdown_content += f"* **Hypothesis**: {hyp}\n"
                    if test:
                        markdown_content += f"  **Test**: {test}\n"
                    if result:
                        markdown_content += f"  **Result**: {result}\n"
                    if conclusion:
                        markdown_content += f"  **Conclusion**: {conclusion}\n"
                    markdown_content += "\n"

        gaps = content.get("known_gaps_and_unknowns", [])
        if gaps:
            markdown_content += "## Known Gaps and Unknowns\n\n"
            for g in gaps:
                markdown_content += f"* {g}\n"
            markdown_content += "\n"

        recs = content.get("recommendations", [])
        if recs:
            markdown_content += "## Recommendations\n\n"
            for item in recs:
                if isinstance(item, dict):
                    priority = item.get("priority", "")
                    action = item.get("action", "")
                    rationale = item.get("rationale", "")
                    markdown_content += f"* **[{priority}]** {action}"
                    if rationale:
                        markdown_content += f" — {rationale}"
                    markdown_content += "\n"
                else:
                    markdown_content += f"* {item}\n"
            markdown_content += "\n"

    elif stage_name == "investigation_classification":
        markdown_content += "# Investigation Classification\n\n"
        task_type = content.get("type", "unknown")
        reasoning = content.get("reasoning", "")
        markdown_content += f"**Type**: {task_type}\n\n"
        if reasoning:
            markdown_content += f"**Reasoning**: {reasoning}\n\n"

    log("MARKDOWN", f"Writing {filepath}...")
    dir_path = os.path.dirname(filepath)
    if dir_path:
        os.makedirs(dir_path, exist_ok=True)
    with open(filepath, "w") as f:
        f.write(markdown_content)
    return filepath


async def read_stderr_stream_task(process):
    out = ""
    bad = 0
    while True:
        if bad > 100:
            break
        try:
            chunk = await process.stderr.read(65535)
            if not chunk:
                break
            decoded = chunk.decode("utf-8")
            sys.stderr.write(decoded)
            out += decoded
            bad = 0
        except:
            logging.exception("stderr stream error")
            bad += 1
            if (bad % 10) == 0:
                time.sleep(0.1)
    return out


async def read_stdout_buffer_task(process):
    out = ""
    bad = 0
    while True:
        if bad > 100:
            break
        try:
            chunk = await process.stdout.read(65535)
            if not chunk:
                break
            out += chunk.decode("utf-8")
            bad = 0
        except:
            logging.exception("stdout stream error")
            bad += 1
            if (bad % 10) == 0:
                time.sleep(0.1)
    return out


def load_session_id(agent_name: str, subdir: str) -> Optional[str]:
    """Load the session ID from {subdir}/.state/{agent_name}_session.json.

    Returns an empty string if the file does not exist or cannot be read.
    """
    filepath = os.path.join(subdir, ".state", ".sessions", f"{agent_name}.session")
    if os.path.exists(filepath):
        log(agent_name, f"Reading last session ID: {filepath}")
        with open(filepath, "r") as f:
            return f.read()
    return None


def save_session_id(agent_name: str, session_id: str, subdir: str) -> None:
    """Persist the session ID to {subdir}/.state/{agent_name}_session.json via atomic_write.

    Performs a read-before-write comparison: loads the current persisted value and
    only writes if it differs from the proposed new value. This is best-effort;
    concurrent processes may still race between read and write, but atomic writes
    provide partial protection against corruption.
    """
    filepath = os.path.join(subdir, ".state", ".sessions", f"{agent_name}.session")
    atomic_write(filepath, session_id)


def build_path(subdir: list[str], section: str) -> str:
    root_dir, *nested_dirs = subdir
    return os.path.join(root_dir, section, *nested_dirs)


def nudge(
    max_it,
    agent,
    prompt,
    invocation_id_prefix,
    subdir,
    *,
    return_system_state: bool = False,
    nsc: Optional[object] = None,
):
    next_prompt = prompt
    results = []
    for i in range(max_it):
        result = run_json_agent(
            agent,
            next_prompt,
            f"{invocation_id_prefix}-nudge{i}",
            subdir,
            return_system_state=return_system_state,
        )
        results.append(result)
        if return_system_state:
            result = result["out"]
        next_steps = result.pop("next_steps", None)
        if not next_steps:
            break
        if nsc:
            next_steps = run_json_agent(
                nsc,
                f"""
INPUT:
{json.dumps({"next_steps": next_steps})}

Return the filtered list of steps, exactly as written.
Do not include any explanation or commentary.
                """.strip(),
                f"{invocation_id_prefix}-nudge{i}-nsc",
                subdir,
            )["lines"]
            if not next_steps:
                break
        if agent.ephemeral or ((i + 1) % 10 == 0):
            next_prompt = f"END GOAL:\n<reminder>\n{prompt}\n</reminder>\n\n"
        else:
            next_prompt = ""
        if agent.ephemeral:
            next_prompt += f"PREVIOUS RESPONSE: {json.dumps(result)}\n"
        next_prompt += f"ITERATION: {i + 1}/{max_it}\n"
        next_prompt += "<feedback>\nADDRESS YOUR NEXT STEPS:\n"
        for next_step in next_steps:
            next_prompt += f"* {next_step}\n"
        next_prompt += "</feedback>\n"
        if agent.ephemeral:
            next_prompt += "\n"
            next_prompt += prompts.FOLLOWUP
    return results
