import { Command } from "commander";
import fs from "fs";
import process from "process";
import {
  unstable_v2_createSession,
  unstable_v2_resumeSession,
  type SDKMessage,
  type SDKSessionOptions,
} from "@anthropic-ai/claude-agent-sdk";

const program = new Command();

function logErr(msg: string) {
  process.stderr.write(msg + "\n");
}

function logEvent(event: unknown) {
  logErr(
    JSON.stringify(event, null, 2)
      .split("\n")
      .map(line => line.slice(0, 80))
      .join("\n")
  );
}

async function readStdin(): Promise<string> {
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

// Extract assistant text from SDKMessage
function getAssistantText(msg: SDKMessage): string | null {
  if (msg.type !== "assistant") return null;

  return msg.message.content
    .filter(block => block.type === "text")
    .map(block => block.text)
    .join("");
}

async function runExec({
  resumeSessionId,
  outputSchemaPath,
  chdir
}: {
  resumeSessionId?: string | null;
  outputSchemaPath?: string;
  chdir?: string;
}) {
  try {
    const prompt = await readStdin();

    let options: SDKSessionOptions = {
      permissionMode: 'bypassPermissions',
      allowDangerouslySkipPermissions: true,
      cwd: chdir,
    };
    if (outputSchemaPath) {
      const schema = JSON.parse(
        fs.readFileSync(outputSchemaPath, "utf8")
      );

      options.outputFormat = {
        type: "json_schema",
        schema
      };
    }

    const session = resumeSessionId
      ? await unstable_v2_resumeSession(resumeSessionId, options)
      : unstable_v2_createSession(options);

    const finalOutput = await runUntilAnswer(
      session,
      prompt
    );

    process.stdout.write(finalOutput + "\n");

    session.close();
    process.exit(0);
  } catch (err: any) {
    logErr(`[error] ${err.message}`);
    process.exit(1);
  }
}

async function runUntilAnswer(
  session: ReturnType<typeof unstable_v2_createSession>,
  prompt: string,
): Promise<string> {
  let attempt = 0;
  let currentPrompt = prompt;
  let first = true;

  while (true) {
    attempt++;

    // Send input
    await session.send(currentPrompt);

    let finalOutput: string | null = null;

    // Stream response
    for await (const msg of session.stream()) {
      logEvent(msg);

      if (first) {
        first = false;
        process.stderr.write("session id: " + session.sessionId + "\n");
      }

      const text = getAssistantText(msg);
      if (text) {
        finalOutput = text;
      }
    }

    // ✅ Success
    if (finalOutput && finalOutput.trim() !== "") {
      return finalOutput;
    }

    // ❗ Retry
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
  .argument("[session_id]")
  .action(async (resumeKeyword, sessionId, options) => {
    const isResume = resumeKeyword === "resume";

    if (isResume && !sessionId) {
      logErr("Missing session_id");
      process.exit(1);
    }

    await runExec({
      resumeSessionId: isResume ? sessionId : null,
      outputSchemaPath: options.outputSchema,
      chdir: options.cd,
    });
  });

program.parse(process.argv);
