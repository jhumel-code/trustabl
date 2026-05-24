import { tool, query } from "@anthropic-ai/claude-agent-sdk";

export const t1 = tool("a", "A", {}, async () => ({ content: [] }));
export const t2 = tool("b", "B", {}, async () => ({ content: [] }));

export const q = query({ options: { agents: {
  agentA: { description: "A", prompt: "P" },
  agentB: { description: "B", prompt: "P" }
}}});
