# Bounded polling worker

`watch --config FILE` retries bounded ingestion toward a fixed ledger target. It
resumes from persisted reconstruction evidence and stops when the target is
retained, the retry/deadline budget is exhausted, or source/application evidence
is rejected. It never sends webhook notifications.

```sh
fleetwatch watch --config watch.json > progress.ndjson
```

The strict JSON configuration has no defaults; all fields are required:

```json
{
  "schema_version": 1,
  "rpc_url": "https://rpc.example.invalid",
  "baseline": "watched-baseline.json",
  "state": "watched.db",
  "stream": "testnet-demo",
  "to_ledger": 101,
  "poll_interval_seconds": 5,
  "max_attempts": 3,
  "max_duration_seconds": 60
}
```

Replace the placeholder endpoint, baseline and target with matching evidence.
Paths resolve relative to the configuration file's directory, including when
invoked from another working directory. State must differ from the baseline and
configuration paths. No environment expansion, signing keys, receiver URLs or
delivery options are supported. Configuration is limited to 16 KiB; unknown,
duplicate/case-aliased fields, trailing JSON, null/missing required values and
invalid bounds fail before a state DB is opened. HTTPS endpoint restrictions and
TLS/proxy behavior match [ingest](INGESTION.md).

The [synthetic example](../examples/watch.synthetic.json) references the repository
baseline at ledger 99 and targets 101. Its `.invalid` endpoint is deliberately a
placeholder; use a local synthetic RPC fixture, never a real network with that
baseline. An independently authenticated real baseline remains unfinished.
See the [configuration schema](../schema/watch-config-v1.schema.json).

Polling intervals must be 1–60 seconds, attempts 1–120, and the overall worker
runtime 1–3,600 seconds. The interval is a cancellable wait **after** an attempt,
not a promise of wall-clock query frequency. Each attempt retains ingestion's
two-minute limit and 30-second planning/commit bounds, capped by the worker's
remaining deadline. File/configuration loading and DB opening precede the worker
runtime budget. SIGINT/SIGTERM cancel ingestion and waits. A timed-out attempt
stops; it does not silently start an additional retry.

Only bounded availability failures retry: NETWORK_QUERY_FAILED,
LEDGER_QUERY_FAILED, RETENTION_RANGE_UNAVAILABLE or MISSING_OR_UNEXPECTED_LEDGER.
Every decoded source record must have been accepted by projection; a partially
rejected prefix stops instead. Missing ranges may be not-yet-produced or already
pruned; retries do not establish availability and cannot bypass retention. Wrong
networks, unsupported protocols, invalid metadata, header divergence and state
application failures stop immediately. A cursor conflict during fetch retries by
replanning from the new retained cursor within the same attempt budget. A DB or
validation failure stops without retry. No raw upstream error text appears in
progress output.

Output is one [progress JSON object](../schema/watch-progress-v1.schema.json) per
line, at most 121 lines per run. Each attempt yields a RETRYING, COMPLETE,
ALREADY_RETAINED or STOPPED record. Cancellation/deadline during a wait can emit
one final STOPPED record without another ingestion attempt. A broken output
stream stops work immediately; committed progress can already exist. Inspect
`doctor` after stopping, or restart with the same configuration. Existing stable
finding/diagnostic IDs prevent duplicate outbox records.

Counters reset on process restart. They describe completed attempt results,
not durable metrics or all network requests:

| Field | Meaning |
|---|---|
| `sequence` | Monotonic output-record number within this run |
| `attempts` | Ingestion calls made, including partial/conflicted/failed calls |
| `partial_attempts` | Returned results marked incomplete |
| `cursor_conflicts` / `failed_attempts` | Cursor races / other hard call failures |
| `fetched_ledgers` | Decoded records reported by returned ingestion results; failed commits may have fetched unreported records |
| `accepted_ledgers` | Sum of cursor advances made by this run's returned results |
| `processed_through` | Last observed retained cursor, null before any usable result |
| `target_remaining` | Fixed target minus last observed cursor, clamped at zero; null if cursor unknown |
| `observed_events` | Event count in the last reconstruction report seen by this process, null until a report is returned |
| `latest_attempt_status` | Last observed durable attempt status; UNKNOWN until a usable result |
| `source_query` / `fetch_errors` | Coverage/fixed codes from the current returned result; UNKNOWN/empty when a call returns no result |
| `elapsed_milliseconds` | Process-local elapsed time, not ingestion latency or an SLA |

A fresh run against an already-retained target queries no source, writes no
checkpoint, reports `ALREADY_RETAINED` and leaves `observed_events=null`. It
preserves the observed latest-attempt status, including PARTIAL for an extension
beyond that target. It does not clear failed diagnostics or backfill legacy
queues. This can exit 0 even if broader local health still needs attention; use
[doctor](HEALTH.md) after the process closes its DB.

bbolt holds the exclusive local DB lock throughout polling, including waits.
Concurrent CLI inspection/delivery/backup normally cannot open it; lock acquisition
times out after one second. Stop the worker before those maintenance commands.
Progress output omits endpoint URLs, paths, raw payloads, contract IDs/tags,
receiver details, operator reasons and lease tokens. Delivery remains a separate,
explicit `deliver` command, with its existing transport restrictions.

| Exit | Meaning |
|---|---|
| 0 | Target already retained, or completed with no observed findings |
| 2 | Target completed and this process observed reconstruction findings |
| 3 | Invalid configuration, baseline, scope or target |
| 4 | Attempt limit, cancellation/deadline, or rejected/incomplete source/application evidence |
| 5 | State/validation/storage or progress output failure |

The original asserted baseline still limits the complete retained interval to
**16 ledgers**. Repeated polling never rotates a baseline or expands that limit.
This is a finite worker, not a continuously advancing production monitor. It
retains UNVERIFIED baseline/source, NOT_VERIFIED metadata commitments, UNKNOWN
membership/history/source freshness, UNDETERMINED verdict and null live ledger
lag. Target completion does not authenticate the chain or establish current
source health. Production retention, unbounded service operation, persistent
telemetry and authenticated chain evidence remain unfinished.
