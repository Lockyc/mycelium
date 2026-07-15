# Mycelium

[![CI](https://github.com/Lockyc/mycelium/actions/workflows/ci.yml/badge.svg)](https://github.com/Lockyc/mycelium/actions/workflows/ci.yml)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/github/go-mod/go-version/Lockyc/mycelium?logo=go&logoColor=white)
[![License](https://img.shields.io/github/license/Lockyc/mycelium)](LICENSE)

`myco` reads per-repo `mycelium.toml` metadata across a set of repo roots, merges
it with a private relationship overlay into one agent-readable catalog
(`CATALOG.md` + `catalog.json`), audits that catalog for rot, and serves it over
HTTP. It gives a coding agent a proactive map of an ecosystem — which repos and
services exist and when each applies — instead of relying on a human to point.

Both outputs are **for agents, not humans**, and serve two use cases: **read
`CATALOG.md` into context to orient** (a lossy, skimmable Markdown digest), and
**query `catalog.json` to extract** (the full-fidelity graph — every field — for
`jq`/filter/traverse). See [`schema/catalog.md`](schema/catalog.md#two-outputs-two-agent-use-cases).

**Status:** early WIP. The architecture is distributed: **nodes** scan repo roots
and push manifests to a central **hub**, which ingests and rebuilds the catalog,
then serves it over HTTP. The node→hub→serve path runs in a private
reference deployment (a scheduled node behind an auth-gated hub); the catalog is
only as rich as the `mycelium.toml` sidecars committed across the scanned repos.

## Build

    go build -o myco ./cmd/myco
    myco version          # or --version, -v

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
      --catalog ./catalog --ingest-token-file /path/to/token --addr :8080

Serves `/CATALOG.md` and `/catalog.json`, and accepts `POST /manifests`
(node-keyed, bearer-authenticated); each push rebuilds the served catalog.
`--ingest-token-file` is optional — omit it only behind a trusted network
boundary; the hub then logs a loud warning that ingest is unauthenticated.

### Build and audit locally (single-node)

    myco build --manifests <dir> --overlay overlay.toml --out ./catalog
    myco audit --catalog ./catalog

`audit` reports catalog rot: **orphans** (scanned repos with no committed
`mycelium.toml`), dangling overlay edges, and components that vanished since the last
run. Repos that intentionally have no sidecar are suppressed via an `ignore` list of
canonical ids in the private `overlay.toml` (see [`schema/catalog.md`](schema/catalog.md)).

### Validate a sidecar

    myco validate <mycelium.toml>

Lints a single `mycelium.toml` against the schema — run it before committing a new
sidecar. Prints the parsed name and summary on success, or the schema error.

## Demo

See a populated catalog built from the bundled examples — no real repos needed:

    ./scripts/demo.sh

It materialises the bundled [`examples/`](examples/README.md) into throwaway git
repos in a temp dir and runs the full scan → build → audit pipeline.

## Reference

- [`schema/catalog.md`](schema/catalog.md) — the `mycelium.toml` sidecar and
  `overlay.toml` schema (fields, node/edge shapes, public-safe vs private split).
- [`schema/consult-snippet.md`](schema/consult-snippet.md) — the standing-instruction
  snippet to drop into a consuming agent's `CLAUDE.md`/`AGENTS.md` so it consults the
  served catalog proactively.

## Development

    just build   # go build -o myco ./cmd/myco
    just test    # go test ./...
    just gate    # gofmt check + vet + tests (pre-merge gate)
    just release # cut a release from dev

The branch model (`dev` trunk, `main` release branch) and release flow live in
[`CLAUDE.md`](CLAUDE.md).
