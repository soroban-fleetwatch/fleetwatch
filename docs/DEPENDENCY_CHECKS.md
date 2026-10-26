# Dependency and CI security checks

Run the complete gate after activating the development Go environment:

```sh
./scripts/check-vulnerabilities.sh
```

The script verifies module checksums, installs **govulncheck v1.9.0** into an
isolated temporary directory, reports its toolchain/database version and performs:

1. A module-level scan from `cmd/fleetwatch`, using the root module and the CLI's
   runtime dependency graph. This catches advisories in affected modules even
   when the affected package is not imported. The directory is intentional:
   module-only mode has no package patterns and the repository root has no Go files.
2. A source symbol scan of `./...` including tests.
3. A scan of the freshly built, trimmed-path CLI binary.

All gates use text output and preserve nonzero exits. `govulncheck` text mode
exits 3 for findings and nonzero for operational failures. JSON/SARIF/OpenVEX
formats can exit 0 despite findings, so they are not substituted for this gate.
A failed earlier gate stops the script; subsequent scans are not claimed to have
run. Temporary tools/binaries are removed on exit. No credentials or live Stellar
RPC/webhook endpoints are required. Standard HTTPS Go package/checksum access and
`https://vuln.go.dev` are required; preserve inherited proxy and CA routing.
An unavailable database is an operational failure, not a clean result.

Go's language directive remains 1.25.0, while `toolchain go1.27.3` records the
selected development compiler. CI's setup-go action reads the toolchain directive
and selects that exact version, with automatic switching disabled in the CI job.
Local developers should use Go 1.27.3 for the verified workflow. The scanner is a
separately pinned development tool, not an application dependency.

## October 27, 2026 evidence

The initial database request returned Forbidden. A narrow environment draft added
`vuln.go.dev` while preserving existing custom domains/presets. Direct readiness
checks subsequently succeeded, and the real scanner was rerun successfully with
TLS/proxy settings preserved. The saved draft still requires user publication
for the reusable environment; current-instance success does not prove fresh-task
restoration.

The initial source/test and binary scans found no reachable vulnerabilities and
no advisories in imported packages. They did report one affected required module.
A module-level scan exited 3 for **GO-2026-5841**, an out-of-bounds read in
`github.com/klauspost/compress/s2`, affecting the required compress v1.17.6.
The current CLI imports the module's zstd package through Stellar SDK XDR, not s2.
This is not evidence of a reachable FleetWatch exploit.

Updated the indirect dependency to the advisory's fixed version **v1.18.7** and
regenerated its Go checksums. The Stellar SDK and other runtime dependency pins
remain unchanged. With Go 1.27.3 and govulncheck v1.9.0, module, source/test and
compiled-binary gates then all exited 0 with no vulnerabilities found. The
upstream database reported update time **2026-10-21 00:27:29 UTC**. See the
[structured evidence](research/2026-10-27-dependency-checks.json) and
[Go advisory](https://pkg.go.dev/vuln/GO-2026-5841).

These are point-in-time linux/amd64 checks against known advisory data. They do
not cover unknown vulnerabilities, all build tags/platforms, consensus safety or
an independent audit. Run the gates again after dependency/compiler changes and
when advisory data changes. If a module-only finding appears unreachable, review
its scope and fix the dependency; the gate does not silently exempt it.

## CI configuration

`Go CI` has separate `test` and `vulnerabilities` jobs, with read-only repository
permissions, no persisted checkout credentials, time limits and cancellation of
superseded runs. Dependency declarations are checked with `go mod tidy -diff`;
Go commands use `-mod=readonly`. Unit/race/vet/build checks and the complete
vulnerability script run without remote application secrets.

Official action refs are pinned to inspected release SHAs:

| Action | Version | Commit |
|---|---|---|
| actions/checkout | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| actions/setup-go | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |

Their action manifests use Node 24; setup-go documents runner >= v2.327.1 and
support for the go.mod toolchain directive. The workflow uses `ubuntu-latest`.
Release refs/manifests and setup-go documentation were inspected from official
repositories. Local workflow validation used **actionlint v1.7.12**, plus Bash
syntax validation of the script. No remote GitHub Actions success is inferred
from local checks; remote CI is unverified. No release/tag, merge, deployment or
security-report upload is performed by these jobs.

Sources:

- https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
- https://vuln.go.dev
- https://github.com/actions/checkout/tree/3d3c42e5aac5ba805825da76410c181273ba90b1
- https://github.com/actions/setup-go/tree/b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
