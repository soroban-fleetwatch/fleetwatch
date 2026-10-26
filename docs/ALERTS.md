# Durable offline alert outbox

Durable replay now enqueues findings and replay gap issues in the same bbolt
transaction as observations and cursor. It does not automatically send messages.

```sh
/workspace/.tools/bin/fleetwatch replay \
  --journal testdata/replay-transient.json --state /workspace/replay.db
# exit 2; two cumulative findings, two queued alerts
/workspace/.tools/bin/fleetwatch alerts --state /workspace/replay.db \
  --network TESTNET-SYNTHETIC --source synthetic
# exit 0; two PENDING alerts, no delivery performed
```

The alerts command requires an existing FleetWatch state database. State remains
scoped by exact network and source. Existing databases need no destructive rewrite:
replaying a journal seeds missing queue entries, while retaining existing delivery
state. No automatic backfill occurs merely by opening a file. Ordinary stateless
replay does not persist alerts.

Finding alerts use the deterministic replay event ID. Gap alerts hash a versioned
record of domain, network, source and exact issue range. Repeated equivalent replay
does not requeue delivered alerts. Input remains UNVERIFIED_INPUT, and finding
authorization UNVERIFIED. A queued item is not proof of malicious chain activity.
Stored conflicts abort replay; conflict delivery is not implemented. Old queued
gap incidents remain historical observations after the gap is repaired; there is
no resolution-notification workflow yet.

The dispatcher API accepts an injected sender. It claims one persisted lease,
increments the attempt count, sends with a 15-second context timeout, then resolves
the lease. Senders must honor context cancellation. Leases last 30 seconds and
use fresh random fencing tokens; stale workers cannot acknowledge a newer lease.
A lease left by a crashed worker can be reclaimed when expired. Transient failures
wait 2/4/8/16 seconds before another attempt; five total attempts lead to DEAD.
Permanent failures lead directly to DEAD. The first claim binds an alert to a sink
ID; changing that ID for a pending item requires an explicit future routing tool.

Delivery is at least once. If a receiver accepts a request but the process crashes
before acknowledgement is saved, a retry carries the same alert ID. Receiver-side
idempotency is required; exactly-once remote delivery is not claimed. Error details
from senders are replaced with fixed failure codes. Lease tokens are omitted from
CLI output, and no signing key or credential is stored in queue entries.

Limits: 10,000 queued records per network/source, 1 MiB encoded payload per alert,
and batches of at most 50. Hitting a queue/payload bound aborts the entire replay
transaction, so its cursor cannot advance past an unsaved alert. Delivered rows are
retained; automatic retention/pruning is not implemented.

Tests use simulated senders and cover atomic rollback, enqueue deduplication,
durable gaps, scope isolation, lease recovery/fencing, retry backoff/dead letters,
sink binding, cancelled dispatch, error redaction, and delivery with a lost
acknowledgement. The process-crash test also verifies no uncommitted queue rows.

Experimental direct HTTPS/HMAC transport and the separate `deliver` command are now available; see WEBHOOKS.md for strict destination controls and unsupported proxy routing. Continuous worker, operator dead-letter recovery, metrics and production Postgres migration remain unimplemented. No external webhook has been contacted during development.
