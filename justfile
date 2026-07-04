# mycelium — task runner

# List available recipes
default:
    @just --list

# Build the myco binary
[group("build")]
build:
    go build -o myco ./cmd/myco

# Materialise the bundled examples and run the full scan → build → audit pipeline
[group("build")]
demo:
    ./scripts/demo.sh

# Run the test suite
[group("check")]
test:
    go test ./...

# go vet static checks
[group("check")]
vet:
    go vet ./...

# Format all Go files in place
[group("check")]
fmt:
    gofmt -w .

# Non-mutating pre-merge gate: gofmt check + vet + tests
[group("check")]
gate:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
      echo "✗ gofmt: these files need formatting:" >&2
      echo "$unformatted" >&2
      exit 1
    fi
    go vet ./...
    go test ./...
    echo "✓ gate passed"

# Cut the release for the current VERSION: fast-forward main → tag v<VERSION> → GitHub release.
# Run on dev with VERSION already bumped and committed; the tree must be clean and gate-green.
[group("release")]
release:
    #!/usr/bin/env bash
    set -euo pipefail
    version="$(tr -d '[:space:]' < VERSION)"
    tag="v${version}"
    if [ -n "$(git status --porcelain)" ]; then
      echo "✗ working tree is dirty — commit the VERSION bump first" >&2
      exit 1
    fi
    if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
      echo "✗ tag ${tag} already exists — bump VERSION before releasing" >&2
      exit 1
    fi
    just gate
    git push origin dev
    # main only fast-forwards to the release commit; it never diverges from dev.
    git branch -f main dev
    git push origin main
    git tag -a "${tag}" -m "${tag}" main
    git push origin "${tag}"
    gh release create "${tag}" --target main --title "${tag}" --generate-notes
    echo "✓ released ${tag}"
