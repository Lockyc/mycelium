# Catalog Schema

The Mycelium catalog documents the ecosystem of repositories, services, and capabilities. It consists of two layers: a **public-safe sidecar** (`catalog.toml`) present in each repository, and a **private overlay** (`overlay.toml`), kept out of public repos and maintained privately by the catalog operator, that adds internal relationships and infrastructure nodes.

## Sidecar: catalog.toml

The public sidecar documents a single repository or service. One file per repo; safe to commit and share.

### Fields

- **`name`** (required, string): Short identifier for the component (e.g., `"orders-api"`, `"billing-web"`). Used as the canonical key in the catalog.
- **`summary`** (required, string): One-line human-readable description of purpose or value.
- **`kind`** (optional, string): Component category. Valid values: `service`, `app`, `library`, `docs`, `infra`, `tool`.
- **`status`** (optional, enum): Lifecycle stage. Valid values: `active`, `wip`, `experimental`, `archived`. Indicates maintenance level and stability.
- **`tags`** (optional, array of strings): Searchable labels (e.g., `["orders", "billing"]`, `["cli", "go"]`). Used to group related components.
- **`stack`** (optional, array of strings): Technologies and runtimes (e.g., `["go", "postgres"]`, `["typescript", "react"]`). Describes what powers the component.

### Repeatable Blocks

#### `[[provides]]`

Each block documents a discrete capability or service exported by the component. Public-safe only; internal capabilities live in the overlay.

- **`name`** (required, string): Capability identifier (e.g., `"order-events"`, `"cache"`).
- **`summary`** (required, string): What the capability does.
- **`url`** (optional, string): Public-safe endpoint or documentation URL. Omit if the capability is internal; move it to the overlay instead.

### Example: Public-Safe Sidecar

```toml
# catalog.toml — public-safe
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

The private overlay is maintained privately by the catalog operator (never committed to a public repo). It extends the catalog with internal-only nodes (infrastructure, internal services, non-repo capabilities) and relationship edges (how components consume, depend on, and relate to each other).

### Fields

- **`ignore`** (optional, array of strings): Canonical repo ids for repos that intentionally lack a `catalog.toml` and should be suppressed from the audit's orphan list. Match by the id printed by `myco audit` (e.g. `"github.com/acme/scratch"`); a full remote URL is accepted too and is canonicalized. Suppression applies only to orphans — a repo that has a sidecar is always a component regardless of this list.

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
- **`type`** (required, enum): Relationship type. Valid values: `consumes`, `depends-on`, `deploys-to`, `related`.
  - `consumes`: `from` uses a capability or service provided by `to`.
  - `depends-on`: `from` has a build or hard dependency on `to`.
  - `deploys-to`: `from` is deployed or runs on `to`.
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
read each committed `catalog.toml`, and emit a manifest. The **hub** (`myco build`
or `myco serve`) then:

1. Loads all node manifests from the manifests dir.
2. Merges the manifests' components and the overlay into a unified graph.
3. Renders the catalog as `CATALOG.md` (human-readable) and `catalog.json` (machine-readable).

Consistency checks (orphans, dangling edges, staleness) are a separate step,
`myco audit`, run against the rendered `catalog.json`. Orphans — repos a node
scanned that carry no committed `catalog.toml` — ride in each manifest and are
merged into the catalog, so the audit reports them fleet-wide (minus any id in the
overlay `ignore` list) rather than only at scan time.

The overlap between sidecar and overlay (when a repo has an entry in both) is valid: the sidecar documents the public face; the overlay can add internal edges, private capabilities, or infrastructure relationships.
