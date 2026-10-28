# Query an observed fleet at a retained ledger

`report` is an offline, read-only query over a persisted replay scope. It returns
the contracts explicitly observed using an exact owner/tag at an exact ledger.
It does not discover all contracts on the network or query RPC history.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch replay --journal testdata/replay-transient.json --state demo.db
# exit 2: synthetic changes at ledgers 101 and 102
./fleetwatch report --state demo.db --network TESTNET-SYNTHETIC \
  --ledger 101 --owner OWNER_DEMO \
  --tag-hex 72656c656173652d737461626c65
# exit 0: observed contracts A/B; reference hash aa... -> bb...
```

The network and owner are exact, case-sensitive v1 identifiers. Synthetic labels
are accepted; a report does not establish that an owner is a deployed Stellar
contract. `--source synthetic|user_snapshot` selects the existing scope (default
synthetic). These source kinds are not independently authenticated collector
identities. Owner is the reference owner, not a claim about upgrade authority.

Tags are raw bytes represented as hex. Uppercase hex input is accepted and the
report emits lowercase hex; the bytes are unchanged. NUL and invalid UTF-8 bytes
are preserved, and `ff0061` is distinct from `ff006100`. Empty, odd-length,
non-hex and oversized tags are rejected.

The [JSON Schema](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/schema/fleet-report-v1.schema.json) describes the output:

- `current` contains the requested retained snapshot's canonical SHA-256 digest,
  its observed hash (or null when unresolved), and sorted observed member IDs.
- `previous`, when present, describes the adjacent retained ledger. It is null
  for the replay baseline. No earlier state is invented.
- `membership_change` separates joined, left and retained members and gives their
  observed union. Joined/left describe differences in supplied input, not proof
  of deployment, destruction, or a precise transaction ordering.
- `related_events` contains the existing replay events at the requested ledger
  with that owner/tag or a contract belonging to either observed member set.
  This includes repoints into/out of the reference and input omissions. Event
  IDs match the persisted cumulative replay report; unrelated findings and
  changes at other ledgers are excluded.
- `processed_through`, `sequence_coverage` and `input_issues` describe the latest
  retained replay scope, including any failed extension beyond its prefix.

Snapshot digests use the existing canonicalization in [REPLAY.md](REPLAY.md).
They bind the local snapshot bytes after canonicalization; they are not ledger
header hashes, transaction proofs or cryptographic attestations of provenance.
Related events are recomputed from retained observations before use. No approval
receipt is evaluated and no finding's severity or authorization is changed.

Missing scopes or unretained ledgers return `snapshot_status=NOT_RETAINED` with
null observations and an incomplete exit. The command never substitutes a nearby
snapshot, queries an unprocessed suffix after a gap, or extrapolates a hash into
an unobserved interval. A retained snapshot with a missing reference still lists
its observed members but sets `reference_status=NOT_OBSERVED` and `wasm_hash=null`;
this is not proof the reference was deleted.

An empty member list with an observed reference can be a successful query. It
only says that this input contains no matching members. Every report retains
`fleet_scope=OBSERVED_SNAPSHOT_MEMBERS`, `membership_coverage=UNKNOWN`,
`evaluation_status=UNVERIFIED_INPUT`, and `final_verdict=UNDETERMINED`. Even a
complete supplied ledger sequence does not prove complete membership, history
or on-chain approval. Same-ledger intermediate transitions remain unobservable.

| Exit | Meaning |
|---|---|
| 0 | Requested snapshot and reference observed, latest supplied sequence complete |
| 3 | Invalid query or arguments |
| 4 | Scope/ledger unavailable, reference unresolved, or latest sequence incomplete |
| 5 | Database unavailable/locked/invalid or output failure |

Exit 0 can include critical related findings; it means the query succeeded, not
that the transition is safe. A later gap makes exit 4 take precedence even for an
earlier retained snapshot; that snapshot's observation remains in the JSON.
Use `compare` or `replay` for severity-based alert exits.

Use a trusted local database path. The command uses a read-only bbolt handle,
never creates missing databases, and never sends alerts or modifies state.
Stop processes holding a writable database handle first; lock acquisition times
out after one second. This query checks retained replay invariants but does not
inspect outbox delivery state or approval indices; see [HEALTH.md](HEALTH.md).
