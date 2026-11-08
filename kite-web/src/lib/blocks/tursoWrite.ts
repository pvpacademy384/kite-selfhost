import { z } from "zod";
import { templated } from "../flow/dataSchema";
import { BlockDefinition } from "./types";

export const tursoWrite: BlockDefinition = {
  type: "action_turso_write",
  id_prefix: "turso",
  title: "Write data",
  description: "Add, change or delete data in your Turso database",
  icon: "database-zap",
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
          "An INSERT, UPDATE, DELETE, REPLACE, CREATE or ALTER query, like UPDATE users SET points = points + 1 WHERE user_id = ?. UPDATE and DELETE need a WHERE. Put a ? where a value goes and the value in turso_params. Placeholders don't work in the query itself."
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
        affected_rows: z
          .number()
          .describe("Number of rows the query added, changed or deleted"),
        last_insert_id: z
          .number()
          .nullable()
          .describe("ID of the last row added, if the table has one"),
      })
      .describe("What the query changed"),
  },
  run: { kind: "custom" },
};
