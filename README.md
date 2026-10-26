# Soroban FleetWatch

**Experimental prototype — offline comparisons and diagnostic RPC capture; live Testnet validation is pending.**

Soroban FleetWatch is a Go-based, open-source tool for observing **Soroban contract fleets** that share executable code through externally managed references (CAP-85, Stellar Protocol 28). A single update to the owner contract's tagged Wasm hash can change the executable of many referenced contracts. FleetWatch compares captured ledger-state snapshots, identifies such changes, calculates the affected contracts, and evaluates observed hash transitions against an explicit approval file.

> **Security boundary:** Snapshot comparisons use JSON supplied by an operator. They do not authenticate ledger entries, validate network consensus, attribute an on-chain actor, or provide continuous alerting. An unsigned policy match is not proof of authorization. The separate signed approval verifier checks scoped signatures against operator-provided trust roots; it does not authenticate chain observations.

## Quick start

Requires Go 1.25+ for pinned Stellar SDK v0.8.0. The cloud environment uses verified Go 1.27.3.

```sh
go test ./...
go run ./cmd/fleetwatch validate --snapshot testdata/before.json
go run ./cmd/fleetwatch compare \
  --before testdata/before.json \
  --after testdata/after.json \
  --format json \
  --fail-on never
```

The synthetic demo changes the `release-stable` reference from hash `aa…aa` to `bb…bb` and detects that demo contracts A and B are affected.

An unapproved transition fails with exit code 2 when executed as a compiled binary:

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch compare --before testdata/before.json --after testdata/after.json
# exit status 2: critical finding
```

When the transition appears in the supplied approval policy, it produces an informational finding instead:

```sh
./fleetwatch compare --before testdata/before.json --after testdata/after.json \
  --policy testdata/approved-policy.json
# exit status 0
```

For CI reporting:

```sh
./fleetwatch compare --before old.json --after new.json \
  --policy approval.json --format json > fleetwatch-report.json
```

Reports explicitly label coverage as `UNKNOWN` and authorization as `UNVERIFIED`, including unsigned policy matches. Input bounds and compatibility are documented in [snapshot format](docs/SNAPSHOT_FORMAT.md).

Experimental read-only capture is documented in [CAPTURE.md](docs/CAPTURE.md). Multi-call consistency remains unproven and produces a partial diagnostic result.

Diagnostic [historical metadata collection](docs/HISTORY.md) reads bounded consecutive ledger-close metadata from RPC and checks header hashes/linkage. Two real Testnet protocol-29 records have been parsed; metadata commitments and authenticated durable fleet reconstruction remain unimplemented.

The offline [changes command](docs/METADATA_CHANGES.md) projects executable-related metadata in transaction-phase order, preserving preimages, intermediate writes and restores. It does not yet apply a baseline or infer complete fleet membership.

The separate [reconstruct command](docs/RECONSTRUCTION.md) applies an operator-asserted baseline to a fixed watchlist, checks executable preimages and reports operation-boundary changes. It stops on conflicts and uncertain watched evictions; baseline/source authentication and metadata commitments remain unverified.

The [bounded polling worker](docs/WATCH.md) retries a fixed target with strict JSON configuration, cancellation and NDJSON progress. It retains the original 16-ledger interval bound and does not send alerts.

Explicit [bounded RPC ingestion](docs/INGESTION.md) fetches from a retained reconstruction cursor through a selected target and saves evidence, findings and diagnostics atomically. It remains limited to 16 ledgers beyond the original asserted baseline.

Optional [reconstruction checkpoints](docs/RECONSTRUCTION_CHECKPOINTS.md) retain the original baseline, accepted raw metadata and findings atomically. Use `checkpoint` to inspect and revalidate a retained prefix after restart; this does not establish current source health or authenticate the chain.

Offline per-ledger snapshot [replay](docs/REPLAY.md) can expose transient changes and missing observations. It supports optional durable local state with `replay --state FILE`; continuous authenticated ledger-meta ingestion remains unfinished.

Durable replay also maintains a [local alert outbox](docs/ALERTS.md); `alerts` lists pending records without sending them. Experimental signed HTTPS [webhook delivery](docs/WEBHOOKS.md) is a separate `deliver` command. Explicit [dead-letter recovery](docs/ALERT_RECOVERY.md) preserves alert identity and records each operator retry.

Use [doctor](docs/HEALTH.md) to inspect persisted replay coverage and alert delivery health without changing the database. For reconstruction streams, add `--source rpc_ledger_meta --stream ID` to check the retained prefix, latest attempt, queue and recovery audit together. Its `OK_OFFLINE` status does not establish live chain coverage.

Use [report](docs/FLEET_REPORT.md) to query an observed owner/tag fleet at an exact retained ledger, including adjacent membership changes, snapshot digests and related replay events. Membership outside the supplied snapshots remains unknown.

The [backup command and recovery guide](docs/BACKUP.md) preserve replay, delivery and approval indices in a single local transaction snapshot. Restoring an older history still requires reconciling later nonce use and acknowledgments.

Offline [signed approval verification](docs/APPROVALS.md) checks Ed25519 statements against explicit trust roots. Optional [durable incident binding](docs/APPROVAL_BINDING.md) prevents nonce reuse within one database history. These checks remain separate from unsigned policy matching and do not authenticate chain observations.

Durable reconstruction now has [stream-scoped finding and diagnostic alerts](docs/RECONSTRUCTION_ALERTS.md). Inspect them with `alerts --source rpc_ledger_meta --stream ID`; explicit webhook delivery retains all unverified labels.

## CLI interface

| Command | Purpose |
|---|---|
| `version` | Print build version |
| `validate --snapshot FILE` | Validate a normalized snapshot |
| `compare --before FILE --after FILE [--policy FILE]` | Compare two observed ledger states |
| `--format text\|json` | Emit human- or machine-readable report |
| `--fail-on critical\|high\|warning\|info\|never` | Set CI severity threshold (default: high) |

Exit statuses: **0** success; **2** security threshold triggered; **3** invalid inputs/usage. Using `go run` wraps nonzero exits and typically displays `exit status 2`; compiled binaries preserve exact status codes.

## Architecture

```text
            Current prototype                          Future modules
       ┌──────────────────────┐                 ┌──────────────────────┐
       │ two normalized JSONs │                 │ RPC + ledger history │
       └──────────┬───────────┘                 └──────────┬───────────┘
                  ▼                                        ▼
        ┌────────────────────┐                  ┌────────────────────┐
        │ schema validation  │  ◀───────────────│ verified ingestion │
        └──────────┬─────────┘                  └────────────────────┘
                   ▼
        ┌────────────────────────┐
        │ reference & fleet diff │
        └──────────┬─────────────┘
                   ▼
       ┌──────────────────────────┐
       │ exact hash-approval match│
       └──────────┬───────────────┘
                  ▼
          text/JSON security report
                  │
                  └── next: signed policies, continuous watches, alerts
