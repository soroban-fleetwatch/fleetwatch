# Experimental continuous follow mode

`fleetwatch follow --config FILE` advances a fixed watched fleet through
consecutive endpoint-reported closed ledgers. It has no fixed target or process
duration. Each source batch still contains at most sixteen ledgers; explicit
retention and finding budgets stop the process before it discards evidence.
This is a single-host pilot, not production certification or authenticated chain
approval. The older `watch` command keeps its original sixteen-ledger lifetime.

```sh
fleetwatch follow --check --config follow.json
fleetwatch follow --config follow.json
# Optional local queue sync; no external dispatch:
fleetwatch follow --config follow.json --delivery-state delivery.db --delivery-stream pilot
```

`--check` validates only configuration and the operator-asserted baseline. It
does not create/open a database, call RPC, bind a port or establish readiness.
Run from any working directory: paths resolve relative to the configuration.
The state directory must exist and be writable. Use a new state file; this is a
separate versioned format and does not migrate a legacy replay/watch database.

All configuration fields are required, canonical and non-null. Unknown or
duplicate/case-aliased fields, oversized JSON and unsafe paths/listeners fail:

```json
{
  "schema_version": 1,
  "rpc_url": "https://soroban-testnet.stellar.org",
  "baseline": "baseline.json",
  "state": "follow.db",
  "listen": "127.0.0.1:9789",
  "poll_interval_seconds": 5,
  "batch_size": 8,
  "head_delay_ledgers": 1,
  "max_endpoint_lag": 4,
  "max_retained_ledgers": 100000,
  "max_retained_payload_bytes": 67108864
}
```

Use an actual baseline for that exact network, not the synthetic repository
fixture. Baseline creation/authentication remains an explicit operator task;
current RPC snapshots cannot prove atomic or independently authenticated state.
The header pin must match the first retained ledger's previous hash. The
process never replaces the baseline after a failure or silently skips a gap.
The [synthetic example](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/examples/follow.synthetic.json) uses a deliberately
invalid endpoint and is for configuration checks only.

Poll intervals are 1–60 seconds after a completed cycle; batch size 1–16;
head delay 0–8 endpoint-reported ledgers; maximum acceptable endpoint lag
head-delay-plus-one through 100,000. Each cycle has a two-minute deadline.
SIGTERM/SIGINT cancel source calls and waits. The endpoint must use HTTPS without
URL credentials/query/fragment; inherited proxy routing and OS TLS trust remain
enabled. Supported protocols remain 28–29.

## Durable continuity and retained data

The new follow store retains the original baseline, each accepted raw metadata
record and derived group/event data, the current complete projection, and a
finding journal. A transaction commits evidence, findings and cursor together.
Replaying the same history with different batch sizes derives identical IDs.
No new baseline is created at a batch boundary. Original baseline provenance
stays in event identity. Inactive reference values and known-absent reference
keys carry across batches, preserving preimage checks and eviction semantics.

At every restart, the store replays **all retained raw evidence from the original
assertion** and checks the head, counts, groups and journal payloads. A cached
projection does not replace this validation. Validation must finish within two
minutes; a large retained history may require a different storage design rather
than a guessed shortcut. Second writers time out on the exclusive bbolt lock.

Retention limits are 1–100,000 ledgers, 1–256 MiB of retained payload, and at most
10,000 findings. Byte accounting covers the original baseline, current full
projection, retained records and finding payloads. It does not cap bbolt page
allocation, filesystem overhead, process memory or journal logs. Monitor actual
disk space separately. A batch that exceeds a bound is rolled back as a whole.
No automatic pruning, baseline rotation, queue deletion or archive upload occurs.
Increasing configured budgets requires operator review; it is not a source-trust
upgrade. This version has no online compaction or production retention migration.
Offline [follow-compact](COMPACTION.md) can reclaim physical allocation in a new
file while preserving all evidence and operational counters. It does not change
logical payload/ledger/finding limits or free logical retention capacity.

