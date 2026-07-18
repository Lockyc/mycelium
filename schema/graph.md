---
type: reference
---

# Graph Schema

The Mycelium graph documents the ecosystem of repositories, services, and capabilities. It consists of two layers: a per-repo **sidecar** (`mycelium.toml`) committed in each repository, and a **private overlay** (`overlay.toml`), kept out of public repos and maintained privately by the graph operator, that adds internal relationships and infrastructure nodes.

## Sidecar: mycelium.toml

The sidecar documents a single repository or service (one file per repo). It is committed to its repo and so inherits that repo's visibility: in a **public or shared** repo it must be public-safe (world-readable → no private information); in a **private** repo it may include internal detail. Either way, cross-repo relationships and non-repo infrastructure belong in the overlay, not the sidecar.

### Fields

- **`name`** (required, string): Short identifier for the component (e.g., `"orders-api"`, `"billing-web"`). Used as the canonical key in the graph.
- **`summary`** (required, string): One-line human-readable description of purpose or value.
- **`kind`** (optional, string): Component category. Valid values: `service`, `app`, `library`, `docs`, `infra`, `tool`.
- **`status`** (optional, enum): Lifecycle stage. Valid values: `active`, `wip`, `experimental`, `archived`. Indicates maintenance level and stability.
- **`tags`** (optional, array of strings): Labels for what the component is *about* — domain, topic, lifecycle, target platform (e.g., `["orders", "billing"]`, `["prelaunch", "macos"]`). Used to group related components.
- **`stack`** (optional, array of strings): What the component is *built with* — languages, frameworks, runtimes, datastores, defining libraries (e.g., `["go", "postgres"]`, `["typescript", "react"]`).

`tags` and `stack` are disjoint by construction: **a technology name is never a tag.**
If a label names a language, framework, runtime, or library it belongs in `stack` —
`rust`, `tmux`, `astro`, and `cloudflare-workers` are all `stack`, never `tags`. The
test is the question each answers: *what is this about* (`tags`) versus *what is this
built with* (`stack`). Both render in `MAP.md`, so a technology listed in both
appears twice on the entry.

### Repeatable Blocks

#### `[[provides]]`

Each block documents a discrete capability or service exported by the component. In a public/shared repo keep these public-safe — internal-only capabilities live in the overlay instead; a private repo's sidecar may list internal capabilities directly.

- **`name`** (required, string): Capability identifier (e.g., `"order-events"`, `"cache"`).
- **`summary`** (required, string): What the capability does.
- **`url`** (optional, string): Endpoint or documentation URL. In a public/shared repo it must be public-safe — omit an internal endpoint (put it in the overlay); a private repo's sidecar may list an internal URL directly.

### Example: Public-Safe Sidecar

```toml
# mycelium.toml — public-safe
name    = "orders-api"
summary = "Order processing service"
kind    = "service"
status  = "active"
tags    = ["orders", "billing"]
stack   = ["go", "postgres"]

[[provides]]
name    = "order-events"
summary = "Publishes order lifecycle events on a message bus"
```

## Overlay: overlay.toml

The private overlay is maintained privately by the graph operator (never committed to a public repo). It extends the graph with internal-only nodes (infrastructure, internal services, non-repo capabilities) and relationship edges (how components consume, depend on, and relate to each other).

### Fields

- **`ignore`** (optional, array of strings): Canonical repo ids for repos that intentionally lack a `mycelium.toml` and should be suppressed from the audit's orphan list. Match by the id printed by `myco audit` (e.g. `"github.com/acme/scratch"`); a full remote URL is accepted too and is canonicalized. Suppression applies only to orphans — a repo that has a sidecar is always a component regardless of this list.

### Repeatable Blocks

#### `[[node]]`

Defines a capability or component not represented by a repository (e.g., infrastructure, internal services, abstract capabilities).

