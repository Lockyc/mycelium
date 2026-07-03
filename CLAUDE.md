# CLAUDE.md — Mycelium

`myco` is a Go CLI: scan repo roots for `catalog.toml` sidecars → merge with a
private overlay → render `CATALOG.md`/`catalog.json` → audit → serve.

## Layout
- `cmd/myco` — CLI + subcommand dispatch.
- `internal/catalog` — model, identity (canonical git-URL dedup), merge, render.
- `internal/scan` — node role: walk roots, read sidecars + git info → manifest.
- `internal/audit` — orphan / dangling-edge / staleness / schema checks.
- `internal/serve` — HTTP handler for the catalog dir.

## Invariants
- Components dedupe by **canonical git-remote URL**, never by path.
- Sidecars are **public-safe**; relationship edges + private nodes live in the
  overlay only.
- Repo roots are always configurable — never hardcode a path.
- Only third-party dep: `go-toml/v2`. Git via `os/exec`, JSON/HTTP via stdlib.

## Test / build
    go test ./...
    go build -o myco ./cmd/myco
