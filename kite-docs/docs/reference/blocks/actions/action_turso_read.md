---
sidebar_position: 37.9
---

import EmbedFlowNode from "../../../../src/components/EmbedFlowNode";
import NodeInfoExplorer from "../../../../src/components/NodeInfoExplorer";

# Read rows

<EmbedFlowNode type="action_turso_read" />

The `Read rows` block runs a `SELECT` query in your own [Turso](https://turso.tech) database and returns the rows, for things like a points leaderboard or a member's saved settings. Enable Turso under [Integrations](../../integrations.md) first, and enter your database URL and auth token there.

Put a `?` in the query wherever a value goes, and the values in _Values_, one per line in the same order. For example, the query `SELECT points FROM users WHERE user_id = ?` with `{{ user.id }}` as the value. Placeholders don't work in the query itself, so what members type can't change what the query does.

The result has `rows`, a list with one entry per row by column name, and `first`, the first row. Use `found` to check whether the query found anything before you read `first`, like `{{ result('turso_x').first.points }}`. At most 100 rows are returned, and `truncated` tells you if there were more.

A token that can only read, made with `turso db tokens create my-db --read-only`, keeps this block from ever changing your data.

<NodeInfoExplorer type="action_turso_read" />
