---
type: reference
links:
  - rel: see-also
    to: schema/graph.md
---

# Consult snippet

Add a pointer like this to your coding agent's standing instructions (e.g. a
top-level `CLAUDE.md` / `AGENTS.md`) so the agent consults the ecosystem graph
proactively instead of assuming a capability doesn't exist. Replace `<hub URL>` with
wherever you serve the artifacts. Both files are for the agent, not for humans — the
snippet names which to reach for when.

> **Ecosystem discovery** → before substantial cross-cutting work in any repo,
> consult the Mycelium ecosystem graph instead of assuming a capability doesn't exist.
> **Read `<hub URL>/MAP.md` into context to orient** — a skimmable map of every repo
> and service capability and when each applies. When you need a specific field,
> endpoint, or to filter/traverse, **query `<hub URL>/graph.json`** (the full-fidelity
> graph, every field). **Prefer the query interface over hand-written `jq`:** `GET
> <hub URL>/q` lists every query; then e.g. `GET <hub URL>/q/capability/<name>` for a
> capability's summary + providers, `GET <hub URL>/q/components?kind=app&stack=rust` to
> filter, `GET <hub URL>/q/used-by/<name>` for blast radius, `GET <hub URL>/q/search?q=`
> to find. Each returns already-filtered JSON — no path to construct, and an unknown
> name is a 404, not a silent empty result. **Fallback** (only for what no named query
> covers): `jq` over `graph.json`, whose components are flat —
> `jq -r '.components[].provides[]? | "\(.name): \(.summary)"' graph.json`.
> For a specific repo's own **documentation graph** —
> which docs it has, how they link, whether any are unfindable — each `graph.json`
> component carries a compact `docGraph` digest; that's enough to gauge a repo's docs
> without a second fetch. To walk the **full** doc-graph, **follow the digest's `url`**
> (a link the hub stamps in, e.g. `docGraph.url` = `/repos/github.com/acme/web/docgraph.json`):
> `curl "<hub URL>$(jq -r '.components[]|select(.name=="<repo>").docGraph.url' graph.json)"`.
> Regenerate with `myco build`.
