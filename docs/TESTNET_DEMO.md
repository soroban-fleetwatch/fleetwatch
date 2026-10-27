# CAP-85 Testnet validation

Phase 8 is in progress. The compiled fixture tests and read-only RPC preflight
pass. No fixture contract has been deployed and no upgrade transaction has been
submitted by this workflow. The live smoke below has not yet run successfully.
This is a test-only contract workspace, separate from the Go watcher.

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

The cloud funding request currently fails with proxy `Forbidden` for
`friendbot.stellar.org`. A draft adds only that domain to the existing custom
allowlist. Review/save the draft and publish the cloud environment before retrying.
Saving the draft alone does not activate the change.

The official prebuilt CLI also returned a transport `Connect` error for RPC
while FleetWatch's Go client succeeded. Preserve the proxy and CA trust when
resolving this; do not disable TLS verification. The CLI must pass its own health
check before executing live steps:

```sh
stellar network health --rpc-url https://soroban-testnet.stellar.org \
  --network-passphrase 'Test SDF Network ; September 2015' --output json
```

Use a dedicated disposable Testnet identity stored outside the repository. Never
use a production identity, pass a seed on the command line, or commit its config.
The following commands create/fund a test-only identity, then perform Testnet writes:

```sh
mkdir -m 700 -p /workspace/.private/fleetwatch-testnet
stellar keys generate fleetwatch-demo --config-dir /workspace/.private/fleetwatch-testnet
stellar keys fund fleetwatch-demo --config-dir /workspace/.private/fleetwatch-testnet \
  --rpc-url https://soroban-testnet.stellar.org \
  --network-passphrase 'Test SDF Network ; September 2015'
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

The live runner is prepared but its transaction path remains unverified. Do not
present its future output filenames as evidence that those transactions happened.

## Full live acceptance still required

After the snapshot smoke, retain actual transaction hashes and fetch their
successful transaction/ledger metadata through a separate read path. Establish
the baseline and bounded ledger interval, reconstruct both members' transition,
and compare the result with the known submitted upgrade. Repeat for rollback and
direct repoint. Exercise a deliberate history gap, process restart with no
duplicate events, and a local verified-TLS alert receiver. External delivery
requires an explicitly authorized receiver. A snapshot comparison alone does not
pass this acceptance gate.

Baseline authentication, independent header anchoring, metadata commitments,
broader fleet completeness and production monitoring remain separate engineering
gaps. This demo cannot turn the current UNDETERMINED chain verdict into approval.
