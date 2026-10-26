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
Approval receipt and nonce indices are outside this command's checks.

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
