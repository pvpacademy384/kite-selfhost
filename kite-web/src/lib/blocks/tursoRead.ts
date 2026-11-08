import { z } from "zod";
import { templated } from "../flow/dataSchema";
import { BlockDefinition } from "./types";

export const tursoRead: BlockDefinition = {
  type: "action_turso_read",
  id_prefix: "turso",
  title: "Read rows",
  description: "Read rows from your Turso database with a SELECT query",
  icon: "table-properties",
  category: "Turso",
  requires: ["turso"],
  credits: 1,
  strict_settings: true,
  fields: [
    {
      name: "turso_sql",
      schema: z
        .string()
        .min(1, "Required")
        .max(4000)
        .describe(
          "A SELECT query, like SELECT points FROM users WHERE user_id = ?. Put a ? where a value goes and the value in turso_params. Placeholders don't work in the query itself, so what members type can't change it."
        ),
    },
    {
      name: "turso_params",
      schema: templated(
        z.string().optional(),
        "The values for the ? marks of turso_sql, one per line and in the same order, like {{user.id}}. Placeholders work here."
      ),
    },
  ],
  result: {
    schema: z
      .object({
        rows: z
          .array(z.record(z.string(), z.unknown()))
          .describe(
            "The rows, up to 100, each with a value for every column by its name"
          ),
        first: z
          .record(z.string(), z.unknown())
          .nullable()
          .describe("The first row, or empty if the query found none"),
        count: z.number().describe("Number of rows in rows"),
        found: z.boolean().describe("Whether the query found any rows"),
        truncated: z
          .boolean()
          .describe(
            "Whether there were more than 100 rows and the rest was left out"
          ),
        columns: z.array(z.string()).describe("Names of the columns"),
      })
      .describe("The rows the query returned"),
  },
  run: { kind: "custom" },
};
