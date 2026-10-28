# CAP-85 Testnet validation

Dripwave Phase 8's scoped Testnet validation passed on 2026-10-27. Actual fixture
deployments, shared upgrades/rollback, direct repointing, bounded history,
restart recovery and local signed alert delivery were exercised. See the
[recorded evidence](research/2026-10-27-testnet-validation.md). This is a test-only
contract workspace, separate from the Go watcher. Chain authentication and
production readiness remain unfinished; every chain verdict is UNDETERMINED.

## Local preparation

The pinned tools are Rust 1.99.1 with `wasm32v1-none` and rustfmt, Soroban SDK
29.0.1, and Stellar CLI 28.1.0. The Rust workspace has a committed Cargo.lock.
The CLI performs required contract-spec processing; calling `cargo build`
directly is insufficient for this SDK. Do not set the SDK's spec-processing
environment marker to bypass that check.

In the prepared cloud instance:

```sh
export PATH=/workspace/.tools/go/bin:/workspace/.tools/stellar-cli-28.1.0:/workspace/.tools/rust/bin:$PATH
export GOCACHE=/workspace/.cache/go-build
export GOMODCACHE=/workspace/.cache/go-mod
export GOPATH=/workspace/.cache/go-path
export CARGO_HOME=/workspace/.cache/cargo
cd /workspace/fleetwatch
./scripts/demo-testnet.sh check
```

On another machine, install the same Rust toolchain and target with rustup,
install Stellar CLI 28.1.0 from its official release, and activate Go from
`.go-version`. The Linux x86-64 CLI archive SHA-256 is
`c1680deee94301d33ada7a17f98411e642a4248c727afbd2e43050d345746462`.
The fixture CI job demonstrates the installation and offline checks.

`check` builds two member Wasm versions and an administrator-controlled owner,
then runs three tests against those actual Wasm blobs in the Soroban test host:

- Publish A, deploy two distinct external-reference members, publish B, and
  observe both members return version 2. Republish A and observe version 1.
- Repoint one member to direct Wasm A, publish B again, and observe only the
  member still using the external reference follow the shared update.
- Reject unauthorized publication, factory deployment and member repointing.

These tests use a simulated host and mocked authorization only for the positive
case. They are not transactions on Testnet. The negative tests do not mock auth.
Artifacts and host snapshots are ignored. No private keys are required by `check`.

The final read-only preflight uses FleetWatch's existing HTTPS/JSON/XDR reader,
checks the exact Testnet passphrase and supported protocol, requires a fresh
head, and retrieves one ledger two behind that head. Its JSON reports the sampled
sequence separately, with header and metadata digests. No account is funded and
no transaction is signed or sent. Exit 4 means a failed prerequisite; the report
keeps source authentication UNVERIFIED and metadata commitments NOT_VERIFIED.

## Controlled live smoke

The cloud policy now permits `friendbot.stellar.org`; faucet funding and
subsequent fixture transactions succeeded. Other environments need that hostname
as well as the public Testnet RPC. Preserve the managed proxy and CA trust.

The official CLI's jsonrpsee RPC transport uses a direct connector instead of
the managed HTTP proxy. The live runner starts a loopback-only adapter for the
CLI, with allowed RPC methods and one fixed upstream Testnet HTTPS URL. Go's
outbound client retains the inherited proxy and verifies upstream TLS with the
OS trust store. It rejects redirects, browser-origin requests, unknown/ambiguous
RPC methods and oversized bodies. It accepts no upstream URL override or local
authorization headers. This is a developer adapter, not a public service.

An actual CLI health check through the adapter succeeded. To reproduce manually,
run this in one terminal:

```sh
go run ./scripts/testnet-rpc-bridge
```

Pass its printed loopback address as `--rpc-url` to `stellar network health`,
with `--network-passphrase 'Test SDF Network ; September 2015' --output json`.
Stop the adapter with Ctrl-C; it also expires after 15 minutes. The live runner
starts and stops its own instance. Do not expose it beyond loopback.

