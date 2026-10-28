# Durable diagnostic reconstruction checkpoints

`reconstruct` can retain a bounded reconstruction stream in the existing local
bbolt database. The original asserted baseline, accepted raw metadata prefix,
projected state, groups and findings commit in one synchronous transaction.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch reconstruct --baseline testdata/reconstruction-baseline.json \
  --capture testdata/reconstruction-capture.json \
  --state /workspace/reconstruction.db --stream operator-demo
# exit 2: accepted synthetic ledger 100 with two findings

# A separate invocation resumes from retained raw evidence, using the ORIGINAL baseline.
./fleetwatch reconstruct --baseline testdata/reconstruction-baseline.json \
  --capture testdata/reconstruction-next-capture.json \
  --state /workspace/reconstruction.db --stream operator-demo
# exit 2: cumulative report through ledger 101, three findings

./fleetwatch checkpoint --state /workspace/reconstruction.db \
  --network SYNTHETIC-NETWORK --stream operator-demo
# exit 0: found=true, coverage=RETAINED_PREFIX_ONLY, report.processed_through=101
```

The fixtures are synthetic model/XDR inputs, not consensus-valid ledgers, real
contract upgrades or production baseline evidence. The second capture repoints
one watched instance to direct Wasm. No RPC calls, transactions or deliveries are
performed by these commands.

## Scope and restart behavior

`--state` and `--stream` must be supplied together. Stream IDs are 1–128 ASCII
letters, digits, dots, underscores or hyphens. A database scope is the exact
network passphrase plus stream ID, keyed by SHA-256 of a versioned tuple under
`fleetwatch.watched-reconstruction.v1`. A stream is bound to its original
canonical baseline snapshot digest and header pin when its first valid attempt
commits, including a failure-only diagnostic before any prefix is accepted.
A different baseline is rejected; use a separate stream for an intentional new
assertion. Equivalent snapshot list order/hex case does not silently create a
different baseline identity.

Stream IDs separate local storage. They do not identify an authenticated provider
or make copied input into independent sources. Event IDs remain evidence/baseline
bound, so two aliases of identical evidence can have identical event IDs.

Later captures can overlap the retained prefix or start at its next ledger. Exact
overlaps are idempotent; any changed metadata or descriptor at an accepted ledger
is rejected without changing the stream. Matching heights or header hashes alone
are not enough to replace raw metadata. Captures cannot prepend an earlier
baseline, skip an unknown gap or change networks within a stream.

Loading a checkpoint strictly decodes its record, revalidates the raw capture and
reconstructs it from the **original baseline**. The complete recomputed report
must match the stored report. Reconstructing from only the final active snapshot
would lose previously observed references that may be needed for a later repoint;
retained raw evidence recreates that cache and known-absence state on restart.
This checks local consistency, not tamper-proof provenance: an attacker able to
replace the entire database and its evidence can recompute a consistent record.

## Accepted prefix versus failed attempts

The command's JSON report describes the current merged attempt. A bad later
ledger, missing suffix or incomplete source returns exit 4 with a PARTIAL report.
Any newly accepted prefix is still saved atomically. Raw records beyond the first
missing or failing ledger are not retained, even if supplied: resend that suffix
after repairing the gap. A source failure whose requested range was already cached
still returns PARTIAL / SOURCE_ATTEMPT_INCOMPLETE rather than hiding the failed
attempt behind old evidence.

The stored record contains **only the accepted contiguous prefix** and a canonical
complete report for that smaller range. Full failed attempt bodies are not stored.
The [reconstruction outbox](RECONSTRUCTION_ALERTS.md) now retains latest-attempt
summaries and historical diagnostics separately in the same transaction.
`checkpoint` therefore always labels its accepted report
RETAINED_PREFIX_ONLY: a COMPLETE_CAPTURE_APPLICATION stored report does not mean
the latest request succeeded or the source is currently fresh. This is not a
continuous watcher health check, source-lag metric, or gap alarm service.

The [checkpoint JSON Schema](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/schema/reconstruction-checkpoint-v1.schema.json)
wraps the existing reconstruction report without exposing full raw ledger
metadata. `found=false` has a null report and exit 4. Missing/locked/unrecognized
or inconsistent database returns 5 without a success report. Inspection opens
the file read only, never initializes a missing file, and preserves its bytes.

Reconstruction events remain separate from snapshot replay and its `bind-approval`
command. Inspect their outbox with `alerts --source rpc_ledger_meta --stream ID`
and explicitly dispatch with the same options on `deliver`. `doctor` still covers
the older snapshot workflow. No authorization is upgraded by storage or HMAC.

## Failure, limits and recovery

Normal bbolt locking/fsync/local-filesystem requirements apply. New files use mode
0600. No transaction can advance the persisted report cursor without its raw
evidence, projected state and findings. An injected abort, cancellation just before
commit or process exit during the transaction leaves the previous committed
checkpoint available. Corrupt derived verdicts or changed retained evidence fail
revalidation rather than being trusted. Hardware power-loss durability and hostile
filesystem behavior have not been independently verified.

Limits apply across the accumulated stream: 16 ledgers from the original baseline,
the existing capture/normalizer/reconstruction budgets, and 25 MiB combined encoded
baseline/capture/report per stream. A database allows at most 128 reconstruction
streams. Recompute runs in the transaction under the CLI's 30-second context and
can block other writers; this bounded local prototype is not production backfill
storage. No pruning, automatic baseline rollover or migration is implemented.
Keep the original database when upgrading parser/rule versions: changed derivation
can require an explicit migration, not deletion of retained history.

Reconstruction exit categories remain 0 (complete/no findings), 2 (complete with
findings), 3 (invalid input/usage), 4 (incomplete application or refused scope/
overlap/range/storage bound), and 5 (database/output failure). Refused writes can
return only stderr rather than a JSON report. An output or storage error can
occur after commit; inspect `checkpoint` and safely retry exact inputs instead of
assuming nothing was saved. The checkpoint inspector exits 0 for a found and
validated prefix, 3 for invalid arguments, 4 for an absent scope, and 5 for an
unavailable/invalid database or output failure. Its exit does not evaluate findings.

Use the [backup command](BACKUP.md) to preserve all namespaces together. Recovery
tests verify that raw reconstruction evidence and its non-active reference cache
survive backup/restore. Old backups still lack subsequent observations, approval
reservations or delivery acknowledgments; do not run original/restored copies as
independent active histories. Record network and stream identifiers with the
operator's recovery inventory. Full raw metadata can contain unrelated public
transaction data, so keep database and backups in operator-controlled storage.

All outputs keep UNVERIFIED baseline/source authentication, NOT_VERIFIED metadata
commitments, UNKNOWN membership/history completeness and UNDETERMINED final
verdicts. Durable storage adds recovery, not independent chain authentication.
Production retention, operator dead-letter recovery, signed-policy integration,
continuous ingestion and controlled live upgrade proof
remain unfinished.


Use [ingest](INGESTION.md) to fetch a missing bounded RPC range directly into the
retained stream. It checks the expected cursor again before committing, retains
source/application diagnostics and preserves the original baseline and range bound.
