# Inspecting offline state

`doctor` opens an existing local database read only and checks one exact
network/source scope. Stop the replay or delivery process before running it:
bbolt's file locks can prevent inspection while another process holds the file.
Lock acquisition times out after one second. Use a trusted local database path.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch replay --journal testdata/replay-transient.json --state demo.db
# exit 2: the synthetic sequence contains critical findings
./fleetwatch doctor --state demo.db --network TESTNET-SYNTHETIC
# exit 0: OK_OFFLINE, processed_through=102, two pending alerts
```

The JSON report includes the retained cursor, supplied sequence coverage, event
count, latest replay issues, and outbox counts; see [health-v1.json](../schema/health-v1.json).
Inspection recomputes retained
findings and checks their correspondence with the outbox in a single read
transaction. Missing event or current-issue alerts, changed event payloads,
invalid issue identities, and malformed coverage diagnostics fail inspection.
This checks local consistency, not tamper-proof provenance: an attacker able to
rewrite the complete database can replace the observations and recompute it.
Snapshot-mode approval receipt, nonce and recovery audit indices are outside this command's checks.

`OK_OFFLINE` means the retained supplied sequence is complete and there are no
dead letters, expired leases, or exhausted unsettled attempts. It does not mean
there were no security findings, that alerts were delivered, or that a live
watcher is running. Pending alerts are normal for the explicit, separate
`deliver` command; `doctor` does not send, retry, acknowledge, or reclaim them.

`outbox` counters are defined as follows:

| Counter | Meaning |
|---|---|
| `total` | All retained alerts, including historical gap alerts |
| `pending` | Alerts in PENDING state |
| `due` / `scheduled` | Pending retry time is at/before / after the inspection clock |
| `inflight` | Alerts with a persisted lease, including expired leases |
| `expired_leases` | INFLIGHT lease deadline is at/before the inspection clock |
| `delivered` / `dead` | Alerts acknowledged / abandoned by the dispatcher |
| `exhausted_unsettled` | Pending or expired INFLIGHT records at the attempt limit |

Due/scheduled partition pending; expired leases are a subset of inflight.
A valid fifth delivery lease is allowed to finish before it is considered
exhausted. Time classifications use the local clock, with no clock synchronization
claim. No alert creation timestamp is retained, so outbox age is unavailable.
Lease tokens, sink addresses, raw envelopes, tags, and contract IDs are omitted.

After a replay gap is repaired, its historical outbox record remains visible in
the totals. It does not cause `INPUT_SEQUENCE_INCOMPLETE` once the latest replay
report is complete; a dead historical gap alert still needs delivery attention.

All reports retain `chain_history=UNKNOWN`, `evaluation_status=UNVERIFIED_INPUT`,
`final_verdict=UNDETERMINED`, and `ledger_lag=null`. This command has no RPC
endpoint or independently authenticated latest ledger and cannot measure live
freshness, comprehensive history, throughput, backup health or availability.
It is a local diagnostic, not an HTTP metrics service or a production readiness
gate.

| Exit | Meaning |
|---|---|
| 0 | Local scope OK_OFFLINE (security findings may still exist) |
| 2 | Delivery needs attention |
| 3 | Invalid arguments |
| 4 | Scope absent or supplied input sequence incomplete; takes precedence over 2 |
| 5 | Database missing, locked, unrecognized, inconsistent, or output failed |

Missing files are never created. A scope absent from a recognized database
produces a `NO_STATE` JSON report with exit 4. Unavailable or invalid databases
produce no success report. This check does not perform page-level database
recovery, verify external backups, or modify the retained state.


## Reconstruction stream inspection

```sh
fleetwatch reconstruct --baseline testdata/reconstruction-baseline.json \
  --capture testdata/reconstruction-capture.json --state reconstruction.db --stream demo
# exit 2: synthetic findings retained and queued
fleetwatch doctor --state reconstruction.db --network SYNTHETIC-NETWORK \
  --source rpc_ledger_meta --stream demo
# exit 0: OK_OFFLINE, retained ledger 100, two pending alerts
```

This mode has a separate [reconstruction health schema](../schema/reconstruction-health-v1.schema.json).
Snapshot-mode health JSON stays unchanged. The reconstruction report includes
retained ledger/event counts, the latest attempt summary, historical diagnostic
count, delivery counters and aggregate recovery receipt/recovered-alert counts.
The last attempt's status and code describe the latest offline invocation, not
current source availability. When only a failed first attempt exists,
`state_found=false` and `processed_through` is its asserted baseline ledger;
`retained_ledgers=0`. An absent scope instead has cursor 0 and no last attempt.

A single read transaction rederives the retained projection from the original
baseline and raw evidence, checks checkpoint/attempt/outbox agreement, and
validates each recovery sequence against its alert's cycle, payload digest and
bound sink. Receipt inspection scans the bounded queue audit once. Missing,
orphaned, duplicate or inconsistent receipts fail inspection. Recovery JSON
rejects duplicate/case-aliased members, unknown fields and trailing data. These
are local consistency checks; a writer able to replace the complete DB can
replace the evidence and audit. They do not establish operator identity.

The command does not claim leases, write attempt markers or send alerts. No
lease tokens, bound sink values, individual alert payloads, operator request IDs
or reasons appear in the health report. The latest attempt retains its bounded
scope/range/digests and fixed diagnostic code. Revalidation has a 30-second CLI
context deadline, in addition to the one-second file-lock timeout.

`OK_OFFLINE` requires an integrated checkpoint and a COMPLETE latest attempt,
with no dead letters, expired leases or exhausted unsettled attempts. Findings
and pending delivery do not themselves change this status. A failed extension
leaves `state_found=true` for the accepted prefix, but reports
`LATEST_ATTEMPT_INCOMPLETE` and exit 4. Repairing the source clears that reason;
historical diagnostic rows remain in the totals and their dead letters still
require delivery attention. Explicit [requeue](ALERT_RECOVERY.md) can clear a
dead letter without changing its evidence or sending it.

Old checkpoints without outbox integration report `ALERTS_NOT_INTEGRATED` and
`LATEST_ATTEMPT_UNKNOWN`, with exit 4. Opening them read-only does not backfill
anything; the next explicit reconstruction call performs the existing backfill.
An absent scope reports `NO_STATE` / `NO_RECONSTRUCTION_STATE`; a failed first
attempt reports `NEEDS_ATTENTION` with that reason and
`LATEST_ATTEMPT_INCOMPLETE`. Missing or incomplete latest attempts take exit 4
precedence over delivery attention (exit 2). Invalid state or audit produces no
success report and exits 5. Output failure also exits 5.

Every reconstruction health report says `coverage=RETAINED_PREFIX_ONLY`,
`source_freshness=UNKNOWN` and `ledger_lag=null`. The assurance block keeps
baseline/source authentication UNVERIFIED, metadata commitment NOT_VERIFIED,
membership/history completeness UNKNOWN and final verdict UNDETERMINED.
`OK_OFFLINE` is not a live health probe or a chain-safety verdict. Continuous
worker telemetry and independently authenticated chain evidence remain unfinished.
