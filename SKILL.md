---
name: stacklit
description: Use the stacklit codebase map (stacklit.json) to locate code and check blast radius before grep or glob.
---

Read `stacklit.json` before broad searches.

1. Missing or stale: `stacklit generate`.
2. Find the module by `purpose`, then read only its `file_list`.
3. Before editing, check `depended_by` and `dependencies.most_depended`.
4. New code goes where `hints.add_feature` says; verify with `hints.test_command`.
5. Respect `hints.do_not_touch`.

MCP tools, if connected: `get_overview`, `find_module`, `get_module`, `get_dependencies`, `list_modules`, `get_hot_files`, `get_hints`.

Docs, APIs, schemas, owners: `stacklit ingest`, then read `stacklit.context.json`.
Index drift: `stacklit diff`.
