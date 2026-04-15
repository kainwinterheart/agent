# =========================
# UTIL
# =========================

import sys
import subprocess
import json
import time
import re
import asyncio
import os
import glob
from typing import Optional
from config import MAX_REPAIR_ATTEMPTS, DOCUMENT_STORES_DIR


def log(step, msg):
    sys.stderr.write(f">> [{time.strftime('%Y-%m-%d %H:%M:%S')}] [{step}] {msg}\n")


async def run_codex_async(sess_id: Optional[str], prompt: str, ephemeral: bool, timeout: Optional[str] = None) -> tuple[str, Optional[str]]:
    cmd_args = []
    if timeout:
        cmd_args.extend(["timeout", timeout])
    if sess_id:
        cmd_args.extend(["codex", "exec", "resume", sess_id, prompt])
    else:
        cmd_args.extend(["codex", "exec", prompt])
    if ephemeral:
        cmd_args.append("--ephemeral")
    
    process = await asyncio.create_subprocess_exec(
        *cmd_args,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        stdin=subprocess.DEVNULL
    )
    
    async def read_stderr_stream_task(process):
        out = ''
        while True:
            chunk = await process.stderr.read(1024)
            if not chunk:
                break
            decoded = chunk.decode('utf-8')
            sys.stderr.write(decoded)
            out += decoded
        return out
    
    async def read_stdout_buffer_task(process):
        out = ''
        while True:
            chunk = await process.stdout.read(1024)
            if not chunk:
                break
            out += chunk.decode('utf-8')
        return out
    
    task_a = asyncio.create_task(read_stderr_stream_task(process))
    task_b = asyncio.create_task(read_stdout_buffer_task(process))
    
    try:
        stderr, stdout = await asyncio.gather(task_a, task_b)
    finally:
        if process.returncode is None:
            process.kill()
            await process.wait()
    
    if not sess_id and not ephemeral and (match := re.search(r"session id:\s*(\S*)", stderr)):
        sess_id = match.group(1)
    
    if not stdout:
        raise RuntimeError("Empty output, likely timeout issue")
    return stdout, sess_id


def run_codex(session: Optional[str], prompt: str, ephemeral: bool, timeout: Optional[str] = None) -> tuple[str, Optional[str]]:
    """Sync wrapper for async run_codex_async - maintains backward compatibility."""
    return asyncio.run(run_codex_async(session, prompt, ephemeral, timeout))


def extract_json(text: str):
    start = text.find("{")
    end = text.rfind("}") + 1
    if start != -1 and end != -1:
        return text[start:end]
    raise ValueError("Text does not contain JSON object")


def run_json_agent(agent, input_text):
    raw = agent.run(input_text)
    for _ in range(MAX_REPAIR_ATTEMPTS):
        try:
            return json.loads(extract_json(raw))
        except KeyboardInterrupt:
            raise
        except Exception as e:
            raw = agent.run(f"""
Your previous output was invalid JSON; it failed to parse with the following error:
```
{e}
```

Fix it. ONLY return valid JSON.

Previous output:
{raw}
""")
    
    raise ValueError("Agent failed to respond with JSON")


def assert_not_empty(obj, step):
    if not obj:
        raise RuntimeError(f"{step} produced empty output")


