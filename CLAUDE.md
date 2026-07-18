---
type: architecture
links:
  - rel: depends-on
    to: schema/graph.md
---

# CLAUDE.md — Mycelium

`myco` is a Go CLI: scan repo roots for `mycelium.toml` sidecars → merge with a
private overlay → render `MAP.md`/`graph.json` → audit → serve.

## Layout
- `cmd/myco` — CLI + subcommand dispatch.
- `internal/graph` — model, identity (canonical git-URL dedup), merge, render.
- `internal/scan` — node role: walk roots, read sidecars + git info → manifest;
  also captures each non-bare component's docgraph doc-graph (`docgraph.go`).
- `internal/audit` — orphan / dangling-edge / staleness / doc-rot / docgraph-version
  checks over `graph.json` (schema validation is a separate step — `myco validate` /
  `ParseSidecar` at scan).
- `internal/serve` — HTTP handler for the artifact dir; also serves per-repo full
  doc-graph payloads.
- `internal/transport` — node push (POST manifest to hub), hub ingest (receive + validate).
- `internal/hub` — hub role: Build (merge manifests → graph) and Serve (HTTP + ingest endpoint).

## Invariants
- **Two outputs, two agent use cases — both for agents, neither for humans.** `MAP.md`
  (`RenderMarkdown`) is the deliberately **lossy** Markdown *map to read into context* — orient
  before cross-cutting work; it carries capability *names* per entry but omits their
  summaries/urls to stay skimmable. `graph.json` (`RenderJSON`) is the **full-fidelity graph
  to query** — every field, for `jq`/filter/traverse. The filenames carry the rule: read the
  map, query the graph. Not human-vs-machine — both are agent-facing, split by task.
  `schema/graph.md` is the contract for both; reconcile it with any render change.
- **`MAP.md` is component-first, and has no capability index.** Each entry states what a
  thing is, what it provides, who uses it, and what it is built with — together. A
  capability-first index was tried and dropped: it was a near-bijection (all but a couple of
  capabilities have exactly one provider), so it spent a line per capability restating a
  component name while saying nothing about the component it named — `git-mirror — homelab`
  forces a jump to homelab's entry to learn what homelab is. Its sort order bought nothing for
  a file that is read *whole, into context*; lookup is `graph.json`'s job. **Only
  multi-provider capabilities get a callout** (`## Shared capabilities`) — overlap is the one
  fact this layout hides, and listing sole-provider capabilities there just rebuilds the index.
- **`Used by` reverses only the use edges** (`useEdgeTypes` — `consumes`/`depends-on`/
  `deploys-to`), so the line means blast radius. **Footgun:** do not reverse `markets`/`sells`/
  `related` into it — "business *sells* reductable" would render as reductable being *used by*
  business, which is false. Thematic edges belong to `Relationships`, which keeps their type.
- **Size is not the constraint — signal is.** Shaving lines buys nothing measurable, so judge a
  field by whether it answers a question the summary can't. That is why `stack` is rendered
  (non-derivable) and each capability's `summary` is not (it would multiply the file for a fact
  the entry already implies). *Point-in-time evidence, as of the v0.5.0 scan of the reference
  deployment — the fleet grows, so re-measure rather than trusting these:* the whole map was
  ~6 KB (~1.6k tokens), `stack` was populated 20/20, and `summary` 32/32 at ~3x the file.
- **Overlay nodes are entries, not just capability providers.** A `[[node]]` (a non-repo
  actor — managed service, SaaS dep) rides in `Graph.Nodes` and renders in the same
  name-sorted list as components. **Footgun:** `Merge` feeds nodes into the capability index,
  but they are *not* in `Components` — so any renderer that walks only `Components` drops them
  from the map silently, which is exactly what removing the capability index would have done.
- Components dedupe by **canonical git-remote URL**, never by path.
- **A sidecar inherits its repo's visibility.** It's committed to its repo, so a
  *public/shared* repo's sidecar must be public-safe (world-readable → no private
  info); a *private* repo's sidecar may carry internal detail (it's only as exposed
  as the repo, and the served graph is access-gated). Relationship edges + non-repo
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
  prior manifest; different nodes' manifests are merged into the same graph.
- **Orphans persist through the manifest into the graph** — a scanned repo with no
  committed `mycelium.toml` rides in the manifest as an `Orphan` and is merged into
  `graph.json`, so `myco audit` reports it fleet-wide (not just as a scan-time warning).
  A repo that is a component on any node is never an orphan. The overlay `ignore` list
  (canonical ids) suppresses repos that intentionally lack a sidecar — orphan curation
  lives in the overlay, the same private surface as edges and nodes.
- **Per-repo doc-graph is node-captured, best-effort, schemaVersion-1-pinned.**
  The node shells out to `docgraph graph --json` per non-bare component (read-only,
  preserving nodes-are-read-only-on-repos) and attaches a `DocGraphDigest` to the
  `Component` (rides into `graph.json`) plus the raw full payload to
  `Manifest.DocGraphs` (out-of-band, keyed by canonical id). **Footgun:** the full
  payload must stay on `Manifest`, never on `Component` — `Component` is shared by
  both `Manifest` and `Graph`, so a field on it would either bloat `graph.json` or
  fail to reach the hub. docgraph absent / bare repo / bad output → no digest, never
  a failed scan. Unknown `schemaVersion` is recorded, not interpreted — the digest
  carries only the observed value, and `myco audit` reports it as a
  `docgraph-version` finding (alongside `doc-rot` for a component with ≥1 island).
  `entryDocs` is the subset of Mycelium's own `conventionalEntryDocs` (not a
  docgraph concept) present in the repo. The hub writes payloads to
  `<outDir>/repos/<id>/docgraph.json` (on-disk layout mirrors the served URL) and
  `internal/serve` serves `GET /repos/<id>/docgraph.json`. Full field/route detail:
  `schema/graph.md`.

## Test / build
    go test ./...
    go build -o myco ./cmd/myco

## Regenerating the served graph
After adding or editing a `mycelium.toml` sidecar (in any scanned repo), changing the
overlay, or shipping a `myco` change, **regenerate the served graph** rather than
waiting for the scheduled scan — otherwise the live graph lags the sources. A node
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
- **Never a release without notes.** `just release` builds the body from the commit
  subjects since the previous tag; `just release notes=<file>` overrides that with prose
  when a release needs explaining (a breaking change, a migration). It refuses to publish
  an empty body. **Footgun:** do not reach for `gh release create --generate-notes` — it
  summarises merged *PRs*, and this repo integrates by direct merge to `dev` with no PRs,
  so it silently yields a bare compare link. v0.4.0 shipped with empty notes that way.
