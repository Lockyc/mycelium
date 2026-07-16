# Examples

Sample `mycelium.toml` sidecars and a private `overlay.toml` that demonstrate the
Mycelium graph schema. [`scripts/demo.sh`](../scripts/demo.sh) materialises
these into throwaway git repos in a temp dir and runs `myco scan` / `build` /
`audit` end-to-end, so you can see a populated graph without any real repos.

    ./scripts/demo.sh          # build + print the graph, then clean up
    KEEP=1 ./scripts/demo.sh   # keep the temp workspace to inspect the files

- `repos/<name>/mycelium.toml` — one public-safe sidecar per example repo.
- `overlay.toml` — private nodes + relationship edges merged in at build time.

The three example repos (`orders-api`, `billing-web`, `shared-lib`) plus the
overlay produce a graph with three components, three capabilities
(`order-events`, `billing-ui`, `postgres`), and four resolved relationship
edges — so `myco audit` reports clean. See [`schema/graph.md`](../schema/graph.md)
for the full field reference.
