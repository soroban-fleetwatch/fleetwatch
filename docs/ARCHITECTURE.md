# Architecture

FleetWatch is one Go module, `github.com/soroban-fleetwatch/fleetwatch`, built
into one binary. It has no server component, no shared database and no
on-chain contract. A Rust workspace under `contracts/testfixtures` exists only
to build Testnet fixtures.

## Data flow

```text
 Stellar RPC (HTTPS, read-only)          operator files
 getNetwork · getLatestLedger            baseline · watchlist · config
 getLedgers · getLedgerEntries           snapshots · policy · trust roots
            │                                     │
            ▼                                     ▼
 internal/source ── strict JSON (internal/jsoninput), bounded XDR,
            │        network/protocol/freshness checks, retries
            ▼
 internal/history ── header hash + previous-link checks, change projection
            │         in transaction-phase order, watched reconstruction
            ▼
 internal/follow, internal/ingest, internal/monitor
            │   batch planning, cursor fencing, retention budgets
            ▼
 internal/store (bbolt) ── raw evidence + projection + findings + cursor
            │               + outbox, committed in one transaction
            ├──► doctor · report · checkpoint · follow-inspect (read)
            ├──► internal/followdelivery ──► delivery outbox DB
            │                                   │
            │                                   ▼
            │                      internal/alert ── HMAC HTTPS webhook
            │                                         (allowlist, DNS pinning,
            │                                          optional CONNECT proxy)
            └──► internal/policy ── Ed25519 approvals, kept separate
```

## Packages

| Package | Responsibility |
| --- | --- |
| `cmd/fleetwatch` | Command-line entry point; flag parsing and exit codes only |
| `cmd/fleetwatch-browser` | WebAssembly build of the current-state viewer |
| `internal/jsoninput` | Bounded JSON reader: size, depth, member count, duplicate and case-alias rejection |
| `internal/source` | RPC transport and typed decoding of executables, owner/tag entries and ledger metadata |
| `internal/history` | Ledger range checks, change projection and asserted-baseline reconstruction (pure, no I/O) |
| `internal/fleetwatch` | Snapshot model, comparison, replay and fleet reports (pure, no I/O beyond file loading) |
| `internal/ingest`, `internal/monitor` | Bounded resumable ingestion and the finite polling worker |
| `internal/follow` | Continuous follower state, archives, restore, compaction and loopback status |
| `internal/store` | bbolt persistence for replay, reconstruction, outboxes, approvals, recovery and backup |
| `internal/followdelivery` | Moves validated follow findings into a separate delivery outbox |
| `internal/alert` | Lease-fenced dispatch and the signed HTTPS webhook sender |
| `internal/policy` | Signed approval statements, trust roots, thresholds, revocation and rotation |
| `internal/config` | Strict configuration for the `watch` worker |
| `internal/stateformat` | Names of recognized on-disk root buckets |

Source adapters are separate from comparison and reconstruction. The pure
packages take decoded values and return findings; they never call the network.
That makes every detection rule testable with offline fixtures.

## Storage

Each command that keeps state uses a local bbolt file with an exclusive lock.
A batch of ledgers commits its raw metadata, the new projection, findings, the
cursor and outbox rows in one transaction. If anything fails, nothing from that
batch is kept. On restart the follower replays all retained raw evidence from
the original baseline and checks it before trusting cached state.

Limits are explicit. A follow stream holds at most 100,000 ledgers, 256 MiB of
retained payload and 10,000 findings. When a limit is reached, `follow` stops
with exit 6 instead of discarding evidence. Archives, restore and compaction
work on a stopped follower and write new files.

## Delivery

Delivery is at least once. Each alert has a stable ID, which receivers use to
deduplicate. Attempts hold a lease with a random fencing token, retry with
backoff and become dead letters after five attempts. An operator can requeue a
dead letter; each requeue writes an audit receipt. Webhook requests are signed
with HMAC over the exact body. The sender accepts only HTTPS hosts on an
explicit allowlist, rejects private and changing DNS answers, and pins the
connection to the checked IP.

## Trust boundaries

| Boundary | Trusted for | Not trusted for |
| --- | --- | --- |
| Operator files and paths | Configuration, the asserted baseline, the watchlist | Proof of chain state |
| RPC endpoint | Availability of data | Correctness of data (labelled `UNVERIFIED`) |
| Local database | Continuity between runs | Provenance against someone who can rewrite the whole file |
| Webhook receiver | Receiving alerts | Anything returned beyond an acknowledgement |
| Approval signers | Approving an exact transition | Authenticity of the observed chain data |

The [threat model](THREAT_MODEL.md) lists the attacks considered and the checks
for each.

## Deployment shape

The intended deployment is one Linux host running `follow` under systemd, with
a local state file and loopback-only status and metrics. Delivery is optional
and runs as a separate process with its own database. See
[Operations](OPERATIONS.md) and [ADR 0004](adr/0004-portable-single-host-follow.md).
No permanent host has been accepted yet; see [Phase 9 acceptance](PHASE9_ACCEPTANCE.md).
