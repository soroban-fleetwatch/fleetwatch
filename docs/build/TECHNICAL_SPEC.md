# Technical Implementation Contract — FleetWatch

This is the implementation target, not a report of features already built. Refer to `END_TO_END_BUILD_PLAN.md` for stage sequencing. Use official Stellar XDR and SDK types once verified; all interface sketches below are design proposals and must not be mistaken for SDK symbols.

## 1. Components and interfaces

Suggested packages (refactor incrementally; do not churn existing M1 just for a directory tree):

```text
cmd/fleetwatch/main.go                 command entrypoint
internal/core/{model,validate,compare}.go
internal/source/{source,rpc,mock}.go    bounded current-state capture
internal/ingest/{reader,normalize,replay}.go
internal/store/{postgres,migrations}.go
internal/monitor/{detector,watcher}.go
internal/policy/{unsigned,signed,trust}.go
internal/alert/{outbox,webhook}.go
internal/api/                        optional read-only API
internal/config/                     explicit typed configuration
pkg/report/                          stable schema serializers if public SDK needed
contracts/testfixtures/              optional Rust Testnet only
examples/                            snapshots, config and local webhook sink
schema/                              versioned report / input JSON Schemas
docs/{adr,research,guide,runbook,build,wave}/
scripts/                             reproducible demos and issue generator
.github/workflows/                    build, tests, release candidate dry run
```

Proposed boundary types, **not verified Stellar library APIs**:

```go
type Source interface {
    Network(ctx context.Context) (NetworkIdentity, error)
    Capture(ctx context.Context, watchlist []ContractID) (Capture, error)
}

type LedgerReader interface {
    Next(ctx context.Context, from uint32) (LedgerDelta, error)
}

type Checkpointer interface {
    Load(ctx context.Context, network string) (Cursor, error)
    CommitLedgerAndEvents(ctx context.Context, delta LedgerDelta, events []Event) error
}

type AlertSink interface {
    Deliver(ctx context.Context, alert Alert) (DeliveryReceipt, error)
}
```

Keep `core.Compare` side-effect free. Network requests, database writes, policy signature verification, and webhooks belong in their own packages.

## 2. Identifiers, coverage and evidence

- **Network ID:** canonical network passphrase or verified network identifier; never just `TESTNET` as an unverified string in production.
- **Contract ID:** validated with a supported Stellar address library; track key by network + contract.
- **External ref:** owner contract address + raw `SCString` bytes in lowercase hex. Must preserve NUL, invalid UTF-8, case and byte boundaries.
- **Wasm hash:** 32 bytes. Represent in lowercase hex for output and as raw bytes internally where practical.
- **Source:** `synthetic`, `user_snapshot`, `rpc_current`, `ledger_meta` (or extensible enum). Include `source_instance`, observed ledger, expected range, cursor, collected-at timestamp, trust characteristics and capture error list.
- **Completeness:** `COMPLETE`, `PARTIAL`, `UNKNOWN`; additionally `ledger_sequence_consistent=true/false/unknown`, `fleet_scope=EXPLICIT_WATCHLIST|OBSERVED_NETWORK_SET|FULL_BACKFILL`. Never imply FULL_BACKFILL without verified source coverage.
- **Evidence:** ledger seq, transaction hash when present, contract data key, record digest, supporting RPC/ledger-meta identifiers, raw XDR digest, parser version and collection error. Protect source confidentiality.
- **Severity:** critical/high/warning/info but separately carry `confidence` and `evaluation_status`; severity is not proof of maliciousness.

Proposed normalized immutable event fields: `event_id`, `schema_version`, `network_passphrase`, `ledger_sequence`, `transaction_index` (nullable), `change_index`, `owner_contract`, `tag_hex`, `old_hash` (nullable), `new_hash` (nullable), `contract_ids_affected`, `observation_status`, `source`, `evidence_refs[]`, `derived_rules_version`, `created_at`.

Event ID key: deterministic hash of canonical `(network, ledger, tx idx or change idx, source ledger change ID, event kind, owner, tag, from, to)`, with uniqueness enforced by database index. The hash algorithm and serialization must be specified/tested. Don't include local wall-clock time in dedup identity.

## 3. CAP-85 decoding contract

Decode contract instances with executable variants:

- `CONTRACT_EXECUTABLE_WASM` → direct 32-byte hash.
- `CONTRACT_EXECUTABLE_STELLAR_ASSET` → asset executable (no wasm).
- `CONTRACT_EXECUTABLE_EXTERNAL_REF` → owner + tag bytes; lookup of the owner's **persistent** contract data entry keyed by `SCV_EXECUTABLE_TAG(tag)` gives Wasm hash.

Check and cite actual current XDR type names against official CAP-85 and Go SDK at implementation time. No dynamic JSON string-to-enum parser in place of typed XDR. A direct Wasm to external-ref update and vice versa must be represented as changes to the contract executable identity, separate from an owner's tag-value change.

A single RPC `getLedgerEntries` result is **not proof that a multi-call capture spans exactly one ledger**. Inspect response ledger metadata, read API consistency guarantees and fail closed on mixed heights. Use a source providing single-ledger state or historical changes for definitive transitions. `getLedgerEntries` is not historical state lookup.

## 4. Core detection rules