```

This repo does not deploy a Soroban smart contract. An on-chain monitoring contract is not needed for the current functionality.

## Data model

- **Contract executable `wasm`:** a direct 32-byte hash.
- **Contract executable `external_ref`:** owner contract identifier and `tag_hex`, representing the raw CAP-85 executable tag bytes.
- **Contract executable `stellar_asset`:** the Stellar Asset Contract executable variant.
- **Reference:** owner, raw tag bytes, and resolved 32-byte Wasm hash at the observed ledger.
- **Approval:** exact owner, tag, previous hash, new hash, and nonempty human change ticket.

The CAP-85 executable tag is an `SCString`, which is not safe to assume is Unicode text. Store its original bytes as **hex**, never a lossy decoded string.

See [Snapshot Format](docs/SNAPSHOT_FORMAT.md), [Threat Model](docs/THREAT_MODEL.md), and [Development Plan](docs/BUILD_PLAN.md).

## Current detection rules

| Rule | Severity | Meaning |
|---|---|---|
| `UNAPPROVED_REFERENCE_CHANGE` | Critical | Shared reference changed without an exact local-policy match |
| `POLICY_MATCHED_REFERENCE_CHANGE` | Info | Exact before/after transition matches provided policy |
| `CONTRACT_EXECUTABLE_REPOINTED` | High | Contract switched executable kind, direct hash, or reference owner/tag |
| `REFERENCE_NOT_OBSERVED` | High | Previously seen reference missing in a later supplied snapshot |
| `UNRESOLVED_EXTERNAL_REFERENCE` | High | A contract points to an owner/tag whose value is missing from input |
| `CONTRACT_NOT_OBSERVED` | High | A previously observed contract is missing in a later snapshot |
| `NEW_REFERENCE_OBSERVED` | Info | Reference first appears, without a historic baseline |
| `NEW_CONTRACT_OBSERVED` | Info | Contract first appears, without a historic baseline |

**Do not equate missing input with deletion.** CAP-85 tag entries have restrictions against deletion; missing observations are treated as collection gaps, not proof the ledger entry was destroyed.

## Project status

- [x] Research CAP-85 and competing pre-deployment upgrade tools
- [x] Define the normalized fleet snapshot and approval schemas
- [x] Implement deterministic comparison, severity, CLI and regression tests
- [ ] Read ledger entries directly using the official Stellar Go SDK
- [ ] Test with actual Protocol-28+ network fixtures (Testnet)
- [ ] Persist replay-safe block checkpoints and recover from gaps
- [ ] Verify signed approval provenance and expected upgrade authority
- [ ] Continuous watch mode, alert delivery and dashboards
- [ ] Harden against untrusted data sources, resource exhaustion and partial ledgers

## Research references

- [Stellar CAP-85 — Externally Managed Contract Executables](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0085.md)
- [Protocol 28 introduction](https://stellar.org/blog/developers/introducing-adapter-protocol-28-on-stellar)
- [Official Stellar Go SDK](https://github.com/stellar/go-stellar-sdk)
- [Soroban Upgrade Safeguard — complementary predeployment tool](https://github.com/ShippedLabs/soroban-upgrade-safeguard)

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). This repository is **not audited** and should not be the sole protection for deployed funds or contracts. Please report issues privately rather than publishing exploitable vulnerabilities.
