# CLAUDE.md — Mycelium

`myco` is a Go CLI: scan repo roots for `mycelium.toml` sidecars → merge with a
private overlay → render `CATALOG.md`/`catalog.json` → audit → serve.

## Layout
- `cmd/myco` — CLI + subcommand dispatch.
- `internal/catalog` — model, identity (canonical git-URL dedup), merge, render.
- `internal/scan` — node role: walk roots, read sidecars + git info → manifest.
- `internal/audit` — orphan / dangling-edge / staleness checks over `catalog.json`
  (schema validation is a separate step — `myco validate` / `ParseSidecar` at scan).
- `internal/serve` — HTTP handler for the catalog dir.
- `internal/transport` — node push (POST manifest to hub), hub ingest (receive + validate).
- `internal/hub` — hub role: Build (merge manifests → catalog) and Serve (HTTP + ingest endpoint).

## Invariants
- **Two outputs, two agent use cases — both for agents, neither for humans.** `CATALOG.md`
  (`RenderMarkdown`) is the deliberately **lossy** Markdown *map to read into context* — orient
  before cross-cutting work; it carries capability *names* per entry but omits their
  summaries/urls to stay skimmable. `catalog.json` (`RenderJSON`) is the **full-fidelity graph
  to query** — every field, for `jq`/filter/traverse. Read to orient, query to extract. Not
  human-vs-machine — both are agent-facing, split by task. `schema/catalog.md` is the contract
  for both; reconcile it with any render change.
- **`CATALOG.md` is component-first, and has no capability index.** Each entry states what a
  thing is, what it provides, who uses it, and what it is built with — together. A
  capability-first index was tried and dropped: it was a near-bijection (all but a couple of
  capabilities have exactly one provider), so it spent a line per capability restating a
  component name while saying nothing about the component it named — `git-mirror — homelab`
  forces a jump to homelab's entry to learn what homelab is. Its sort order bought nothing for
  a file that is read *whole, into context*; lookup is `catalog.json`'s job. **Only
  multi-provider capabilities get a callout** (`## Shared capabilities`) — overlap is the one
  fact this layout hides, and listing sole-provider capabilities there just rebuilds the index.
- **`Used by` reverses only the use edges** (`useEdgeTypes` — `consumes`/`depends-on`/
  `deploys-to`), so the line means blast radius. **Footgun:** do not reverse `markets`/`sells`/
  `related` into it — "business *sells* reductable" would render as reductable being *used by*
  business, which is false. Thematic edges belong to `Relationships`, which keeps their type.
- **Size is not the constraint — signal is.** The whole map is ~6 KB (~1.6k tokens); shaving
  lines buys nothing measurable, so judge a field by whether it answers a question the summary
  can't. That is why `stack` is rendered (20/20 populated, non-derivable) and each capability's
  `summary` is not (32/32 populated, but ~3x the file).
- **Overlay nodes are entries, not just capability providers.** A `[[node]]` (a non-repo
  actor — managed service, SaaS dep) rides in `Catalog.Nodes` and renders in the same
  name-sorted list as components. **Footgun:** `Merge` feeds nodes into the capability index,
  but they are *not* in `Components` — so any renderer that walks only `Components` drops them
  from the map silently, which is exactly what removing the capability index would have done.
- Components dedupe by **canonical git-remote URL**, never by path.
- **A sidecar inherits its repo's visibility.** It's committed to its repo, so a
  *public/shared* repo's sidecar must be public-safe (world-readable → no private
  info); a *private* repo's sidecar may carry internal detail (it's only as exposed
  as the repo, and the served catalog is access-gated). Relationship edges + non-repo
  private nodes live in the overlay regardless.
- Repo roots are always configurable — never hardcode a path.
- Only third-party dep: `go-toml/v2`. Git via `os/exec`, JSON/HTTP via stdlib.
- **Sidecars are read from a committed ref, default HEAD** — only committed `mycelium.toml`
  is seen; working-tree edits are not scanned. A node may pass `--ref <branch>` (e.g. `dev`)
  to read the active trunk instead; it falls back to HEAD per-repo when the branch is absent,
  so a mixed fleet (some repos `main`-only, some `main`+`dev`) needs no per-repo config.
- **Nodes are read-only on repos** — a node may not write to the git store it scans; it only
  reads and pushes manifests to the hub.
- **Hub stores manifests keyed by node id** — re-pushing from the same node replaces its
  prior manifest; different nodes' manifests are merged into the same catalog.
- **Orphans persist through the manifest into the catalog** — a scanned repo with no
  committed `mycelium.toml` rides in the manifest as an `Orphan` and is merged into
  `catalog.json`, so `myco audit` reports it fleet-wide (not just as a scan-time warning).
  A repo that is a component on any node is never an orphan. The overlay `ignore` list
  (canonical ids) suppresses repos that intentionally lack a sidecar — orphan curation
  lives in the overlay, the same private surface as edges and nodes.

## Test / build
    go test ./...
    go build -o myco ./cmd/myco

## Regenerating the served catalog
After adding or editing a `mycelium.toml` sidecar (in any scanned repo), changing the
overlay, or shipping a `myco` change, **regenerate the served catalog** rather than
waiting for the scheduled scan — otherwise the live catalog lags the sources. A node
scan (`myco scan --push`) rebuilds the hub on receipt. Deployment mechanics (how the
reference deployment triggers a rescan, and the mirror-lag caveat for repos read via a
pull-mirror) are deployment-specific and live with that private deployment's docs.

## Branching & versioning
- **`main` + `dev`.** `dev` is the integration trunk — all work flows through it. `main` is
  the release branch **and the public face**: it carries the latest tagged release **plus
  documentation-only updates**, and stays a clean ancestor of `dev` at rest. **Never commit
  *code* directly to `main`** — code flows `dev`→release only; a code commit on `main` drifts
  it ahead and breaks the next fast-forward (fix by back-merging `main` into `dev`, never
  force-push).
- **Docs land on `main` without a release.** A change touching *only* documentation (README,
  `docs/`, this `CLAUDE.md`, code comments — no code, build, `VERSION`, or behaviour) may be
  committed straight onto `main`, then **immediately forward-merged into `dev`**
  (`git checkout dev && git merge main`) so `dev` stays ⊇ `main` and the ancestor invariant
  holds. No version bump, no tag, no release. **Footgun:** a doc commit left on `main`
  un-merged into `dev` is dropped at the next release (which fast-forwards `main` to `dev`,
  which lacks it) — the forward-merge into `dev` is mandatory and immediate.
- Feature / fix / scratch branches off `dev`, merge back to `dev`. Short descriptive names.
- **Semver, `v`-prefixed tags.** The tracked root **`VERSION`** file is the single source of
  truth, `go:embed`-ed via `version.go` (root `mycelium` package) so `myco version`
  self-reports — never restate the version elsewhere.
- **Cut a release** from `dev` with `VERSION` bumped and committed: `just release`
  runs `gate`, fast-forwards `main`, tags `v<VERSION>`, and publishes the GitHub release.
  Plain Go CLI — no signing/updater.
