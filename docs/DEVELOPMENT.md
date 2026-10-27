# Cloud development

Use the existing `/workspace/fleetwatch` checkout. Cloud tasks are isolated; do not create a worktree unless requested.

The system `/usr/bin/go` is unrelated to the Go language. Activate the verified toolchain and writable caches in each shell:

```sh
export PATH=/workspace/.tools/go/bin:$PATH
export GOCACHE=/workspace/.cache/go-build
export GOMODCACHE=/workspace/.cache/go-mod
export GOPATH=/workspace/.cache/go-path
cd /workspace/fleetwatch
go test ./...
go vet ./...
go test -race ./...
go build -o /workspace/.tools/bin/fleetwatch ./cmd/fleetwatch
/workspace/.tools/bin/fleetwatch validate --snapshot testdata/before.json
```

Go 1.27.3 is pinned in `.go-version` for CI, recommended by the go.mod toolchain directive, and used for current-instance validation; the module language floor remains Go 1.25 for Stellar SDK v0.8.0. Remote CI for the current checkpoint has not been verified. No database, service, or credentials are needed for this offline milestone. Synthetic contract labels are accepted; this is not Stellar address verification. Unsigned policy matches do not authenticate approvals. See `build/STATUS.md` for remaining work.

Read-only capture, bounded resumable ingestion and finite polling are implemented; see CAPTURE.md, INGESTION.md and WATCH.md. Controlled live CAP-85 validation now passes within its documented unverified assurance; see TESTNET_DEMO.md. See RPC_JSON.md for reproducible parser fuzz/benchmark commands and the limits of those measurements.

The optional continuous `follow` mode and separate state format are documented
in FOLLOW.md. OPERATIONS.md describes the portable Linux candidate, local
installation and remaining hosted-production gates. It requires no new Go
dependency, database server or credential. `follow --check` is offline; ordinary
tests use synthetic typed metadata. Live follow validation is read-only.

Run `./scripts/check-linux-package.sh` on Linux amd64 or arm64 to exercise the portable
candidate's archive checksums, extracted installation, offline host report and
failure cases (overwrite, invalid baseline, insufficient space, unsafe state
permissions and tampering). CI runs the same check in its Go test job. It uses
temporary directories and reserved `.invalid` RPC URLs, starts no observer and
requires no Testnet or receiver access. Dependencies must already be cached for
the Go build to be offline. The report does not certify production hosting.

Packaging now accepts `./scripts/package-linux.sh NEW_DIR [amd64|arm64]`.
Default remains amd64. Package checks build both targets, inspect ELF/static and
Go target metadata, run only the host-native binary, and reject installation of
the foreign architecture before staging. Linux ARM64 execution is not validated
by cross-building on this amd64 workspace. Zero-cost host access and actual
Dripwave Phase 9 completion evidence remain separate gates.


Run `./scripts/check-vulnerabilities.sh` for the pinned module, source/test and
fresh-binary security gates. It installs its own temporary scanner and requires
Go package/checksum access plus vuln.go.dev. See [DEPENDENCY_CHECKS.md](DEPENDENCY_CHECKS.md)
for findings, compiler/action pins, CI checks and point-in-time limits.

For the optional CAP-85 fixture workspace, activate
`/workspace/.tools/stellar-cli-28.1.0` and `/workspace/.tools/rust/bin` in PATH,
set `CARGO_HOME=/workspace/.cache/cargo`, and run `scripts/demo-testnet.sh check`.
See [TESTNET_DEMO.md](TESTNET_DEMO.md) for pinned tools, live validation commands
and the distinction between local Wasm tests, scoped live evidence and production readiness.

The cloud bootstrap is now versioned as `scripts/setup-cloud.sh`; the environment
configuration stores its complete contents. It verifies a staged Go archive/tree
before replacing an existing compiler, preserves the old tree on checksum/staging
failure and is idempotent for the exact pinned version. It then runs frozen-module
checks, native tests/build and a synthetic CLI smoke. It does not publish an
application or provision a host. The saved repository declaration must select the
verified source HEAD at mount path `fleetwatch`, not an older main checkpoint.
See [environment repair evidence](research/2026-10-27-environment-repair.md).
