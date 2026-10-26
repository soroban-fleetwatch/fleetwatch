# Bounded resumable RPC ingestion

`ingest` connects the existing read-only RPC history collector to durable watched
reconstruction. It plans from a validated local cursor, fetches only the missing
range through an explicit target, and atomically retains the accepted raw prefix,
projected findings, latest attempt and alert outbox. It does not send alerts.

```sh
fleetwatch ingest --rpc-url https://soroban-testnet.stellar.org \
  --baseline watched-baseline.json --state watched.db --stream testnet-demo \
  --to-ledger TARGET
fleetwatch doctor --state watched.db --network 'Test SDF Network ; September 2015' \
  --source rpc_ledger_meta --stream testnet-demo
```

The baseline must describe the watched fleet at the immediately preceding ledger
and include its operator-asserted header pin. It must match the network claimed
by `getNetwork`. Do not use the repository's synthetic baseline against a real
network. Preparing an independently authenticated real baseline is still
unfinished. See [baseline and projection limits](RECONSTRUCTION.md).

The target must be after the original baseline and at most **16 ledgers beyond
it**, including retained ledgers. This is a finite diagnostic operation, not an
unbounded `watch` service. Repeating calls does not expand that retention bound.
There is no automatic baseline rotation, pruning or migration to bypass it.
All existing per-stream evidence, scope and outbox bounds still apply.

A new stream starts at baseline + 1. A resumed stream starts at its validated
retained cursor + 1; rejected input is fetched again. The original baseline
snapshot digest, ledger and header pin remain bound even when a failed first
attempt produced no checkpoint. Planning validates retained raw evidence and
queue/latest-attempt correspondence before any source call. The write transaction
checks the expected cursor again: if another same-process operation advanced it
during the fetch, the stale batch is refused without changing that operation's
checkpoint or latest attempt. Replan and fetch again. Other processes normally
cannot open the DB concurrently because bbolt holds the exclusive file lock.

RPC requests retain the [history collector's](HISTORY.md) exact start-ledger,
network/protocol, retention, bounded XDR and header-link checks. Projection also
checks the link to the asserted baseline and across a resumed batch boundary.
The endpoint is HTTPS without URL credentials/query/fragment, with certificate
verification, disabled redirects and inherited proxy/CA routing. Do not unset
proxy variables or disable TLS verification to reach a denied endpoint.

The JSON [result](../schema/ingestion-result-v1.schema.json) reports the requested
target, starting/retained cursor, actual fetch start/count, source query coverage,
latest attempt status, fixed collector errors and attempted reconstruction report.
`source_query=COMPLETE_ENDPOINT_SEQUENCE` does not mean projection succeeded.
A valid RPC response can still fail baseline linkage or state application.
`fetched_ledgers` counts decoded capture records, including any rejected during
projection; only `processed_through` describes accepted progress.

A source or application failure can retain a valid prefix and a diagnostic,
return `status=PARTIAL`, and exit 4. Repeating the target resumes from that
prefix. Failed first attempts keep a bounded diagnostic without fabricating a
checkpoint. Historical diagnostics remain after repair. Cancellation or a
storage failure returns no success report; the existing transaction rules prevent
a partial commit. SIGINT/SIGTERM cancel CLI work. The operation has a two-minute
overall context deadline, with 30-second planning and commit/revalidation deadlines;
RPC calls also have existing request timeouts and bounded transient retries.

If the target is already retained (including an older target), the command
returns `status=ALREADY_RETAINED`, `source_query=NOT_QUERIED`, `fetch_from=0`,
zero fetched records and `report=null`. It performs no RPC query and no DB write.
It preserves the observed latest-attempt status, including PARTIAL for a failed
extension beyond that target. It does not backfill legacy checkpoints or clear
failed-attempt markers merely because the target is already present. Use `doctor`
for the latest local health result. An ALREADY_RETAINED result means only that
the selected target was retained when inspected, and exits 0.

| Exit | Meaning |
|---|---|
| 0 | Completed with no findings, or selected target already retained |
| 2 | Completed reconstruction contains findings (including retained earlier findings) |
| 3 | Invalid CLI input, baseline, target range, stream or endpoint |
| 4 | Source/application incomplete, baseline/cursor conflict, range bound or cancellation |
| 5 | Database/validation failure or output failure |

Output failure can occur after a commit. Inspect `doctor` or repeat the same
target; progress and stable alert IDs are retained, so alerts are not duplicated.
Only the explicit `deliver` command contacts a webhook receiver.

All output retains baseline/source UNVERIFIED, metadata commitments NOT_VERIFIED,
membership/history completeness UNKNOWN and final verdict UNDETERMINED.
Source errors contain fixed codes, not raw endpoint/credential/error strings.
Neither a COMPLETE result nor linked headers authenticate consensus, metadata
commitments, historical fleet membership or current source freshness. No new
approval binding, signing keys or on-chain transaction capability is introduced.
Continuous worker configuration, production retention/telemetry and authenticated
chain evidence remain unfinished.