- **`name`** (required, string): Node identifier (e.g., `"shared-postgres"`, `"internal-vpn"`).
- **`summary`** (required, string): What the node is or provides.
- **`provides`** (optional, array of strings): List of capabilities exported by this node (e.g., `["postgres"]`, `["vpn", "dns"]`). These are logical capability names, not necessarily service endpoints.

#### `[[edge]]`

Defines a directed relationship between two components or nodes. An edge connects a source component/node to a target, stating the nature of the relationship.

- **`from`** (required, string): Source component or node name (e.g., `"web-frontend"`, `"billing-web"`).
- **`to`** (required, string): Target component or node name (e.g., `"postgres"`, `"shared-lib"`).
- **`type`** (required, enum): Relationship type. Valid values: `consumes`, `depends-on`, `deploys-to`, `markets`, `sells`, `related`.
  - `consumes`: `from` uses a capability or service provided by `to`.
  - `depends-on`: `from` has a build or hard dependency on `to`.
  - `deploys-to`: `from` is deployed or runs on `to`.
  - `markets`: `from` is a marketing or promotional property for `to`.
  - `sells`: `from` is the commercial vehicle that sells/commercialises `to`.
  - `related`: `from` is thematically or functionally related to `to`; no strict dependency.

### Example: Overlay

```toml
# overlay.toml — private (operator-maintained, never committed to a public repo)
ignore = ["github.com/acme/scratch"]  # repos that intentionally have no sidecar

[[node]]
name     = "shared-postgres"
summary  = "Shared Postgres instance"
provides = ["postgres"]

[[edge]]
from = "web-frontend"
to   = "postgres"
type = "consumes"
```

## Merging and Output

Sidecars are gathered by **nodes** (`myco scan`), which walk their repo roots,
read each committed `mycelium.toml`, and emit a manifest. The **hub** (`myco build`
or `myco serve`) then:

1. Loads all node manifests from the manifests dir.
2. Merges the manifests' components and the overlay into a unified graph.
3. Renders the graph in two forms — `MAP.md` and `graph.json` — for the two agent use cases below.

### Two outputs, two agent use cases

Both outputs are for coding agents, not humans. They are the *same graph* rendered two ways; an agent picks by task:

- **`MAP.md`** — the map to **read into context**. A compact Markdown digest that is intentionally **lossy**. It is **component-first**: one entry per component and overlay node, name-sorted, carrying summary, kind/status, the *names* of what it provides, who uses it, stack, and tags — then shared capabilities, relationships, and undocumented repos. It drops each capability's `summary` and `url` so it stays skimmable. Fetch and read it to orient before cross-cutting work: *"what exists, and when does each apply?"* (`RenderMarkdown`.)
  - **`Used by`** reverses only the *use* edges (`consumes`, `depends-on`, `deploys-to`), so the line is an entry's blast radius: change this and these must be re-pinned or rebuilt. Thematic edges (`markets`, `sells`, `related`) are excluded — reversing them would assert something false — and appear only under `Relationships`, with their type intact.
  - **`Shared capabilities`** lists only capabilities with more than one provider. A sole provider is already stated on its own entry; there is no full capability index, because one provider is the norm and an index of them restates component names while saying nothing about the components it names. Capability-first lookup is `graph.json`'s job.
- **`graph.json`** — the **complete, queryable graph**. Every field (all `provides`, `stack`, `url`s, and edges), lossless. Query it with `jq` (or a tool) when you need a specific field, an endpoint, or to filter/traverse — not to skim. (`RenderJSON` marshals the whole struct.)

Rule of thumb: **read `MAP.md` to orient, query `graph.json` to extract** — the filenames say it.

Consistency checks (orphans, dangling edges, staleness) are a separate step,
`myco audit`, run against the rendered `graph.json`. Orphans — repos a node
scanned that carry no committed `mycelium.toml` — ride in each manifest and are
merged into the graph, so the audit reports them fleet-wide (minus any id in the
overlay `ignore` list) rather than only at scan time.

The overlap between sidecar and overlay (when a repo has an entry in both) is valid: the sidecar documents the public face; the overlay can add internal edges, private capabilities, or infrastructure relationships.
