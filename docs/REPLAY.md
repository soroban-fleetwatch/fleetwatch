# Offline sequence replay

`replay` exercises ordered, supplied per-ledger snapshots. It does not consume
Stellar ledger metadata, write a database, deliver alerts, or verify chain history.

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
baseline conflicts). No suffix is processed after either issue. There is no
persistent checkpoint: processed_through describes only this invocation.

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
