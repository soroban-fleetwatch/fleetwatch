# Normalize executable-related metadata changes

`changes` reads a diagnostic [history capture](HISTORY.md) and emits the
contract-instance and executable-tag records present in ledger-close metadata.
It performs no RPC calls and writes no database state.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch changes --capture testdata/history-changes-capture.json > changes.json
```

The bundled file is a **synthetic XDR ordering fixture**, not a valid consensus
ledger or Testnet observation. It contains illustrative executable-tag preimages
in fee groups to exercise grouping order; it does not establish that actual
Stellar fee processing can change executable references. It includes A→B→A
writes in two operations of the same transaction, a restore, upgrade removal and
separate eviction marker. No signing key or actual transaction is involved.

The [output schema](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/schema/metadata-changes-v1.schema.json) records each
accepted ledger's header hash, metadata digest, ordered `changes`, and separate
`unordered_evictions`. Only persistent contract-instance entries and CAP-85
SCV_EXECUTABLE_TAG entries are projected. Other account, TTL, code and ordinary
contract-data entries are not part of this projection. No complete network state
or fleet membership can be inferred from it.

## Ordering and record types

The order follows the pinned SDK's
[LedgerChangeReader](https://github.com/stellar/go-stellar-sdk/blob/v0.8.0/ingest/ledger_change_reader.go)
and
[transaction change grouping](https://github.com/stellar/go-stellar-sdk/blob/v0.8.0/ingest/ledger_transaction.go):

1. FEE groups for **all** transactions, in metadata apply order.
2. Each transaction's TX_BEFORE, OPERATION groups, and TX_AFTER, in that order.
3. POST_FEE groups for all transactions (available in LedgerCloseMeta v2).
4. UPGRADE groups in metadata order.

LedgerCloseMeta v0/v1/v2 and TransactionMeta v0/v1/v2/v3/v4 are decoded explicitly.
Legacy TransactionMeta v0 contributes operation groups only. Protocol support
remains 28–29, so old-ledger special handling for protocol ≤12 is out of scope.
The reserved LedgerCloseMeta v1 `unused` vector must be empty; a nonempty vector
fails rather than silently dropping possible data. Contract events and diagnostic
events are not treated as additional ledger-entry mutations.

Every change retains its exact vector `change_index`, even when preceding
unrelated entries were filtered out. Transaction and operation indices are zero
based. Non-applicable transaction/operation/upgrade indices are null. Transaction
hashes are metadata claims, not checked envelope hashes or signer identities.

| Type | Meaning |
|---|---|
| STATE | Supplied preimage/assertion; not a write |
| CREATED / UPDATED | Supplied entry creation or update |
| RESTORED | Supplied restore; distinct from creation/update |
| REMOVED | Supplied key removal, with no invented prior value |
| EVICTED | Key from the eviction vector, without an invented value or execution position |

The collector's eviction vector does not supply a verified operation position.
It is kept in `unordered_evictions`, with phase EVICTION_UNORDERED. Its
`change_index` is only its original vector position. The command never merges
these markers into a total execution order or treats them as permanent deletion.

For observed values, `entry_xdr` and `key_xdr` are re-encoded from typed XDR;
`last_modified` must be positive and not exceed the enclosing ledger. The source
metadata digest links back to the original full metadata, including bytes not in
this projection. Normalized contract/reference values use the existing typed
source decoder. Owner addresses are checked and opaque tags remain exact bytes
in lowercase hex; `ff0061` and `ff006100` remain different keys. Removal/eviction
records omit entries, hashes and last-modified values rather than filling them in.

IDs are lowercase SHA-256 of compact Go JSON for an ordered record with fields
Domain, Network, HeaderHash, MetadataSHA256, Change. Domain is
`fleetwatch.metadata-change.v1`; Change is the normalized record with its ID
empty/omitted. Identity includes phase/indices, full key/entry evidence and the
metadata digest. It has no wall clock. Repeated normalization is deterministic;
different networks or different metadata bytes intentionally produce different
IDs. This is not a signature or an RFC 8785 canonicalization protocol.

## Assurance, failures and bounds

Before normalization, capture shape/coverage claims are checked and each ledger's
raw XDR, header linkage and retained evidence descriptors are revalidated. A
changed digest/sequence/descriptor is not accepted merely because it appears in
a previously serialized capture. Inflated VERIFIED assurance flags are rejected.
This does not authenticate the endpoint or verify metadata commitments.

An invalid/unsupported executable change aborts its **whole ledger**. Earlier
accepted ledgers remain in the report; no prefix of the offending ledger is
emitted. An incomplete input capture can yield its valid prefix but remains
PARTIAL with exit 4. `processed_through` describes this projection only and is not
a persisted ingestion checkpoint.

The normalizer retains STATE records, intermediate writes and operation boundaries
instead of compacting to final state. It does **not** yet validate STATE against a
trusted baseline, pair preimages with every mutation, reconstruct membership,
detect/classify incidents, calculate blast radius, or apply changes. Those steps
are required before continuous ingestion and within-ledger incident reporting.

The separate experimental [reconstruct command](RECONSTRUCTION.md) now applies an
operator-asserted baseline to an explicit watchlist, checks executable-field
preimages and compares operation-boundary membership. It adds no independent
baseline authentication or metadata commitment verification.

Every report retains CHANGED_ENTRIES_ONLY scope, UNKNOWN membership/history
coverage, UNVERIFIED source authentication, NOT_VERIFIED metadata commitments,
NOT_APPLIED state changes, and UNDETERMINED final verdict. COMPLETE_CAPTURE means
the requested capture's records were structurally projected; empty output is not
proof of an unchanged or safe fleet. Snapshot replay rejects these diagnostic
change reports.

Limits: strict JSON input ≤25 MiB; existing capture/XDR limits; ≤10,000 emitted
records across the capture; ≤100,000 scanned entry changes per ledger; ≤10,000
transactions, operation groups, upgrades or eviction keys per applicable vector;
≤20 MiB compact normalized ledger JSON across the capture. Pretty-printing adds
formatting overhead. A 30-second normalization context is checked between ledgers
and changes; parsing and a single decoder call remain bounded by input/XDR limits.

Exit 0: complete diagnostic projection; 3: malformed input/usage/envelope;
4: invalid evidence/change, incomplete capture, cancellation or normalization
limit; 5: output failure. No alert threshold is evaluated. Output can contain
contract IDs and public entry data; keep it in operator-controlled storage.
