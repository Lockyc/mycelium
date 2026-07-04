# CLAUDE.md — Mycelium

`myco` is a Go CLI: scan repo roots for `catalog.toml` sidecars → merge with a
private overlay → render `CATALOG.md`/`catalog.json` → audit → serve.

## Layout
- `cmd/myco` — CLI + subcommand dispatch.
- `internal/catalog` — model, identity (canonical git-URL dedup), merge, render.
- `internal/scan` — node role: walk roots, read sidecars + git info → manifest.
- `internal/audit` — orphan / dangling-edge / staleness / schema checks.
- `internal/serve` — HTTP handler for the catalog dir.
- `internal/transport` — node push (POST manifest to hub), hub ingest (receive + validate).
- `internal/hub` — hub role: Build (merge manifests → catalog) and Serve (HTTP + ingest endpoint).

## Invariants
- Components dedupe by **canonical git-remote URL**, never by path.
- Sidecars are **public-safe**; relationship edges + private nodes live in the
  overlay only.
- Repo roots are always configurable — never hardcode a path.
- Only third-party dep: `go-toml/v2`. Git via `os/exec`, JSON/HTTP via stdlib.
- **Sidecars are read from a committed ref, default HEAD** — only committed `catalog.toml`
  is seen; working-tree edits are not scanned. A node may pass `--ref <branch>` (e.g. `dev`)
  to read the active trunk instead; it falls back to HEAD per-repo when the branch is absent,
  so a mixed fleet (some repos `main`-only, some `main`+`dev`) needs no per-repo config.
- **Nodes are read-only on repos** — a node may not write to the git store it scans; it only
  reads and pushes manifests to the hub.
- **Hub stores manifests keyed by node id** — re-pushing from the same node replaces its
  prior manifest; different nodes' manifests are merged into the same catalog.

## Test / build
    go test ./...
    go build -o myco ./cmd/myco

## Branching & versioning
- **`main` + `dev`.** `dev` is the integration trunk — all work flows through it. `main`
  is the release branch: it only fast-forwards to a tagged release commit and stays a
  clean ancestor of `dev`. **Never commit directly to `main`** — a direct commit drifts it
  ahead and breaks the fast-forward; fix by back-merging `main` into `dev`, never force-push.
- Feature / fix / scratch branches off `dev`, merge back to `dev`. Short descriptive names.
- **Semver, `v`-prefixed tags.** The tracked root **`VERSION`** file is the single source of
  truth, `go:embed`-ed via `version.go` (root `mycelium` package) so `myco version`
  self-reports — never restate the version elsewhere.
- **Cut a release** from `dev` with `VERSION` bumped and committed: `just release`
  runs `gate`, fast-forwards `main`, tags `v<VERSION>`, and publishes the GitHub release.
  Plain Go CLI — no signing/updater.
