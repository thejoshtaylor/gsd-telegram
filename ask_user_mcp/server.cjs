#!/usr/bin/env node
/**
 * Ask User MCP Server — CJS entry point for Node.js compatibility.
 *
 * Uses dynamic import() to load the ESM @modelcontextprotocol/sdk.
 * Node resolves the SDK from the parent telegram-claude/node_modules.
 *
 * When Claude calls ask_user(), this server writes a request file that the
 * Telegram bot monitors. The bot displays inline keyboard buttons.
 * When the user taps a button, their choice is sent back to Claude as a message.
 */

const { randomUUID } = require("crypto");
const { writeFileSync } = require("fs");
const { tmpdir } = require("os");
const { resolve } = require("path");

async function main() {
  const { Server } = await import("@modelcontextprotocol/sdk/server/index.js");
  const { StdioServerTransport } = await import(
    "@modelcontextprotocol/sdk/server/stdio.js"
  );
  const { CallToolRequestSchema, ListToolsRequestSchema } = await import(
    "@modelcontextprotocol/sdk/types.js"
  );

  const server = new Server(
    { name: "ask-user", version: "1.0.0" },
    { capabilities: { tools: {} } }
  );

  server.setRequestHandler(ListToolsRequestSchema, async () => ({
    tools: [
      {
        name: "ask_user",
        description:
          "Present options to the user as tappable inline buttons in Telegram. " +
          "IMPORTANT: After calling this tool, STOP and wait. Do NOT add any text after " +
          "calling this tool — the user will tap a button and their choice becomes their " +
          "next message. Just call the tool and end your turn.",
        inputSchema: {
          type: "object",
          properties: {
            question: {
              type: "string",
              description: "The question to ask the user",
            },
            options: {
              type: "array",
              items: { type: "string" },
              description:
                "List of options for the user to choose from (2–6 options recommended)",
              minItems: 2,
              maxItems: 10,
            },
          },
          required: ["question", "options"],
        },
      },
    ],
  }));

  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    if (request.params.name !== "ask_user") {
      throw new Error(`Unknown tool: ${request.params.name}`);
    }

    const args = request.params.arguments;
    const question = (args && args.question) || "";
    const options = (args && args.options) || [];

    if (!question || options.length < 2) {
      throw new Error("question and at least 2 options required");
    }

    const requestUuid = randomUUID().slice(0, 8);
    const chatId = process.env.TELEGRAM_CHAT_ID || "";

    const requestData = {
      request_id: requestUuid,
      question,
      options,
      status: "pending",
      chat_id: chatId,
      created_at: new Date().toISOString(),
    };

    const requestFile = resolve(tmpdir(), `ask-user-${requestUuid}.json`);
    writeFileSync(requestFile, JSON.stringify(requestData, null, 2));

    return {
      content: [
        {
          type: "text",
          text: "[Buttons sent to user. STOP HERE — do not output any more text. Wait for user to tap a button.]",
        },
      ],
    };
  });

  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("Ask User MCP server running on stdio");
}

main().catch(console.error);
