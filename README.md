# Mycelium

`myco` reads per-repo `catalog.toml` metadata across a set of repo roots, merges
it with a private relationship overlay into one agent-readable catalog
(`CATALOG.md` + `catalog.json`), audits that catalog for rot, and serves it over
HTTP. It gives a coding agent a proactive map of an ecosystem — which repos and
services exist and when each applies — instead of relying on a human to point.

**Status:** early WIP. v1 builds the CLI (`scan`/`build`/`serve`/`audit`/
`validate`) and runs as a single hub node.

## Build

    go build -o myco ./cmd/myco

## Use

    myco scan --roots <dir>[,<dir>] --node <id> --out manifest.json
    myco build --manifests <dir> --overlay overlay.toml --out ./catalog
    myco audit --catalog ./catalog
    myco serve --catalog ./catalog --addr :8080

## Demo

See a populated catalog built from the bundled examples — no real repos needed:

    ./scripts/demo.sh

It materialises [`examples/`](examples/) into throwaway git repos in a temp dir
and runs the full scan → build → audit pipeline.
