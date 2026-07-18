# Mycelium

[![CI](https://github.com/Lockyc/mycelium/actions/workflows/ci.yml/badge.svg)](https://github.com/Lockyc/mycelium/actions/workflows/ci.yml)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/github/go-mod/go-version/Lockyc/mycelium?logo=go&logoColor=white)
[![License](https://img.shields.io/github/license/Lockyc/mycelium)](LICENSE)

`myco` reads per-repo `mycelium.toml` metadata across a set of repo roots, merges
it with a private relationship overlay into one agent-readable graph
(`MAP.md` + `graph.json`), audits that graph for rot, and serves it over
HTTP. It gives a coding agent a proactive map of an ecosystem — which repos and
services exist and when each applies — instead of relying on a human to point.

Both outputs are **for agents, not humans**, and serve two use cases: **read
`MAP.md` into context to orient** (a lossy, skimmable Markdown digest), and
**query `graph.json` to extract** (the full-fidelity graph — every field — for
`jq`/filter/traverse). See [`schema/graph.md`](schema/graph.md#two-outputs-two-agent-use-cases).

**Status:** working, pre-1.0 — the public surface (CLI flags, graph schema) can
still shift. The architecture is distributed: **nodes** scan repo roots and push
manifests to a central **hub**, which ingests and rebuilds the graph, then serves
it over HTTP. The node→hub→serve path runs in a private reference deployment (a
scheduled node behind an auth-gated hub); the graph is only as rich as the
`mycelium.toml` sidecars committed across the scanned repos.

## Build

    go install ./cmd/myco   # installs myco to the Go bin dir, on PATH
    myco version            # or --version, -v

Or build in place — the binary lands in the repo root, so invoke it as `./myco`:

    go build -o myco ./cmd/myco

## Use

### Scan and collect metadata

    myco scan --roots <dir>[,<dir>] --node <id> --out manifest.json

Walks the repo roots, reads each committed `mycelium.toml` sidecar, gathers git
metadata (origin remote, tags), and writes a manifest (JSON). The `--node` id
tags this manifest; used by a hub to track which node pushed it.

### Node (scan where the repos live, push to a hub)

    myco scan --roots <repo-store> --node <id> \
      --source local-checkout --exclude-owners vendor --fallback-host <host> \
      --ref dev --push https://<hub> --token-file /path/to/token

Reads each repo's committed `mycelium.toml` (bare repos and working trees alike),
skips denied owners, and POSTs the manifest to the hub. `--ref <branch>` reads
the sidecar from that branch (e.g. `dev`) instead of HEAD, falling back to HEAD
per-repo when the branch is absent; omit it to read each repo's default branch.

### Hub (ingest manifests, rebuild, serve)

    myco serve --manifests <dir> --overlay overlay.toml \
      --dir ./graph --ingest-token-file /path/to/token --addr :8080

Serves `/MAP.md` and `/graph.json`, `GET /repos/<id>/docgraph.json` (a
component's full doc-graph payload, when it has one), and accepts
`POST /manifests` (node-keyed, bearer-authenticated); each push rebuilds the
served graph. Each component in `graph.json` carries a compact `docGraph`
digest when the node captured one — see
[`schema/graph.md`](schema/graph.md#per-repo-doc-graph-docgraph). Capturing
these digests requires **`docgraph` v3.1.0+ on the node's PATH** (it reads the
graph at the scanned ref via `docgraph graph --ref`, which works on a bare
repo store too); with an older or absent `docgraph` the digests are simply
omitted (best-effort, never fatal).
`--ingest-token-file` is optional — omit it only behind a trusted network
boundary; the hub then logs a loud warning that ingest is unauthenticated.

### Build and audit locally (single-node)

    myco build --manifests <dir> --overlay overlay.toml --dir ./graph
    myco audit --dir ./graph

`audit` reports graph rot: **orphans** (scanned repos with no committed
`mycelium.toml`), dangling overlay edges, and components that vanished since the last
run — plus **doc-rot** (a component with island docs: unfindable or unplaced) and
**docgraph-version** (a component whose docgraph `schemaVersion` is newer than
Mycelium's pinned `1`). Repos that intentionally have no sidecar are suppressed via an
`ignore` list of canonical ids in the private `overlay.toml` (see
[`schema/graph.md`](schema/graph.md)).

### Validate a sidecar

    myco validate <mycelium.toml>

Lints a single `mycelium.toml` against the schema — run it before committing a new
sidecar. Prints the parsed name and summary on success, or the schema error.

### Query the graph

    myco query <name> [args] [--dir <dir> | --url <hub>] [--json]

Named queries (capabilities, capability, component, components, used-by, uses,
search) over a local `graph.json` (`--dir`) or a live hub (`--url`) — text by
default, `--json` to pipe. Bare `myco query` prints the index. The hub also
serves the same queries at `GET /q/*` (`GET /q` lists them). No jq required for
the common cases — see [`schema/graph.md`](schema/graph.md#querying-prefer-over-hand-written-jq).

## Demo

See a populated graph built from the bundled examples — no real repos needed:

    ./scripts/demo.sh

It materialises the bundled [`examples/`](examples/README.md) into throwaway git
repos in a temp dir and runs the full scan → build → audit pipeline.

## Reference

- [`schema/graph.md`](schema/graph.md) — the `mycelium.toml` sidecar and
  `overlay.toml` schema (fields, node/edge shapes, public-safe vs private split).
- [`schema/consult-snippet.md`](schema/consult-snippet.md) — the standing-instruction
  snippet to drop into a consuming agent's `CLAUDE.md`/`AGENTS.md` so it consults the
  served graph proactively.

## Development

    just build   # go build -o myco ./cmd/myco
    just install # go install ./cmd/myco (myco onto PATH)
    just test    # go test ./...
    just gate    # gofmt check + vet + tests (pre-merge gate)
    just release # cut a release from dev

The branch model (`dev` trunk, `main` release branch) and release flow live in
[`CLAUDE.md`](CLAUDE.md).
