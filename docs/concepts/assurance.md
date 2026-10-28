# Assurance labels

FleetWatch reports what it observed and how far that observation can be
trusted. These labels appear in JSON output and alerts. They are part of the
result, not decoration.

| Field | Current value | Meaning |
| --- | --- | --- |
| Source authentication | `UNVERIFIED` | Data came from the configured RPC endpoint. Nothing independent confirms the endpoint told the truth. |
| Baseline | `UNVERIFIED` | The starting state was asserted by the operator and pinned to a ledger header hash. It is not independently proven. |
| Metadata commitments | `NOT_VERIFIED` | Header hashes and previous-header links are checked. Transaction-set and result hashes inside the header are not yet recomputed, so altered transaction metadata with a valid header is not detected. |
| Membership / coverage | `UNKNOWN` | Only the contracts you listed are watched. Other contracts may use the same reference. |
| History completeness | `UNKNOWN` | Contiguous ledgers were checked within the retained range. Older history and gaps outside it are not claimed. |
| Final verdict | `UNDETERMINED` | FleetWatch never declares a change safe or malicious. |
| Authorization (snapshot reports) | `UNVERIFIED` | A policy-file match is not a signature. |

## Unsigned policy vs signed approval

These are different results and are never merged:

- **Policy match** (`POLICY_MATCHED_REFERENCE_CHANGE`): the observed old→new hash
  pair appears in a JSON file you supplied. Anyone who can edit that file can
  add a match.
- **Signed approval** (`verify-approval`, `bind-approval`): an Ed25519 statement
  bound to network, owner, tag, old and new hash, validity range and a nonce,
  checked against trust roots and signer thresholds you supplied. It proves the
  signers approved that exact transition. It does not prove the chain data is
  authentic.

## Status values you will see

| Value | Meaning |
| --- | --- |
| `COMPLETE_ENDPOINT_SEQUENCE` | The endpoint returned every requested ledger in order with valid header links. A structural claim about the endpoint's answer only. |
| `PARTIAL` | Some data was accepted and some was not. Only the accepted prefix is kept. Exit code 4. |
| `OK_OFFLINE` (`doctor`) | The local database is internally consistent. Not a statement about chain safety or source health. |
| `ALREADY_RETAINED` | The requested range is already stored. No RPC call was made. |

## What would raise these labels

Planned work, each tracked as an issue in `docs/build/issues.json`:
cross-checking headers across independent endpoints, recomputing
transaction-set and result hashes, and building baselines from pinned ledger
reads. A label changes only when tests and recorded evidence support it.
