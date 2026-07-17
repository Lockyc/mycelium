# mycelium — task runner

# `default` pipes `just --list` through a small stock-perl filter that clips long recipe
# docs to your terminal width (…) instead of wrapping. Self-contained — no external files;
# falls back to plain `just --list` where perl is absent. Edit the recipes below, not this.
# List available recipes
default:
    @if command -v perl >/dev/null 2>&1; then just --color always --list | perl -CS -Mutf8 -lpe 'BEGIN{($w)=`stty size 2>/dev/null </dev/tty`=~/ (\d+)/; $w||=100; $col=(-t STDOUT && !exists $ENV{NO_COLOR})} s/\e\[[0-9;]*m//g unless $col; (my $v=$_)=~s/\e\[[0-9;]*m//g; if(length($v)>$w){my($o,$n)=("",0); while(length && $n<$w-1){ if($col && s/^(\e\[[0-9;]*m)//){$o.=$1}else{s/^(.)//;$o.=$1;$n++} } $_=$o."…".($col?"\e[0m":"")}'; else just --list; fi

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
# Notes default to the commit subjects since the previous tag; pass notes=<file> for prose.
[group("release")]
release notes="":
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
    if [ -n "{{notes}}" ] && [ ! -f "{{notes}}" ]; then
      echo "✗ notes file not found: {{notes}}" >&2
      exit 1
    fi
    just gate
    git push origin dev
    # main only fast-forwards to the release commit; it never diverges from dev.
    # Guard the doc carve-out footgun: a doc commit landed on main but not merged
    # back into dev would be silently dropped by `branch -f`. Fail loudly instead.
    if ! git merge-base --is-ancestor main dev; then
      echo "✗ main is not an ancestor of dev — a doc commit on main isn't merged into dev." >&2
      echo "  Run: git checkout dev && git merge main   (then re-run the release)." >&2
      exit 1
    fi
    # Resolve the previous tag BEFORE tagging, so it doesn't find this release's own tag.
    prev="$(git describe --tags --abbrev=0 2>/dev/null || true)"
    git branch -f main dev
    git push origin main
    git tag -a "${tag}" -m "${tag}" main
    git push origin "${tag}"
    # Build the release body. NOT --generate-notes: it summarises merged PRs, and this repo
    # integrates by direct merge to dev, so it yields a bare compare link and says nothing —
    # v0.4.0 shipped with empty notes that way. Commit subjects since the previous tag are
    # the real changelog here; a notes=<file> override carries prose when a release needs it.
    body="$(mktemp)"
    trap 'rm -f "$body"' EXIT
    if [ -n "{{notes}}" ]; then
      cat "{{notes}}" > "$body"
    elif [ -n "$prev" ]; then
      git log --no-merges --pretty='- %s' "${prev}..main" > "$body"
    else
      echo "- initial release" > "$body"
    fi
    # Check emptiness BEFORE appending the compare link — otherwise the link (and the blank
    # line before it) is itself content, and a release with nothing to say passes the check.
    if [ ! -s "$body" ]; then
      echo "✗ refusing to publish ${tag}: no commits since ${prev:-the beginning} and no notes=<file>" >&2
      exit 1
    fi
    if [ -n "$prev" ]; then
      printf '\n**Full Changelog**: %s/compare/%s...%s\n' \
        "$(gh repo view --json url -q .url)" "$prev" "$tag" >> "$body"
    fi
    gh release create "${tag}" --target main --title "${tag}" --notes-file "$body"
    echo "✓ released ${tag}"
