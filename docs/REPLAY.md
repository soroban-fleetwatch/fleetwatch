# Offline sequence replay

`replay` exercises ordered, supplied per-ledger snapshots. It does not consume
Stellar ledger metadata, deliver alerts, or verify chain history. Database persistence is optional through --state.

```sh
/workspace/.tools/bin/fleetwatch replay --journal testdata/replay-transient.json
# exit 2; two changes, at ledgers 101 and 102: A -> B -> A
/workspace/.tools/bin/fleetwatch replay --journal testdata/replay-gap.json
# exit 4; missing ledger 101; processed_through stays at 100; no inferred change
```

Journals bind schema version 1, source (`synthetic` or `user_snapshot`), network,
expected inclusive `from_ledger`/`to_ledger`, and `snapshots`. The first snapshot is
the baseline. At most 1,000 snapshots and an expected span of 1,000 ledgers are
accepted; JSON input stays capped at 25 MiB. At most 10,000 events can be emitted.
Unknown fields, duplicate JSON keys, malformed snapshots, network mismatches,
out-of-order entries, and observations outside the declared range are rejected.

A missing observation stops replay at the contiguous prefix with OBSERVATION_GAP.
A duplicate ledger with equivalent normalized state is ignored. A conflicting
same-ledger observation reports SOURCE_DIVERGENCE, removes findings for that
ledger, and rolls processed_through back to the preceding ledger (or 0 if the
baseline conflicts). No suffix is processed after either issue. Without --state there is no persistent checkpoint: processed_through describes only this invocation.

Event IDs are lowercase SHA-256 of Go's JSON encoding of an explicitly ordered
record: Domain, Source, Network, Before, After, Ledger, Finding. Domain is
`fleetwatch.snapshot-event.v1`; Before/After are SHA-256 digests of canonical
snapshot JSON. Canonicalization sorts contracts by ID and references by owner/tag,
lowercases hash/tag hex, and normalizes nil arrays to empty arrays. Identity
includes the complete finding and no wall clock. Reordering lists and inserting
equivalent duplicates leaves event IDs stable. These IDs are scoped to the v1
snapshot rules; they are not transaction-level ledger event IDs.

`COMPLETE_INPUT_SEQUENCE` means an observation exists at each declared ledger,
not that its contents or chain provenance are complete. Evaluation remains
UNVERIFIED_INPUT and finding authorization UNVERIFIED. Per-ledger state cannot
detect multiple intermediate transitions within one ledger. Two distant
snapshots cannot prove the absence of intervening changes. Future ledger-meta
adapters and transactional storage must supply that stronger evidence separately.

Exit 0: no findings/issues; 2: one or more findings; 3: invalid journal/usage;
4: missing/conflicting ledger observation; 5: output failure. Source issues take
precedence over findings. There is no approval-policy option yet.

Schemas live in `schema/`: snapshot-v1, journal-v1 and capture-v2. Capture envelopes
are diagnostic and are deliberately rejected as journals or v1 snapshots. Schemas
express structural limits; runtime validation additionally checks duplicates,
ordering, expected ranges, exact identifiers and byte identities. A contract-shaped
schema string is not a verified Stellar address; the capture adapter checks its
checksum. Capture may contain ledger 0 only as a diagnostic when a response omits
its ledger, and such a result remains partial.

## Optional durable state

```sh
/workspace/.tools/bin/fleetwatch replay \
  --journal testdata/replay-transient.json --state /workspace/replay.db
```

The database path must be new or an existing FleetWatch bbolt database. Do not
pre-create an empty file. New files use mode 0600; symlinks, unknown databases,
and conflicting concurrent writable opens are rejected. Keep the database on a
local filesystem supporting advisory locking and fsync. The file is not encrypted.

State is scoped by exact network and source kind. Later segments can begin at the
next ledger or overlap recorded observations. Previously recorded evidence fills
known overlap; conflicting same-ledger state aborts the transaction (exit 4).
Missing ranges stop at the contiguous prefix, save its findings and cursor with
the diagnostic issue, and return 4. Out-of-order input within an individual journal
is still invalid. A new journal cannot prepend an earlier baseline to an existing
scope. Observation/event bounds apply across the accumulated scope, not per call.

Cursor, observations and findings commit in one synchronous transaction. On
restart, replay recomputes findings and checks stored invariants. Duplicate replay
returns the cumulative report with the same event IDs; exit 2 may therefore recur
for existing findings. No alerts are delivered. A failed output write can occur
after commit: repeat the same journal safely to retrieve the report again.
Storage failures return 5. An intentional pre-commit process-crash test verifies
that incomplete writes do not advance the checkpoint.

Stop all processes using the file before copying it for backup; live filesystem
copies are not a supported backup procedure. Reopen a restored copy and replay
the original journal to validate it. There is no schema migration or automatic
retention yet. Preserve the file when changing parser/rule versions; a versioned
migration must handle changed event derivation. This local store remains bounded
to 1,000 ledgers, 10,000 events, 25 MiB of encoded state per scope. Durable ledger
metadata ingest and the production Postgres/outbox service are still unimplemented.
