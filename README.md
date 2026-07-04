# Mycelium

[![CI](https://github.com/Lockyc/mycelium/actions/workflows/ci.yml/badge.svg)](https://github.com/Lockyc/mycelium/actions/workflows/ci.yml)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)
[![License](https://img.shields.io/github/license/Lockyc/mycelium)](LICENSE)

`myco` reads per-repo `catalog.toml` metadata across a set of repo roots, merges
it with a private relationship overlay into one agent-readable catalog
(`CATALOG.md` + `catalog.json`), audits that catalog for rot, and serves it over
HTTP. It gives a coding agent a proactive map of an ecosystem — which repos and
services exist and when each applies — instead of relying on a human to point.

**Status:** early WIP. v1.1 implements a distributed architecture: **nodes** scan
repo roots and push manifests to a central **hub**, which ingests and rebuilds the
catalog, then serves it over HTTP. The node→hub→serve path runs in a private
reference deployment (a scheduled node behind an auth-gated hub); the catalog is
only as rich as the `catalog.toml` sidecars committed across the scanned repos.

## Build

    go build -o myco ./cmd/myco
    myco version          # or --version, -v

## Contributing

Work lands on the `dev` integration branch; `main` is the release branch and only
fast-forwards to a tagged release. Branch feature/fix work off `dev`, run `just gate`
(gofmt + vet + tests) before merging, and merge back to `dev`. Releases follow
[semantic versioning](https://semver.org): the root `VERSION` file is the single source
of truth (embedded into the binary), and `just release` tags `v<VERSION>` and publishes
the matching GitHub release.

## Use

### Scan and collect metadata

    myco scan --roots <dir>[,<dir>] --node <id> --out manifest.json

Walks the repo roots, reads each committed `catalog.toml` sidecar, gathers git
metadata (origin remote, tags), and writes a manifest (JSON). The `--node` id
tags this manifest; used by a hub to track which node pushed it.

### Node (scan where the repos live, push to a hub)

    myco scan --roots <repo-store> --node <id> \
      --source local-checkout --exclude-owners vendor --fallback-host <host> \
      --push https://<hub> --token-file /path/to/token

Reads each repo's committed `catalog.toml` (bare repos and working trees alike),
skips denied owners, and POSTs the manifest to the hub.

### Hub (ingest manifests, rebuild, serve)

    myco serve --manifests <dir> --overlay overlay.toml \
      --catalog ./catalog --ingest-token-file /path/to/token --addr :8080

Serves `/CATALOG.md` and `/catalog.json`, and accepts `POST /manifests`
(node-keyed, bearer-authenticated); each push rebuilds the served catalog.
`--ingest-token-file` is optional — omit it only behind a trusted network
boundary; the hub then logs a loud warning that ingest is unauthenticated.

### Legacy: single-node build and audit

    myco build --manifests <dir> --overlay overlay.toml --out ./catalog
    myco audit --catalog ./catalog

## Demo

See a populated catalog built from the bundled examples — no real repos needed:

    ./scripts/demo.sh

It materialises the bundled [`examples/`](examples/README.md) into throwaway git
repos in a temp dir and runs the full scan → build → audit pipeline.

## Reference

- [`schema/catalog.md`](schema/catalog.md) — the `catalog.toml` sidecar and
  `overlay.toml` schema (fields, node/edge shapes, public-safe vs private split).
- [`schema/consult-snippet.md`](schema/consult-snippet.md) — the standing-instruction
  snippet to drop into a consuming agent's `CLAUDE.md`/`AGENTS.md` so it consults the
  served catalog proactively.
