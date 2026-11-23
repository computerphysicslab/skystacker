#!/usr/bin/env bash
# Build SkyStacker and refresh every installed copy in one step.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

gofmt -w *.go
go vet ./...
STAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
go build -ldflags "-X main.buildDate=${STAMP}" -o skystacker .

install -d /home/jips/.local/bin /home/jips/go/bin
install -m 755 skystacker /home/jips/.local/bin/skystacker
install -m 755 skystacker /home/jips/go/bin/skystacker

/home/jips/.local/bin/skystacker --selftest | tail -2
go test ./... | tail -2
printf 'PATH binary: '; command -v skystacker
skystacker --version
