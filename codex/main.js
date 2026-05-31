#!/usr/bin/env node

import { Command } from "commander";
import fs from "fs";
import process from "process";
import { Codex } from "@openai/codex-sdk";

const program = new Command();

const codex = new Codex({
  apiKey: process.env.OPENAI_API_KEY,
});

function logErr(msg) {
  process.stderr.write(msg + "\n");
}

function logEvent(event) {
  logErr(JSON.stringify(event, null, 2).split("\n").map(line => line.slice(0, 80)).join("\n"));
}

async function readStdin() {
  return new Promise((resolve, reject) => {
    let data = "";

    process.stdin.setEncoding("utf8");

    process.stdin.on("data", chunk => {
      data += chunk;
    });

    process.stdin.on("end", () => resolve(data.trim()));
    process.stdin.on("error", reject);
  });
}

async function runExec({ resumeThreadId, outputSchemaPath, chdir }) {
  try {
    const opts = {
      sandboxMode: "danger-full-access",
      workingDirectory: chdir,
      skipGitRepoCheck: true,
      approvalPolicy: "never",
    };
    const thread = resumeThreadId
      ? codex.resumeThread(resumeThreadId, opts)
      : codex.startThread(opts);

    const prompt = await readStdin();

    let options = {};
    if (outputSchemaPath) {
      const schema = JSON.parse(
        fs.readFileSync(outputSchemaPath, "utf8")
      );

      options.responseFormat = {
        type: "json_schema",
        json_schema: schema,
      };
    }

    const finalOutput = await runUntilAnswer(thread, prompt, options);

    process.stdout.write(finalOutput + "\n");

    process.exit(0);
  } catch (err) {
    logErr(`[error] ${err.message}`);
    process.exit(1);
  }
}

async function runUntilAnswer(thread, prompt, options) {
  let attempt = 0;
  let currentPrompt = prompt;
  let finalOutput = null;

  while (true) {
    attempt++;

    const { events } = await thread.runStreamed(currentPrompt, options);

    finalOutput = null;

    for await (const event of events) {
      logEvent(event);

      if (event.type === "thread.started") {
        process.stderr.write("session id: " + event.thread_id + "\n");
      }

      // Primary path
      if (
        event.type === "item.completed" &&
        event.item?.type === "agent_message"
      ) {
        finalOutput = event.item.text;
      }

      // Fallback path
      if (
        event.type === "turn.completed" &&
        finalOutput == null &&
        Array.isArray(event.output)
      ) {
        const last = [...event.output]
          .reverse()
          .find(i => i.type === "agent_message");

        if (last?.text) {
          finalOutput = last.text;
        }
      }
    }

    // ✅ Success
    if (finalOutput && finalOutput.trim() !== "") {
      return finalOutput;
    }

    // ❗ Retry with forcing prompt
    logErr(`[warn] No final message, retrying (attempt ${attempt})`);

    currentPrompt = `
Produce a final user-facing answer for the previous request.

Constraints:
- Do not call tools
- Do not be empty
- Respond with the final answer only
`;
  }
}

program
  .option("--cd <path>")
  .command("exec")
  .option("--output-schema <path>")
  .argument("[resume]")
  .argument("[thread_id]")
  .action(async (resumeKeyword, threadId, options) => {
    const isResume = resumeKeyword === "resume";

    if (isResume && !threadId) {
      logErr("Missing thread_id");
      process.exit(1);
    }

    options = {...program.opts(), ...options};
    if (options.cd) {
      process.chdir(options.cd);
    }

    await runExec({
      resumeThreadId: isResume ? threadId : null,
      outputSchemaPath: options.outputSchema,
      chdir: options.cd,
    });
  });

program.parse(process.argv);
