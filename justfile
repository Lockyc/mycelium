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
