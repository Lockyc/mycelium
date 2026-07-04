# Mycelium

`myco` reads per-repo `catalog.toml` metadata across a set of repo roots, merges
it with a private relationship overlay into one agent-readable catalog
(`CATALOG.md` + `catalog.json`), audits that catalog for rot, and serves it over
HTTP. It gives a coding agent a proactive map of an ecosystem — which repos and
services exist and when each applies — instead of relying on a human to point.

**Status:** early WIP. v1.1 implements a distributed architecture: **nodes** scan
repo roots and push manifests to a central **hub**, which ingests and rebuilds the
catalog, then serves it over HTTP.

## Build

    go build -o myco ./cmd/myco

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

### Legacy: single-node build and audit

    myco build --manifests <dir> --overlay overlay.toml --out ./catalog
    myco audit --catalog ./catalog

## Demo

See a populated catalog built from the bundled examples — no real repos needed:

    ./scripts/demo.sh

It materialises [`examples/`](examples/) into throwaway git repos in a temp dir
and runs the full scan → build → audit pipeline.