Use a dedicated disposable Testnet identity stored outside the repository. Never
use a production identity, pass a seed on the command line, or commit its config.
The following commands create/fund a test-only identity, then perform Testnet writes:

```sh
mkdir -m 700 -p /workspace/.private/fleetwatch-testnet
stellar keys generate fleetwatch-demo --config-dir /workspace/.private/fleetwatch-testnet
public_key="$(stellar keys address fleetwatch-demo --config-dir /workspace/.private/fleetwatch-testnet)"
curl --fail --silent --show-error --proto '=https' --get \
  --data-urlencode "addr=$public_key" https://friendbot.stellar.org/
mkdir -p testnet-runs
./scripts/demo-testnet.sh run fleetwatch-demo \
  /workspace/.private/fleetwatch-testnet testnet-runs/first-upgrade
```

The runner requires a new output directory and fixed Testnet endpoint/passphrase.
It uploads the exact tested Wasm, deploys the owner and two members, captures A,
publishes B, captures B, and requires the diagnostic finding to name exactly both
members. It retains public CLI receipts and both original capture envelopes.
An unexpected capture error stops the run. It does not fund accounts, overwrite
an earlier run, publish a release, send webhooks or change other contracts.

The one permitted capture diagnostic is that multi-call consistency is not
proven. Extracted snapshots are compared explicitly as UNVERIFIED_SNAPSHOTS;
PARTIAL capture and UNVERIFIED authorization are never upgraded to chain proof.
The `UNAPPROVED_REFERENCE_CHANGE` code describes absence of a supplied policy
match, not verified maliciousness or a verdict about the administrator.

The live transaction path passed. A run creates new test-only contracts, spends
Testnet faucet funds for fees, and leaves public transaction evidence. Keep its
signing identity only for the duration of the test; the recorded run's identity
was removed after validation and was never committed or printed.

## Full scoped validation

After a fresh snapshot smoke, continue with the same disposable identity:

```sh
python3 scripts/validate-testnet.py fleetwatch-demo \
  /workspace/.private/fleetwatch-testnet testnet-runs/first-upgrade \
  testnet-runs/first-validation
```

This performs additional Testnet writes to those fixture contracts. It checks
the original deployment/upgrade receipts through a separate Go read path,
recomputes network-bound envelope hashes, decodes successful XDR results and
reconstructs the observed transition from real ledger metadata and an asserted
baseline. It then exercises rollback, direct repoint, another shared update and
submitted version calls, whose actual result metadata must show 1/1 then 1/2.
It runs a finite watcher, cancels after an accepted prefix, resumes it, and
checks stable outbox contents across another process restart.

The target is twelve ledgers beyond the new baseline, inside the existing
sixteen-ledger bound. Slow transactions or source failures can exhaust that
window and fail the run; the script does not expand the bound or relabel a
partial run as success. Use a fresh smoke/fleet for another attempt rather than
changing the acceptance assertions. Output directories are never overwritten.

Two opt-in tests use retained real Testnet data: a local RPC fault replay omits
one closed ledger and proves rejection/cold-reopen recovery, and a backup of the
real outbox is delivered to the existing loopback-only verified-TLS fixture.
The receiver verifies exact-body HMAC, alert IDs and unchanged assurance, then
reopens the DB and checks that acknowledged deliveries are not repeated. These
tests skip in normal CI unless their explicit evidence paths are supplied by
the runner. CI does not perform live transactions or contact an external receiver.

The fault replay is an injected local omission, not a real missing Testnet
ledger. Future-target availability failures from the live watcher are recorded
separately. Local TLS delivery does not establish production webhook routing;
external delivery still requires an explicitly authorized receiver. A snapshot
comparison alone does not pass this validation.

Baseline authentication, independent header anchoring, metadata commitments,
broader fleet completeness and production monitoring remain separate engineering
gaps. This demo cannot turn the current UNDETERMINED chain verdict into approval.
