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
- **`graph.json`** — the **complete, queryable graph**. Every field (all `provides`, `stack`, `url`s, and edges), lossless. Reach for `myco query` (below) rather than hand-writing `jq` — it needs no knowledge of this shape. Query the raw JSON directly only for a specific field, endpoint, or traversal no named query covers — not to skim. (`RenderJSON` marshals the whole struct.) Capability summaries/urls, which `MAP.md` drops, are best fetched with `myco query capability <name>` (or `GET <hub>/q/capability/<name>`). The raw-`jq` equivalent, for the programmatic case, is `jq -r '.components[].provides[]? | "\(.name): \(.summary)"' graph.json`. `MAP.md`'s preamble carries this same pointer so an agent reading only the map still learns the summaries are recoverable.

Rule of thumb: **read `MAP.md` to orient, query `graph.json` to extract** — the filenames say it.

### `graph.json` shape (know this before writing a `jq` query)

Top-level keys: `components`, `capabilities`, `edges`, `dangling_edges`, `orphans`. A component
is **flat** — its `mycelium.toml` fields sit at the top level alongside the derived
`id`/`commit`/`docGraph`, with no wrapper. So a component serialises as:

```jsonc
.components[] = {
  "id":       "github.com/acme/orders-api",  // derived (canonical git URL)
  "name":     "orders-api",                  // derived (== the sidecar name)
  "commit":   "abc123…",                     // derived (scanned ref)
  "summary":  "…", "kind": "service", "status": "active",   // declared in mycelium.toml
  "tags":     ["…"], "stack": ["…"],                        // declared
  "provides": [ { "name": "…", "summary": "…", "url": "…" } ],  // declared
  "docGraph": { "url": "/repos/<id>/docgraph.json", "schemaVersion": 1, … }  // derived (node capture)
}
```

Every field is one segment off the component: `.components[].summary`, `.components[].stack[]`,
`.components[].provides[]`, `.components[].docGraph.url`. `.capabilities` is a separate
`name → [provider names]` index (built from every component's *and* overlay node's provides),
so it answers "who provides X" without walking components. Overlay nodes render under `.nodes`
with the same flat shape (name/summary/provides), minus the fields a non-repo has no notion of.
(The Go model nests the declared fields in a `Sidecar` struct for internal clarity, but the
JSON is flattened at the boundary — a consumer never sees a `sidecar` key.)
Tip: while exploring, drop the `?` — `.components[].provides[]` (wrong path) errors loudly
with *Cannot iterate over null*, whereas `[]?` returns silence whether the path is empty or
wrong; add `?` back once the path is confirmed.

## Querying

**`myco query` is the first-class way to explore and discover the graph** — the one to
reach for. It answers named questions *without your needing to know `graph.json`'s
structure*, and an unknown name is an explicit error, never the silent empty result a
wrong `jq` path gives. One implementation (`internal/query`) sits behind two surfaces:

- **CLI — `myco query` (reach for this):** `myco query` alone lists the queries;
  `myco query <name> [args]` runs one — `capabilities`, `capability <name>`,
  `component <name>`, `components --kind=… --stack=…`, `used-by <name>`, `uses <name>`,
  `search <text>`. Text by default, `--json` to pipe. It reads a local `graph.json`
  (`--dir`, default `.`) or a hub (`--url <hub>`, default `$MYCELIUM_HUB` — set that
  once and no flag is needed). Flags precede the positional:
  `myco query used-by --url <hub> config-core`.
- **HTTP `/q/*` — a bonus, when the binary isn't around:** the same queries by `curl`.
  `GET <hub>/q` lists them; e.g. `GET <hub>/q/capability/monitoring`,
  `GET <hub>/q/components?kind=app&stack=rust`, `GET <hub>/q/used-by/config-core`.

Hand-written `jq` over the raw `graph.json` is for **specific/programmatic** cases only —
something no named query expresses, where you accept learning the schema (below) *and*
the silent-empty-on-wrong-path footgun `myco query` removes. It is not the discovery path.

### Per-repo doc-graph (`docGraph`)

Each component in `graph.json` may carry a **`docGraph`** digest — the node's
capture of that repo's [docgraph](https://github.com/lockyc/docgraph) doc-graph,
taken during scan (the node is the only role with local git access). It is derived,
not declared: nothing in `mycelium.toml` sets it.

- **Digest fields** (all camelCase): `schemaVersion`, `docCount`,
  `contentEdgeCount`, `metadataEdgeCount`, `contentIslands` (unfindable docs — the
  rot signal), `metadataIslands` (docs with no declared placement), `entryDocs`
  (which of `CLAUDE.md`/`README.md`/`docs/index.md` are present), and **`url`** — a
  self-navigating link to this repo's full doc-graph payload (see Full payload),
  stamped by the hub so a consumer *follows the link* rather than reconstructing the
  route from the id.
- **schemaVersion is pinned to 1.** A payload with any other version is
  recorded-but-not-interpreted: the digest carries only the observed
  `schemaVersion`, and `myco audit` reports a `docgraph-version` finding.
- **Omitted when there's nothing to say:** a repo with no markdown, or a node
  without docgraph on PATH, carries no `docGraph`. Bare repos are captured like
  any other (see Ref consistency below).
- **`MAP.md`** surfaces this only as a rot flag — a component with ≥1 island gets a
  one-line `docs: N islands ⚠` marker; a clean doc-graph adds nothing.
- **Full payload:** the node also stashes each repo's complete `docgraph graph
  --json` out-of-band; the hub serves it at the digest's **`url`** —
  **`GET /repos/<id>/docgraph.json`** (`<id>` is the canonical id, e.g.
  `/repos/github.com/lockyc/mycelium/docgraph.json`). Follow `docGraph.url`; don't
  rebuild it. It is never inlined into `graph.json` — the digest answers "healthy /
  how big", the payload is for traversal. The route has one source
  (`graph.RepoDocGraphRoute` / the `RepoDocGraph*` constants): the hub stamps the
  `url`, writes the payload, and the serve handler parses the same prefix/suffix.

**Ref consistency:** the node reads the doc-graph with `docgraph graph --ref
<ref>`, at the **same committed ref** it scanned the sidecar from. So a
component's `docGraph` reflects exactly the scanned ref — not the node's
working-tree state — on every repo, bare or not. (This resolves the
working-tree caveat of the initial release, which read whatever was checked
out.) Requires **docgraph v3.1.0+** on the node (the first version with
`--ref`); an older or absent binary omits the digest rather than failing the
scan.

Consistency checks (orphans, dangling edges, staleness, doc-rot, docgraph-version)
are a separate step, `myco audit`, run against the rendered `graph.json`. Orphans — repos a node
scanned that carry no committed `mycelium.toml` — ride in each manifest and are
merged into the graph, so the audit reports them fleet-wide (minus any id in the
overlay `ignore` list) rather than only at scan time.

The overlap between sidecar and overlay (when a repo has an entry in both) is valid: the sidecar documents the public face; the overlay can add internal edges, private capabilities, or infrastructure relationships.
