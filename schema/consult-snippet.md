# Consult snippet

Add a pointer like this to your coding agent's standing instructions (e.g. a
top-level `CLAUDE.md` / `AGENTS.md`) so the agent consults the catalog proactively
instead of assuming a capability doesn't exist. Replace `<catalog URL>` with wherever
you serve the catalog. Both catalog files are for the agent, not for humans — the
snippet names which to reach for when.

> **Ecosystem discovery** → before substantial cross-cutting work in any repo,
> consult the Mycelium catalog instead of assuming a capability doesn't exist.
> **Read `<catalog URL>/CATALOG.md` into context to orient** — a skimmable map of
> every repo and service capability and when each applies. When you need a specific
> field, endpoint, or to filter/traverse, **query `<catalog URL>/catalog.json`** (the
> full-fidelity graph, every field) with `jq`. Regenerate with `myco build`.
