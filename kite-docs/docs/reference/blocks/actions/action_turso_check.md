---
sidebar_position: 37.92
---

import EmbedFlowNode from "../../../../src/components/EmbedFlowNode";
import NodeInfoExplorer from "../../../../src/components/NodeInfoExplorer";

# Test connection

<EmbedFlowNode type="action_turso_check" />

The `Test connection` block checks that your [Turso](https://turso.tech) database answers, by asking it for the number 1. It fails if the database URL or auth token is wrong, so it's useful in a test command right after you enable Turso under [Integrations](../../integrations.md).

The result has `ok` and `latency_ms`, how long the database took to answer.

<NodeInfoExplorer type="action_turso_check" />
