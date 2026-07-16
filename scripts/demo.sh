#!/usr/bin/env bash
# demo.sh — build a Mycelium ecosystem graph from the bundled examples in a throwaway
# temp workspace, so you can see myco end-to-end without any real repos.
#
#   ./scripts/demo.sh          # build + print the map, then clean up
#   KEEP=1 ./scripts/demo.sh   # keep the temp workspace to inspect the files
#
# Doubles as a smoke test: exercises scan -> build -> audit against real (throwaway)
# git repos.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
myco="$root/myco"
if [ ! -x "$myco" ]; then
	echo "building myco..." >&2
	(cd "$root" && go build -o myco ./cmd/myco)
fi

work=$(mktemp -d)
if [ -z "${KEEP:-}" ]; then trap 'rm -rf "$work"' EXIT; fi
repos="$work/repos"
manifests="$work/manifests"
out="$work/graph"
mkdir -p "$repos" "$manifests" "$out"

# Materialise each example as a throwaway git repo with a fake origin, so myco
# can derive a canonical ID (git@github.com:acme/<name>.git -> github.com/acme/<name>).
for dir in "$root"/examples/repos/*/; do
	name=$(basename "$dir")
	dest="$repos/$name"
	mkdir -p "$dest"
	cp "$dir"mycelium.toml "$dest"/
	git -C "$dest" init -q
	git -C "$dest" remote add origin "git@github.com:acme/$name.git"
	git -C "$dest" add mycelium.toml
	# myco reads sidecars from git HEAD, so the sidecar must be committed (not just present).
	git -C "$dest" -c user.email=demo@example.com -c user.name=demo \
		commit -q -m "example"
done

# Two repos with NO mycelium.toml, to show orphan handling: 'needs-sidecar' is
# flagged by the audit as an orphan; 'scratch' is suppressed via the overlay's
# ignore list (see examples/overlay.toml).
for name in needs-sidecar scratch; do
	dest="$repos/$name"
	mkdir -p "$dest"
	git -C "$dest" init -q
	git -C "$dest" remote add origin "git@github.com:acme/$name.git"
	git -C "$dest" -c user.email=demo@example.com -c user.name=demo \
		commit -q --allow-empty -m "no sidecar"
done

echo "==> myco scan"
"$myco" scan --roots "$repos" --node demo --out "$manifests/demo.json"
echo "==> myco build (with example overlay)"
"$myco" build --manifests "$manifests" --overlay "$root/examples/overlay.toml" --dir "$out"
echo "==> myco audit"
"$myco" audit --dir "$out" || true

echo
echo "===================== MAP.md ====================="
cat "$out/MAP.md"
echo "=================================================="
if [ -n "${KEEP:-}" ]; then
	echo "workspace kept at: $work"
else
	echo "(temp workspace cleaned up; run with KEEP=1 to inspect)"
fi
