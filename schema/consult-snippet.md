# Consult snippet

Add a pointer like this to your coding agent's standing instructions (e.g. a
top-level `CLAUDE.md` / `AGENTS.md`) so the agent consults the catalog proactively
instead of assuming a capability doesn't exist. Replace `<catalog URL>` with wherever
you serve the catalog.

> **Ecosystem discovery** → before substantial cross-cutting work in any repo,
> fetch the Mycelium catalog at `<catalog URL>/CATALOG.md`. It maps every repo and
> service capability and when each applies — consult it instead of assuming a
> capability doesn't exist. Regenerate with `myco build`.
