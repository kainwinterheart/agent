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
import jsonschema
from contextlib import ExitStack
from typing import Optional
from config import DOCUMENT_STORES_DIR
from tempfile import NamedTemporaryFile, TemporaryFile
from schema_utils import schema_to_example


def log(step, msg):
    sys.stderr.write(f">> [{time.strftime('%Y-%m-%d %H:%M:%S')}] [{step}] {msg}\n")


async def run_codex_async(sess_id: Optional[str], prompt: str, schema: dict, ephemeral: bool, timeout: Optional[str] = None) -> tuple[str, Optional[str]]:
    cmd_args = []
    if timeout:
        cmd_args.extend(["timeout", timeout])
    if sess_id:
        cmd_args.extend(["codex", "exec", "resume", sess_id])
    else:
        cmd_args.extend(["codex", "exec"])
    if ephemeral:
        cmd_args.append("--ephemeral")
    
    with ExitStack() as stack:
        if not sess_id:
            schema_file = stack.enter_context(NamedTemporaryFile(delete_on_close=False))
            schema_file.write(json.dumps(schema, indent=2).encode('utf-8'))
            schema_file.close()
            cmd_args.append("--output-schema")
            cmd_args.append(schema_file.name)
        prompt_file = stack.enter_context(TemporaryFile(buffering=0))
        prompt_file.write(prompt.encode('utf-8'))
        prompt_file.seek(0)
        process = await asyncio.create_subprocess_exec(
            *cmd_args,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            stdin=prompt_file,
        )
        
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


def run_codex(session: Optional[str], prompt: str, schema: dict, ephemeral: bool, timeout: Optional[str] = None) -> tuple[str, Optional[str]]:
    """Sync wrapper for async run_codex_async - maintains backward compatibility."""
    return asyncio.run(run_codex_async(session, prompt, schema, ephemeral, timeout))


def extract_json(text: str):
    start = text.find("{")
    end = text.rfind("}") + 1
    if start != -1 and end != -1:
        return text[start:end]
    raise ValueError("Text does not contain JSON object")


def run_json_agent(agent, input_text):
    raw = agent.run(input_text)
    while True:
        try:
            out = json.loads(extract_json(raw))
            jsonschema.validate(out, agent.schema)
            return out
        except KeyboardInterrupt:
            raise
        except Exception as e:
            msg = ""
            example = "."
            if isinstance(e, jsonschema.exceptions.ValidationError):
                path = ".".join(map(str, e.path))
                if path:
                    msg += f"Error within {path}: "
                msg += e.message
                example = ":\n"
                example += schema_to_example(e.schema)
            else:
                msg += str(e)
            raw = agent.run(f"""
Your previous output was invalid JSON; it failed to parse with the following error:
```
{msg}
```

Fix it. ONLY return valid JSON{example}

Previous output:
```
{raw}
```

Previous task:
```
{input_text}
```
""")
    

def assert_not_empty(obj, step):
    if not obj:
        raise RuntimeError(f"{step} produced empty output")


def markdown_document_generator(content: dict, stage_name: str, subdir: list[str]) -> None:
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
    try:
        # Create document_stores directory if needed using config value
        os.makedirs(os.path.join(DOCUMENT_STORES_DIR, *subdir), exist_ok=True)
    except OSError as e:
        log("MARKDOWN", f"Failed to create directory {DOCUMENT_STORES_DIR}: {e}")
        raise RuntimeError(f"Cannot create document storage directory: {e}")
    
    # Generate timestamp for filename
    timestamp = time.strftime('%Y-%m-%d_%H-%M-%S')
    filename = f"{stage_name}_{timestamp}.md"
    filepath = os.path.join(DOCUMENT_STORES_DIR, *subdir, filename)
    
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
                for domain_index, domain in enumerate(domains):
                    if isinstance(domain, dict):
                        id_val = domain.get('id', domain_index + 1)
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


async def read_stderr_stream_task(process):
    out = ''
    while True:
        chunk = await process.stderr.read(65535)
        if not chunk:
            break
        decoded = chunk.decode('utf-8')
        sys.stderr.write(decoded)
        out += decoded
    return out


async def read_stdout_buffer_task(process):
    out = ''
    while True:
        chunk = await process.stdout.read(65535)
        if not chunk:
            break
        out += chunk.decode('utf-8')
    return out