def markdown_document_generator(content: dict, stage_name: str, subdir: str) -> None:
    """Generate Markdown document from JSON dict content and persist to file.
    
    Args:
        content (dict): Dict containing the structured data for the stage
        stage_name (str): Name of the workflow stage
    
    Returns:
        None: Function has side effect of creating a Markdown file in DOCUMENT_STORES_DIR
    
    Raises:
        RuntimeError: If required validation fields are missing from content dict
    
    Side effects:
        - Creates DOCUMENT_STORES_DIR if it doesn't exist using os.makedirs()
        - Writes Markdown content to <stage_name>_<timestamp>.md file
        - Logs generation event via log() utility
    """
    # Validate required fields based on stage type - consistent error handling across all stages
    required_keys = {
        'product_manager_final': ['task_specification'],
        'decomposition_final': ['summary', 'domains'],
        'architecture_after_reviews': ['overview'],
        'tech_plan_after_reviews': ['summary']
    }
    
    if stage_name not in required_keys:
        raise ValueError(f"Unknown stage: {stage_name}")
    
    missing_key = None
    for key in required_keys[stage_name]:
        # Determine which nested structure to check based on stage type
        if stage_name == 'decomposition_final':
            actual_content = content.get('decomposition', {})
            if isinstance(actual_content, dict):
                for req_field in required_keys[stage_name]:
                    if req_field not in actual_content:
                        missing_key = req_field
                        break
        else:
            # For other stages, use existing validation logic
            if stage_name == 'architecture_after_reviews':
                if 'architecture' in content:
                    actual_content = content['architecture']
                else:
                    raise RuntimeError(f"Missing 'architecture' key for {stage_name} stage")
            elif stage_name == 'tech_plan_after_reviews':
                if 'plan' in content:
                    actual_content = content['plan']
                else:
                    raise RuntimeError(f"Missing 'plan' key for {stage_name} stage")
            else:
                actual_content = content
            
            for req_field in required_keys[stage_name]:
                if req_field not in actual_content:
                    missing_key = req_field
                    break
        
        if missing_key:
            raise RuntimeError(f"Missing required field '{missing_key}' for {stage_name} stage")
    
    try:
        # Create document_stores directory if needed using config value
        os.makedirs(os.path.join(DOCUMENT_STORES_DIR, subdir), exist_ok=True)
    except OSError as e:
        log("MARKDOWN", f"Failed to create directory {DOCUMENT_STORES_DIR}: {e}")
        raise RuntimeError(f"Cannot create document storage directory: {e}")
    
    # Generate timestamp for filename
    timestamp = time.strftime('%Y-%m-%d_%H-%M-%S')
    filename = f"{stage_name}_{timestamp}.md"
    filepath = os.path.join(DOCUMENT_STORES_DIR, subdir, filename)
    
    # Build markdown sections based on stage type
    markdown_content = ""
    
    if stage_name == 'product_manager_final':
        actual_content = content
        markdown_content += f"# Task Specification\n\n{actual_content.get('task_specification', 'N/A')}\n\n"
    elif stage_name == 'decomposition_final':
        actual_content = content.get('decomposition', {})
        markdown_content += "# Decomposition\n\n"
        
        if 'domains' in actual_content:
            markdown_content += "## Domains\n\n"
            domains = actual_content['domains']
            if isinstance(domains, list):
                for domain in domains:
                    if isinstance(domain, dict):
                        id_val = domain.get('id', '')
                        if architect_input := domain.get('architect_input'):
                            markdown_content += f"### Domain {id_val}\n\n"
                            markdown_content += f"{architect_input}\n\n"
    elif stage_name == 'architecture_after_reviews':
        actual_content = content.get('architecture', {})
        # Architecture stage uses overview
        markdown_content += "# Architecture\n\n"
        markdown_content += "## Overview\n\n"
        markdown_content += f"{actual_content.get('overview', 'N/A')}\n\n"
        
        if 'components' in actual_content:
            markdown_content += "## Components\n\n"
            components = actual_content['components']
            if isinstance(components, list):
                for component in components:
                    if isinstance(component, dict):
                        name = component.get('name', '')
                        responsibility = component.get('responsibility', 'N/A')
                        background = component.get('background', 'N/A')
                        if responsibility == 'N/A' or background == 'N/A':
                            log("MARKDOWN", f"Missing fields in component dict: falling back to defaults")
                        markdown_content += f"### {name}\n\n"
                        markdown_content += f"**Responsibility**: {responsibility}\n\n"
                        markdown_content += f"**Background**: {background}\n\n"
        
        if 'data_flow' in actual_content:
            markdown_content += "## Data Flow\n\n"
            data_flow = actual_content['data_flow']
            if isinstance(data_flow, list):
                for flow in data_flow:
                    markdown_content += f"- {flow}\n\n"
        
        if 'tech_choices' in actual_content:
            markdown_content += "## Tech Choices\n\n"
            tech_choices = actual_content['tech_choices']
            if isinstance(tech_choices, list):
                for choice in tech_choices:
                    markdown_content += f"- {choice}\n\n"
        
        if 'constraints' in actual_content:
            markdown_content += "## Constraints\n\n"
            constraints = actual_content['constraints']
            if isinstance(constraints, list):
                for constraint in constraints:
                    markdown_content += f"- {constraint}\n\n"
    elif stage_name == 'tech_plan_after_reviews':
        actual_content = content.get('plan', {})
        # Plan stage uses summary instead of overview
        markdown_content += "# Implementation plan\n\n"
        markdown_content += "## Summary\n\n"
        markdown_content += f"{actual_content.get('summary', 'N/A')}\n\n"
        
        if 'files' in actual_content:
            markdown_content += "## Files\n\n"
            files = actual_content['files']
            if isinstance(files, list):
                for file_item in files:
                    if isinstance(file_item, dict):
                        path = file_item.get('path', '')
                        purpose = file_item.get('purpose', 'N/A')
                        background = file_item.get('background', 'N/A')
                        markdown_content += f"### {path}\n\n"
                        markdown_content += f"**Purpose**: {purpose}\n\n"
                        markdown_content += f"**Background**: {background}\n\n"
        
        if 'steps' in actual_content:
            markdown_content += "## Steps\n\n"
            steps = actual_content['steps']
            if isinstance(steps, list):
                for step in steps:
                    if isinstance(step, dict):
                        id_val = step.get('id', '')
                        description = step.get('description', 'N/A')
                        markdown_content += f"### Step {id_val}\n\n"
                        markdown_content += f"{description}\n\n"
    
    # Write file with error handling
    try:
        with open(filepath, 'w') as f:
            f.write(markdown_content)
    except (IOError, OSError) as e:
        log("MARKDOWN", f"Failed to write file {filepath}: {e}")
        raise RuntimeError(f"Cannot create Markdown document: {e}")
