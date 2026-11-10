# Mycelium

[![Release](https://img.shields.io/github/v/release/lockyc/mycelium?sort=semver&label=release)](https://github.com/lockyc/mycelium/releases/latest)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/github/go-mod/go-version/lockyc/mycelium?logo=go&logoColor=white)
[![License](https://img.shields.io/github/license/lockyc/mycelium)](LICENSE)

A map of your repos and services that a coding agent reads before it starts work. Each repo
commits a small `mycelium.toml` saying what it is and what it provides. `myco` gathers them
across machines into one graph — a Markdown summary to read and JSON to query — and audits that
graph for broken links between repos and stale entries.

A sidecar:

```toml
name    = "billing-web"
summary = "Customer-facing billing dashboard"
kind    = "app"
status  = "active"
tags    = ["billing", "frontend"]
stack   = ["typescript", "react"]

[[provides]]
name    = "billing-ui"
summary = "Web UI for invoices and payment methods"
```

What an agent gets back:

```console
$ myco query component orders-api
orders-api — Order processing and lifecycle service
  service active
  stack: go, postgres
  provides order-events: Publishes order lifecycle events on the message bus

$ myco query used-by shared-lib
  billing-web (depends-on)
  orders-api (depends-on)
```

## Install

```bash
go install github.com/lockyc/mycelium/cmd/myco@latest
myco version
```

Needs Go and git. To see a populated graph built from the bundled [`examples/`](examples/README.md)
without any real repos, run `./scripts/demo.sh` from a clone: it builds `myco`, turns the
examples into throwaway git repos in a temp dir, and runs scan → build → audit.

## Usage

**On one machine:**

```bash
myco scan --roots ~/code --node laptop --out manifests/laptop.json
myco build --manifests manifests --overlay overlay.toml --dir ./graph
myco audit --dir ./graph
myco query capabilities --dir ./graph
```

- **`scan`** walks the repo roots, reads each repo's *committed* `mycelium.toml` (bare repos and
  working trees alike) plus its origin remote and commit, and writes a manifest. `--ref <branch>`
  reads the sidecar from that branch instead of HEAD. With
  [docgraph](https://github.com/lockyc/docgraph) v3.1.0+ on PATH, each repo's doc-graph digest
  comes along too.
- **`build`** merges the manifests with the **overlay** — a private `overlay.toml` holding what
  no single repo can say about itself: relationships between repos, shared services, and the
  repos deliberately left without a sidecar — into `MAP.md` and `graph.json`.
- **`audit`** reports graph rot: repos with no sidecar, sidecars that don't parse or use an
  unknown `kind` or `status`, overlay edges pointing at nothing, duplicate names, components that
  vanished since the last run, docs nothing links to, and a node that has stopped scanning
  (each finding: [`schema/graph.md`](schema/graph.md)).
- **`query`** answers named questions — `capabilities`, `capability <name>`, `component <name>`,
  `components --kind=… --stack=… --status=… --tag=…`, `used-by <name>`, `uses <name>`,
  `search <text>`; bare `myco query` lists them. Text by default, `--json` to pipe.
- **`validate <mycelium.toml>`** lints one sidecar before you commit it, including any
  misspelled key the scan would silently drop.

**Across machines:** each machine runs a **node** that scans and pushes to a central **hub**,
which rebuilds and serves the graph over HTTP.

```bash
# node
myco scan --roots <repo-store> --node <id> --ref dev \
  --push https://<hub> --token-file /path/to/token
# hub
myco serve --manifests <dir> --overlay overlay.toml --dir ./graph \
  --ingest-token-file /path/to/token --addr :8080
```

The hub serves `/MAP.md`, `/graph.json` and the same queries at `GET /q/*`, and accepts
bearer-authenticated `POST /manifests`; each push rebuilds the graph. Set `MYCELIUM_HUB` and
`query` and `audit` read the hub with no flag. Omit `--ingest-token-file` only behind a trusted
network boundary.

## Two outputs, both for agents

**Read `MAP.md` into context to orient** — a skimmable Markdown digest — and **query
`graph.json` to extract** — the full graph, every field, for `jq`. The schema for sidecars, the
overlay and the graph: [`schema/graph.md`](schema/graph.md). A snippet to drop into an agent's
`CLAUDE.md` or `AGENTS.md` so it consults the graph before cross-cutting work:
[`schema/consult-snippet.md`](schema/consult-snippet.md).

## Limits

The graph is only as rich as the sidecars committed across the scanned repos, and a sidecar is
read from git, so an uncommitted edit isn't seen.

## Status

Working, pre-1.0: the CLI flags and graph schema can still shift. What's next:
[docs/FOLLOWUPS.md](docs/FOLLOWUPS.md).

## Development

```bash
git clone https://github.com/lockyc/mycelium && cd mycelium
just build   # go build -o myco ./cmd/myco
just test    # go test ./...
just gate    # gofmt check + vet + tests
```

Branch model and release process: [CLAUDE.md](CLAUDE.md).

## License

MIT — see [LICENSE](LICENSE).
