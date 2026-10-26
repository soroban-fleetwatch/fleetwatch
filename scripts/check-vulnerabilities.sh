#!/usr/bin/env bash
# Use text output for the exit-status security gate. JSON output is a report,
# not a substitute for the scanner's finding/error exit status.
set -euo pipefail

if (($# != 0)); then
  printf 'Usage: %s\n' "$0" >&2
  exit 2
fi

fleetwatch_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$fleetwatch_root"
fleetwatch_scan_dir="$(mktemp -d "${TMPDIR:-/tmp}/fleetwatch-govuln.XXXXXX")"
trap 'rm -rf -- "$fleetwatch_scan_dir"' EXIT

export GOTELEMETRY=off
go mod verify
GOBIN="$fleetwatch_scan_dir" go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
"$fleetwatch_scan_dir/govulncheck" -version
# Module-only mode needs a package directory with Go files; the repository
# root has none. The CLI shares the same root go.mod and runtime dependency graph.
"$fleetwatch_scan_dir/govulncheck" -C cmd/fleetwatch -scan=module
"$fleetwatch_scan_dir/govulncheck" -test ./...
go build -trimpath -o "$fleetwatch_scan_dir/fleetwatch" ./cmd/fleetwatch
"$fleetwatch_scan_dir/govulncheck" -mode=binary "$fleetwatch_scan_dir/fleetwatch"