| Finding code | Trigger | Minimum proof | Action |
|---|---|---|---|
| `REFERENCE_CODE_CHANGED` | Same network+owner+tag observed with hash A→B | Consistent complete ledger observation/transition | Record; calculate affected watched instances |
| `UNAPPROVED_REFERENCE_CHANGE` | Verified observed A→B and no matching trusted signed approval | Complete transition, verified trusted approvals evaluated | Critical if policy requires approvals; otherwise policy-dependent |
| `POLICY_MATCHED_REFERENCE_CHANGE` | Exact tuple matches unsigned policy file | Complete transition + unsigned policy | Informational with `authorization=UNVERIFIED` |
| `APPROVED_REFERENCE_CHANGE` | Exact tuple + valid signature/trust/time/network | Complete transition + cryptographic approval | Informational with proof refs |
| `CONTRACT_EXECUTABLE_REPOINTED` | Instance changes direct hash/external owner+tag | Proof of contract instance state transition | High; classify precisely |
| `OBSERVATION_GAP` | Missing ledgers/entries/member or mixed heights | Expected range + failed coverage check | Warn/high; suppress safe verdict |
| `SOURCE_DIVERGENCE` | Two trusted endpoints disagree for same claimed ledger | Independent same-height evidence | High; quarantine until reconciled |
| `UNRESOLVED_EXTERNAL_REF` | Owner/tag cannot be resolved | Source record + lookup attempt | High/unknown, not deletion |
| `NEW_REFERENCE_OBSERVED` | New reference enters watch scope | Source record | Informational; no inferred prior version |
| `CONTRACT_NOT_OBSERVED` | Monitored instance absent | Watchlist vs observed state | Observation warning, never presumed destroyed |

Version rule IDs. Detect transient A→B→A only from sequential deltas; snapshot polling alone cannot guarantee this.

## 5. Storage (illustrative relational schema)

```sql
networks(id, passphrase_hash UNIQUE, display_name, created_at)
source_cursors(network_id, source_id, ledger_sequence, checkpoint_hash, updated_at, UNIQUE(network_id,source_id))
ledger_observations(network_id, ledger_sequence, source_id, observed_at, completeness, raw_digest, UNIQUE(network_id,ledger_sequence,source_id))
contract_executables(network_id, contract_id, valid_from_ledger, valid_to_ledger NULL, kind, wasm_hash NULL, owner_contract NULL, tag_hex NULL, evidence_id)
reference_versions(network_id, owner_contract, tag_hex, valid_from_ledger, valid_to_ledger NULL, wasm_hash, evidence_id)
findings(id UNIQUE, network_id, ledger_sequence, rule_code, severity, confidence, payload_json, created_at)
approval_records(id, digest UNIQUE, signer_id, envelope, valid_from, valid_until, status)
alert_outbox(id UNIQUE, finding_id, sink_id, status, attempt_count, retry_at, created_at)
alert_deliveries(id, outbox_id, attempt, response_class, delivered_at)
```

Implement migrations and foreign keys based on measured query requirements, not by copying this pseudo schema as-is. Use transaction boundaries so a checkpoint cannot advance until the relevant state changes, findings and outbox messages are durably committed. Never permit two active validity intervals for the same `(network, contract)` or `(network,owner,tag)` after replay reconciliation.

## 6. Approval security

Stage S0/M1 `Policy` is unsigned and **not trusted**. In S6, introduce a versioned signed envelope. Suggested signed statement binds `network_passphrase_hash`, `owner_contract`, `tag_hex`, `from_hash`, `to_hash`, `ticket`, `reviewed_artifact_digest`, `valid_from_ledger`, `valid_until_ledger`, and `nonce/release_id`, signed under an explicit trust root. DSSE/in-toto is a candidate, not predetermined; verify its libraries and use canonical serialization. Define key revocation effective dates and rotation, clock/ledger skew behavior, replay protection, and signature thresholds. Clear output:

- `policy_match`: yes/no
- `signature_verified`: yes/no/unknown
- `signer_trusted`: yes/no/unknown
- `approval_valid_for_ledger`: yes/no/unknown
- `final_verdict`: approved/unapproved/undetermined

`Undetermined` must not collapse to `approved`. Provide operator guidance when signer trust cannot be resolved.

## 7. Reliability and operational metrics

Core indicators: last ingested ledger vs latest known, observed gap duration and count, partial capture rate, RPC errors by code, median/95th pipeline latency, total transitions by type, outbox age, delivery failure count and DB replication/backup health. Choose targets **after benchmarking and deployment sizing**, not arbitrary guaranteed SLAs. Metrics labels must avoid unbounded contract IDs and tags. Avoid logging raw auth tokens, full signed envelopes or personally identifying data from unrelated accounts.

## 8. CLI behavior and exit statuses

Preserve M1 existing working command behavior unless a migration is documented. Proposed:

```sh
fleetwatch version
fleetwatch validate --input testdata/before.json
fleetwatch compare --before testdata/before.json --after testdata/after.json --policy testdata/approved-policy.json --format json
fleetwatch capture --network testnet --contracts-file examples/watchlist.txt --output capture.json
fleetwatch replay --network testnet --from-ledger N --to-ledger M --source ledger-meta
fleetwatch watch --config examples/fleetwatch.example.yaml
fleetwatch doctor --config examples/fleetwatch.example.yaml
```

Stable suggested exit categories: `0=success/no matching finding`, `2=alert threshold reached`, `3=invalid input/config`, `4=source unavailable/incomplete`, `5=internal error`. First check what M1 actually does; define a compatibility map before changing. Never let `--fail-on=invalid` silently act as `critical`.

## 9. Hosting topology

Read-only services only; no privileged Stellar keys in watcher:

```text
Stellar RPC / ledger meta → fleetwatch worker → Postgres (private network)
                                                ├→ alert outbox → signed HTTPS webhooks
                                                └→ read-only CLI/API (authenticated)
MkDocs static documentation → static hosting (optional; independent)
```

Choose actual vendor after confirming required availability, cost and permissions; do not deploy services or create paid resources by default. Define TLS, DB credentials, network ACLs, backups and metrics before public API exposure.
