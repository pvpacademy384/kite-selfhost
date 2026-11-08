import { z } from "zod";
import { BlockDefinition } from "./types";

export const tursoCheck: BlockDefinition = {
  type: "action_turso_check",
  id_prefix: "turso",
  title: "Test connection",
  description: "Check that your Turso database answers",
  icon: "plug-zap",
  category: "Turso",
  requires: ["turso"],
  credits: 1,
  strict_settings: true,
  fields: [],
  result: {
    schema: z
      .object({
        ok: z.boolean().describe("True when the database answered"),
        latency_ms: z
          .number()
          .describe("How long the database took to answer, in milliseconds"),
      })
      .describe("The answer of the database"),
  },
  run: { kind: "custom" },
};
