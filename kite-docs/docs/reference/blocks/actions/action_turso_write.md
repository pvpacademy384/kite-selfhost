---
sidebar_position: 37.91
---

import EmbedFlowNode from "../../../../src/components/EmbedFlowNode";
import NodeInfoExplorer from "../../../../src/components/NodeInfoExplorer";

# Write data

<EmbedFlowNode type="action_turso_write" />

The `Write data` block runs an `INSERT`, `UPDATE`, `DELETE`, `REPLACE`, `CREATE` or `ALTER` query in your own [Turso](https://turso.tech) database, for things like adding points or saving a warning. Enable Turso under [Integrations](../../integrations.md) first, and enter your database URL and auth token there.

Put a `?` in the query wherever a value goes, and the values in _Values_, one per line in the same order. For example, the query `UPDATE users SET points = points + 1 WHERE user_id = ?` with `{{ user.id }}` as the value. Placeholders don't work in the query itself, so what members type can't change what the query does.

`UPDATE` and `DELETE` need a `WHERE`, so one block can't change every row by mistake. Use `WHERE 1 = 1` if you mean all rows. `DROP` isn't allowed in flows, do that in the Turso dashboard. A block runs one statement, so use a second block for the next one.

The result has `affected_rows`, how many rows the query changed, and `last_insert_id`, the ID of the last row added.

<NodeInfoExplorer type="action_turso_write" />
