# Follow findings and the durable delivery outbox

`follow-sync` connects replay-validated continuous follow findings to a separate
durable delivery database. It is an **offline operator step**, not automatic
dispatch inside the follow daemon. It sends no notification or RPC request.

Stop the follower, retain a backup, then sync and inspect locally:

```sh
fleetwatch follow-sync --config follow.json \
  --delivery-state delivery.db --stream pilot
fleetwatch alerts --state delivery.db \
  --network 'Test SDF Network ; September 2015' \
  --source follow_ledger_meta --stream pilot
```

Use a stable stream and a separate database path. Config/baseline/source path
collisions are rejected. Missing or replay-invalid follow state does not create
an outbox. A valid source can create a new delivery database; a later import
failure can leave that initialized database, with no partial imported batch.
Keep the database and receipts private. The sync command exits 0 after import
and receipt output, 3 for invalid arguments/config/baseline, and 5 for source,
lineage, manifest, capacity, state or output failure. It has a two-minute
cancellable deadline and never starts or restarts a service.

Sync opens the follower read-only and replays its retained raw history. It reads
the complete finding journal, binds the import to the exact network, original
baseline digest/header, stream and current ledger/header, and atomically merges
the finding set with an import manifest. Repeated sync preserves existing
pending, leased, acknowledged, retry and dead-letter state. Earlier imported
findings cannot disappear. A prior sync header must still occur in the native
source history: rewinds or a mismatch at that header are rejected. Concurrent
imports use an expected prior binding and cannot silently overwrite each other.

Finding IDs are hashes; a newly observed finding can sort before an older one.
Every sync scans all retained findings rather than using the last ID as an
incremental watermark. The adapter accepts at most 10,000 findings and 24 MiB
of serialized events; the retained manifest has a 25-MiB limit and each outbox
alert has a 1-MiB limit. These pilot limits can stop sync. A successful import
does not extend follow retention or permit deleting source evidence.

## Dispatch after configuring an authorized receiver

The existing sender can dispatch the imported queue independently after sync.
The follower may be restarted separately while delivery operates on its own
database. The example hostname below is a placeholder: replace it with your
authorized receiver and provision its HMAC secret through the host's supported
secret mechanism before invoking delivery.

```sh
fleetwatch deliver --state delivery.db \
  --network 'Test SDF Network ; September 2015' \
  --source follow_ledger_meta --stream pilot \
  --webhook-url https://hooks.example.invalid/fleetwatch \
  --allow-host hooks.example.invalid \
  --hmac-key-env FLEETWATCH_WEBHOOK_KEY --limit 10
```

The sender retains verified TLS, HTTPS port 443, exact public hostname allowlist,
public-address DNS checks, pinned dialing, redirect rejection, bounded requests
and responses, and exact-body HMAC signatures. Raw signing keys are 32–4096
bytes, read from the named environment variable and never included in outbox
payloads or receipts. No transport policy is relaxed by the follow source type.
Proxy-backed webhook routing is still unsupported and explicitly rejected;
do not disable a required proxy or its security controls to make delivery work.
Production receiver/proxy integration remains a separate hosting gate.

Delivery uses the existing fenced leases, retries, sink binding and dead-letter
handling. It is at least once: a receiver must deduplicate by the outer delivery
ID if a response or acknowledgement is lost. Resync never resurrects dead
letters or requeues acknowledged alerts. Explicit operator `requeue` and
`recoveries` commands accept the same `--source follow_ledger_meta --stream ID`
options and keep their existing audited recovery-cycle checks. Changing stream
or losing/restoring an older delivery DB can cause repeat delivery; preserve
the same DB/stream and reconcile recovery before dispatching again.

## Payload and assurance

The HTTP envelope remains schema 1. Its payload is schema 3 with
`source: follow_ledger_meta`, stream, fixed projected-state assurance and a
`reconstruction_event`. See [the schema](../schema/follow-alert-v3.schema.json).
Receivers must accept payload schema 3 before using this source; there is no
automatic downgrade to an older payload format.
The original follow event ID is retained inside that event. The outer delivery
ID hashes Go JSON of fields `Domain`, `Network`, `Stream`, `Event`, using domain
`fleetwatch.follow-alert.v1` and the complete canonical reconstruction event.
It is distinct from legacy reconstruction delivery IDs. HMAC and
`Idempotency-Key` bind the outer ID and complete envelope exactly as documented
in [WEBHOOKS.md](WEBHOOKS.md).

The import manifest preserves event claims and binding, not the source's raw
ledger metadata. The native adapter validates raw history before import;
queue inspection/claim checks every canonical payload against that retained
manifest. This checks local consistency, not independent chain authentication.
Baseline/source remain UNVERIFIED, metadata commitments NOT_VERIFIED,
membership/history UNKNOWN, finding authorization UNVERIFIED and final verdict
UNDETERMINED. HMAC authenticates delivery only.

The native follow journal remains immutable: its `pending_findings` counter is
the number of retained findings and is **not** an undelivered-outbox count. Use
`alerts` on the delivery database for delivery status. Source gap episodes and
health failures are not imported as notification diagnostics in this version;
monitor readiness/metrics separately. Zero imported findings is not an all-clear.

Back up both databases. The existing `backup` command copies the delivery
manifest, queue, leases, acknowledgements and recovery receipts together. Follow
archives do not include this separate delivery history. Archive recovery with a
retained compatible delivery DB can resync without resetting acknowledgements;
an older source that predates the prior sync is rejected until reconciled.
No unattended sync scheduler or hosted receiver is configured by these commands.