Source availability errors retain any validated prefix and set not-ready.
Retries count durable **gap episodes**, not one gap per polling attempt. Corrupt
metadata, conflicting preimages, wrong networks, unsupported protocols or header
regression stop the service. A source failure never becomes a clean verdict.
Progress is NDJSON with fixed codes and counters; raw provider errors, endpoint
URLs, contract IDs/tags and signing material are omitted.

## Local health and finding inspection

Only an IPv4 loopback listener is supported. Port zero is useful for isolated
tests; production pilot configurations should choose a fixed unused high port.

| Route | Meaning |
| --- | --- |
| `GET /livez` | This process is serving HTTP; not a source or chain check |
| `GET /readyz` | 200 only after a recent successful source cycle, within configured endpoint lag and capacity bounds; otherwise 503 |
| `GET /status` | Operational JSON and explicit assurance, always 200 if serving |
| `GET /metrics` | Prometheus text, bounded metric names with no per-contract labels |

Readiness is for **experimental read-only ingestion**, not chain safety. The lag
is a claim from this endpoint, not independent global network lag. Stale or
unavailable head claims produce null lag; the corresponding metric is omitted.
New processes start not-ready even if the previous process was healthy.
The listener is not a public authenticated API. Do not expose it through a
public bind or proxy without separately implementing authorization/TLS controls.

Stop the process before offline maintenance:

```sh
fleetwatch follow-inspect --config follow.json --limit 20 > page.json
# If next_after_id is nonempty, use --after-id VALUE for the next page.
fleetwatch backup --state follow.db --output NEW_BACKUP.db
fleetwatch follow-archive --config follow.json --output NEW_EVIDENCE.ndjson.gz
fleetwatch verify-follow-archive --input NEW_EVIDENCE.ndjson.gz
```

Inspection revalidates the entire store, then returns the head, durable cycle and
a bounded page of findings sorted by stable ID. It does not create missing state
or query RPC. The existing backup command consistently copies all buckets; its
512 MiB physical-copy limit still applies. Restore only while stopped, keep the
old file, and validate the restored file with follow-inspect before restart.
An older restore loses subsequent findings/cursors; catch-up requires those
ledgers still be available from the source.

The [evidence archive](ARCHIVES.md) is independently replayable compressed
NDJSON, exported without changing the source database. Its verifier needs no
configuration, database or RPC call. It checks from the original assertion;
checksums do not authenticate chain evidence. `restore-follow-archive --input
FILE --output NEW_DB --sha256 EXPECTED_DIGEST` replays an archive into a separate
new store, with original evidence/findings and explicitly reset operational
counters. Archives do not prune, rotate or raise capacity. Keep database backups
and prior operational evidence for recovery; see the archive guide before use.

The follow finding journal is distinct from the delivery outbox. The offline
[follow-sync bridge](FOLLOW_DELIVERY.md) imports replay-validated findings into
a separate durable outbox, preserving delivery state across resync. Existing
`deliver --source follow_ledger_meta --stream ID` can send those imported alerts.
Manual follow-sync requires a stopped follower; it sends nothing itself. The
paired optional follower flags enable automatic queue sync between polls and
startup catch-up. Sync failure reports OUTBOX_SYNC_FAILED, not-ready, exit 5;
native evidence is retained. No receiver URL or signing secret is accepted in
the follow configuration. External dispatch scheduling and production
proxy-aware receiver routing remain separate work.

| Exit | Meaning |
| --- | --- |
| 0 | Requested graceful stop, successful check or offline inspection |
| 3 | Invalid input/configuration/baseline |
| 4 | Source/projection evidence rejected; inspect before restarting |
| 5 | State, startup, health-listener or output failure |
| 6 | Retention/finding capacity reached; inspect/archive before restarting |

All data retain baseline/source UNVERIFIED, metadata commitments NOT_VERIFIED,
membership/history UNKNOWN, projected-state evaluation and final UNDETERMINED.
Signed approvals, consensus proofs, full archive lifecycle support, shared-writer
Postgres, load sizing and production receiver delivery are not added here.
