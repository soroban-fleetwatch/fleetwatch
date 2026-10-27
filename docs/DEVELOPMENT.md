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

Go 1.27.3 is pinned in `.go-version` for CI, recommended by the go.mod toolchain directive, and used for current-instance validation; the module language floor remains Go 1.25 for Stellar SDK v0.8.0. CI has not been run remotely. No database, service, or credentials are needed for this offline milestone. Synthetic contract labels are accepted; this is not Stellar address verification. Unsigned policy matches do not authenticate approvals. See `build/STATUS.md` for remaining work.

Read-only capture, bounded resumable ingestion and finite polling are implemented; see CAPTURE.md, INGESTION.md and WATCH.md. Controlled live CAP-85 upgrade proof remains unfinished. See RPC_JSON.md for reproducible parser fuzz/benchmark commands and the limits of those measurements.


Run `./scripts/check-vulnerabilities.sh` for the pinned module, source/test and
fresh-binary security gates. It installs its own temporary scanner and requires
Go package/checksum access plus vuln.go.dev. See [DEPENDENCY_CHECKS.md](DEPENDENCY_CHECKS.md)
for findings, compiler/action pins, CI checks and point-in-time limits.

For the optional CAP-85 fixture workspace, activate
`/workspace/.tools/stellar-cli-28.1.0` and `/workspace/.tools/rust/bin` in PATH,
set `CARGO_HOME=/workspace/.cache/cargo`, and run `scripts/demo-testnet.sh check`.
See [TESTNET_DEMO.md](TESTNET_DEMO.md) for pinned tools, funding/network blockers
and the distinction between local Wasm tests and live acceptance.
