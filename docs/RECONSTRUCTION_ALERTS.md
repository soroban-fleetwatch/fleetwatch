# Reconstruction finding and diagnostic alerts

Durable reconstruction now enqueues findings in the same synchronous transaction
as its accepted raw evidence, projection and checkpoint. A bounded latest-attempt
summary and diagnostic are also saved for incomplete application, gaps, preimage
conflicts, uncertain watched evictions, incomplete source requests or changed
evidence at an already accepted ledger.

```sh
./fleetwatch reconstruct --baseline testdata/reconstruction-baseline.json \
  --capture testdata/reconstruction-capture.json \
  --state /workspace/reconstruction.db --stream operator-demo
# exit 2; two synthetic findings queued, none sent
./fleetwatch alerts --state /workspace/reconstruction.db \
  --network SYNTHETIC-NETWORK --source rpc_ledger_meta --stream operator-demo
```

Listing opens the file read only, revalidates the retained projection/queue,
omits lease tokens, and does not send or acknowledge anything. Source
rpc_ledger_meta requires a stream ID; synthetic/user_snapshot do not accept one.
The queues are isolated by network/stream and coexist with the snapshot outbox.

## Atomicity and repair

A full queue, injected abort, cancellation before commit or process exit before
commit cannot advance the checkpoint while losing its finding or diagnostic.
Exact replay/cold resume preserves acknowledged delivery state. On an older
checkpoint predating alert integration, read-only inspection does not initialize
alerts; the next explicit reconstruct call backfills retained findings. Review
historical records before delivery.

A failed attempt with no accepted ledger still saves a diagnostic and binds its
stream to that asserted baseline. Malformed input, invalid baseline assignments
and cumulative stream bounds are refused rather than recorded as valid attempts.
Changed overlap yields EVIDENCE_CONFLICT, keeps accepted evidence unchanged and
records the refused input's digest. The summary contains schema version, network,
stream, baseline ledger/digest/header pin, requested range, retained position,
COMPLETE/PARTIAL status, fixed code and input digest. Arbitrary upstream errors
and full failed capture bodies are not retained.

The input digest is SHA-256 of compact Go JSON fields Domain, Capture, with domain
`fleetwatch.reconstruction-input.v1`. It does not prove a failure reason without
the original input. Exact duplicate attempt identities deduplicate; different
input or retained context can create another historical diagnostic. There is no
wall clock in identity.

Repair changes `checkpoint.last_attempt` to COMPLETE but never deletes historical
diagnostics or their delivery status. A diagnostic describes its identified
request/range, not necessarily a current outage. The HMAC timestamp is delivery
time, not observation time. Last-attempt status describes the latest **offline
invocation**, not an independent source-freshness/availability probe.
The accepted report stays RETAINED_PREFIX_ONLY. A failure-only scope can have
found=false, a null report and a PARTIAL last_attempt. Attempt summaries are
bounded to 4 KiB; accepted/failure-only scopes share the 128-stream limit.

## Payload and delivery

The outer webhook envelope/MAC framing remains v1; the inner reconstruction
payload is **schema_version 2**. Snapshot payloads remain v1. Receivers must
recognize the inner version and reject unsupported versions, preserving labels.
See [payload schema](../schema/reconstruction-alert-v2.schema.json),
[attempt schema](../schema/reconstruction-attempt-v1.schema.json), and
[WEBHOOKS.md](WEBHOOKS.md) for receiver MAC verification.

Payload v2 includes source rpc_ledger_meta, stream, evaluation
UNVERIFIED_PROJECTED_STATE, an assurance object and exactly one of:

- reconstruction_event: baseline digest/pin, ledger/header/metadata evidence and
  the derived event/finding.
- reconstruction_diagnostic: the identified PARTIAL attempt summary.

All payloads enforce UNVERIFIED baseline/source authentication, NOT_VERIFIED
metadata commitments, UNKNOWN membership/history completeness and UNDETERMINED
final verdict. Finding authorization stays UNVERIFIED. HMAC authenticates
delivery by the sender, not consensus, upgrade authority or a signed approval.
The older approval-binding command does not accept these reconstruction incidents.

Finding alert IDs hash ordered Go JSON fields Domain, Network, Stream, Event
under `fleetwatch.reconstruction-alert.v1`. Event is the complete evidence wrapper,
including the derived event ID and finding. Diagnostic IDs hash Domain, Attempt
under `fleetwatch.reconstruction-diagnostic.v1`. IDs include stream context and
differ from reconstruction event IDs. They are local versioned identifiers,
not signatures or RFC 8785 canonicalization.

Listing/claiming checks findings against the rederived retained report and requires
the latest partial diagnostic. Missing/changed records fail closed. Historical
diagnostics receive identity/scope/shape checks; failed raw inputs are unavailable
for rederivation. These are local consistency checks, not protection against an
attacker who replaces the entire database and evidence.

For an operator-configured receiver with a runtime HMAC key:

```sh
./fleetwatch deliver --state /workspace/reconstruction.db \
  --network EXACT_NETWORK --source rpc_ledger_meta --stream STREAM_ID \
  --webhook-url HTTPS_RECEIVER --allow-host EXACT_HOST
```

This guide does not configure or contact a recipient. Existing hostname/IP checks,
certificate verification, redirect rejection, proxy fail-closed behavior, signing
limits and dedup requirements remain. No HMAC key is stored in the database.

Delivery reuses fenced 30-second leases, 15-second send contexts, persisted
backoff and five-attempt dead letters. Store claims inherit the dispatch context
and bounded revalidation; waiting before grant does not consume the granted
lease. Delivery is at least once: a lost acknowledgment can repeat a stable ID,
so receivers must deduplicate. Historical diagnostics can arrive after repair.

There are at most 10,000 finding/diagnostic rows per stream and 1 MiB per stored
alert. If an enqueue exceeds its bound, the complete checkpoint/attempt transaction
fails. Explicit [dead-letter recovery](ALERT_RECOVERY.md) retains the same alert
ID, payload and sink with atomic audit receipts. There is no pruning, automatic
diagnostic resolution or continuous dispatcher. Backup preserves all namespaces together.
New checkpoint records include an alert-integration marker; older software that
does not understand it may refuse those records. Preserve the database when
changing software versions; no automatic downgrade/migration is provided.

Tests use generated trusted local TLS certificates and test-only HMAC material.
They verify exact-body MACs, labels, deduplication, acknowledgment, repair/restart,
pre-commit process crash, capacity, lease waiting and tamper rejection. No live
recipient was contacted. Production monitoring, trusted chain evidence,
continuous ingestion and signed-policy integration remain unfinished.
