# rook

A code intelligence tool that ingests SCIP indexes, stores them in Postgres, and answers questions about symbols over a CLI, GraphQL, and MCP.

Status: scaffolding. See [PLAN.md](./PLAN.md) for the build plan and phase-by-phase exit criteria.

## What it will do

- `rook index <path>` runs scip-python or scip-typescript against a repo and loads the index into Postgres.
- `rook lookup <name>` returns the definition, signature, and call count for a symbol.
- `rook refs <name>` lists every reference site.
- `rook callers <name>` and `rook callees <name>` walk the call graph.
- `rook explain <symbol>` asks Claude for a grounded plain-English walkthrough.
- `rook serve` exposes the same queries over GraphQL.
- `rook mcp` registers as an MCP server so Claude Code can call into it.

None of the above is shipped yet. Check the phase checkboxes in PLAN.md for what's actually done.

## Local development

```
make docker-up    # postgres + redis
make migrate      # run goose migrations
make build        # produces ./rook
make test
```

Requires Go 1.24+, Docker, and (for indexing) the `scip-python` and `scip-typescript` binaries on PATH.
