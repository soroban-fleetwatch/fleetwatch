# Reconstruct a fixed watched fleet offline

`reconstruct` applies revalidated metadata to an **operator-asserted baseline**
and compares the resulting watched executable state at metadata group boundaries.
It is a diagnostic: no RPC, approval binding or alert delivery is performed.
Without `--state`, no checkpoint is written. Optional bounded
[durable checkpoints](RECONSTRUCTION_CHECKPOINTS.md) retain accepted raw evidence
and findings using `--state FILE --stream ID`.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch reconstruct --baseline testdata/reconstruction-baseline.json \
  --capture testdata/reconstruction-capture.json > reconstruction.json
# exit 2: two synthetic reference transitions in the same transaction, A→B→A
```

Both fixtures are synthetic typed XDR/model inputs, not consensus-valid ledgers,
Testnet contract observations, signed approvals or deployments. The final hash
equals the baseline hash, but the report retains both operation-boundary events.
This demonstrates why comparing only the final snapshot misses such transitions.

## Baseline and source requirements

The [baseline schema](../schema/watched-baseline-v1.schema.json) wraps a v1
snapshot with `schema_version=1`, `kind=watched_baseline`, `source=user_asserted`
and a lowercase nonzero 32-byte `header_hash`. Its ledger must immediately precede
the capture's requested start, on the exact same network. The supplied hash must
match the first accepted header's previous hash.

The baseline must contain 1–50 existing contracts with canonical, checksum-valid
Stellar contract addresses. These IDs define an immutable watchlist; changes to
other contract instances never add members to it. Each external reference used
by a watched instance must resolve in the baseline. Up to 1,000 baseline reference
values, including explicitly supplied potential future references, are accepted.
Source keys, hashes, duplicate IDs/references and opaque tag bytes are validated;
hash/tag hex is normalized without changing the underlying bytes.

Matching a supplied header hash is a **local pin**, not proof of a trustworthy
baseline or consensus. A snapshot is not proven to match a ledger's bucket-list
commitment. Every result retains UNVERIFIED baseline and source authentication,
NOT_VERIFIED metadata commitments, UNKNOWN membership/history completeness,
UNVERIFIED_PROJECTED_STATE evaluation and UNDETERMINED final verdict. There is no
approval-based severity downgrade or signed trust evaluation in this command.

Input must be the raw history capture, not a previously edited `changes` report.
The normalizer rechecks original XDR, hashes, descriptor consistency and header
linkage, then supplies executable-related changes in its documented
[transaction-phase order](METADATA_CHANGES.md).

## Application and uncertainty rules

A group is a fee/before/operation/after/post-fee/upgrade vector, identified by its
phase and transaction/operation/upgrade indices. STATE records assert projected
preimages and are not writes. For tracked contract instances they compare only
executable identity; for references they compare owner/tag/hash. Other storage,
entry extensions and last-modified continuity are not established by a baseline
that does not contain them. The report says EXECUTABLE_FIELDS_ONLY explicitly.

UPDATED requires a matching STATE in the **same group**. A preimage from a fee or
previous operation cannot authorize an update in another group. A preimage that
conflicts with the retained projection stops application. Multiple mutations of
one key in a single group are rejected as ambiguous: vector position alone does
not establish the internal call/write order of an operation. Within-operation
transient changes are therefore not detected by this reducer.

All mutations in a group are applied before resolving the watched references and
comparing boundary states. This supports simultaneous reference/member repoints
within the group without inventing entry-by-entry execution order. Changes to
formerly used references remain available for that boundary comparison so a
member leaving a reference does not manufacture a missing-entry finding.

Unknown reference STATE values can seed a previously unobserved reference cache;
CREATED or RESTORED can supply an absent/unknown cached reference. Those are
metadata claims, not an independently authenticated historical baseline. A
creation/restore that conflicts with a known-present entry fails. A watched
contract repointed to a reference still unresolved at the group boundary fails.
The reducer does not issue current-state RPC lookups to fill historical gaps.

Removal of a watched contract or an in-use reference stops the ledger rather
than inventing a clean state. Executable mutations in fee/post-fee groups are
unsupported and stop application. STATE preimages there remain assertions only.
These are conservative supported-subset rules, not a general archive lifecycle
or consensus-execution validator.

Unordered evictions touching a watched instance or a reference used at any
accepted group boundary in that ledger stop the entire ledger. Other evictions
invalidate cached reference knowledge for future use; they are never silently
applied as permanent deletion or inserted into operation order. A restored
reference can be used later only when the supplied records resolve it without
violating these rules.

Every ledger is applied to an isolated candidate. A preimage conflict, unresolved
reference, ambiguity, watched eviction or resource failure discards **all** its
candidate state, groups and findings. Previously accepted ledgers remain in a
PARTIAL report. `processed_through` begins at the validated baseline ledger; it
is an offline projection position, not a durable or authenticated ingestion
checkpoint. An incomplete capture can supply an accepted prefix but returns 4.

## Findings and evidence

The [report schema](../schema/watched-reconstruction-v1.schema.json) includes:

- The original canonical baseline snapshot digest and separately supplied header
  hash, sorted watchlist, and final active watched snapshot.
- Each accepted ledger's source header hash and metadata digest.
- Groups containing their normalized change IDs, first projected change position,
  and before/after active-projection snapshot digests. These digests are not ledger
  roots or complete cached-state commitments. Within a ledger, consecutive group
  active-state digests are continuous even when watched reference membership changes.
- Derived findings with group IDs and stable event IDs. Normalized change IDs can
  be regenerated using `changes --capture FILE` to inspect the entry evidence.

The pure comparison API for operation states permits equal **actual** ledger
numbers; it does not invent synthetic heights. Ordinary `compare` still requires
strictly increasing snapshot ledgers. Findings retain authorization UNVERIFIED.
No unsigned policy or standalone approval receipt is silently applied.

Affected contracts are the union of observed watched members before/after that
group, labeled OBSERVED_MEMBERSHIP_UNION. It can include an instance leaving the
reference in the same operation. It is not proof that every listed instance
actually executed the new code, and says nothing about unmonitored fleet members.
Newly used references can be reported as first observed in the watched scope;
this is not a claim they were newly created on-chain.

Group IDs use SHA-256 of compact Go JSON for ordered fields Domain, Network,
BaselineDigest, HeaderHash, MetadataSHA256, Group, with the group ID empty.
Domain is `fleetwatch.applied-group.v1`. Event IDs similarly bind Domain, Network,
BaselineDigest, BaselineHash, HeaderHash, MetadataSHA256, GroupID, Finding under
`fleetwatch.reconstruction-event.v1`. There is no wall clock. This versioned local
encoding is not a signature or RFC 8785 canonicalization. IDs are distinct from
the older snapshot replay incident IDs and cannot be bound using its approval
command or treated as queued alerts.

## Limits and exits

Existing strict JSON/capture/normalizer bounds apply. Additional limits are 50
watched instances, 1,000 baseline references, 10,000 cached present/absent reference
keys, 10,000 derived events across the capture, and 20 MiB compact accepted-ledger
JSON. A 30-second context is checked between work units. Unknown references are
resolved only from the bounded supplied baseline/metadata, so legitimately large
or unusual history can stop partial rather than be silently skipped.

| Exit | Meaning |
|---|---|
| 0 | Complete diagnostic application with no watched findings |
| 2 | Complete diagnostic application with one or more findings |
| 3 | Malformed input/usage/baseline/capture envelope |
| 4 | Range/network/header mismatch, incomplete source, conflict, ambiguity or limit |
| 5 | Output failure |

Exit 0 remains an UNDETERMINED chain verdict. Snapshot `replay` rejects these
diagnostic reports. Extracting a projected snapshot does not upgrade its source
assurance or create an authenticated recovery checkpoint. Production evidence/cursor storage,
archive/eviction reconciliation, independently trusted baselines/commitments,
continuous workers and controlled live CAP-85 upgrade validation remain unfinished.
