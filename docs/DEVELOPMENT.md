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

Go 1.27.3 was used for current-instance validation; `go.mod` requires Go 1.25 for Stellar SDK v0.8.0. CI has not been run remotely. No database, service, or credentials are needed for this offline milestone. Synthetic contract labels are accepted; this is not Stellar address verification. Unsigned policy matches do not authenticate approvals. See `build/STATUS.md` for remaining work.

Read-only diagnostic capture is now implemented; see CAPTURE.md. Live Testnet verification remains blocked by the current network policy.
