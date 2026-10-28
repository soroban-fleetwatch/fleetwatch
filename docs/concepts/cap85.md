# How CAP-85 fleets work

This page explains the ledger objects FleetWatch watches. The authoritative
source is the [CAP-85 specification](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0085.md),
which is marked Final for protocol 28.

## Three kinds of executable

A Soroban contract instance stores an executable. FleetWatch decodes all three
variants with the Stellar Go SDK v0.8.0 XDR types:

| Variant | What the instance holds | FleetWatch name |
| --- | --- | --- |
| Wasm | A 32-byte Wasm hash | `wasm` |
| Stellar Asset Contract | The built-in asset contract | `stellar_asset` |
| External reference (CAP-85) | `ContractExecutableExternalRef{executable_owner, tag}` | `external_ref` |

## The owner entry

For an external reference, the code is not in the instance. The owner contract
keeps a persistent contract-data entry whose key is an `SCV_EXECUTABLE_TAG`
value holding the tag. The entry's value is a 32-byte Wasm hash.

```text
 member contract A ─┐
 member contract B ─┼─► {owner: C…OWNER, tag: "release-stable"} ─► Wasm hash H1
 member contract C ─┘
```

When the owner writes H2 to that entry, members A, B and C all run H2 from that
ledger on. No transaction touches the members themselves.

## Tags are bytes

The tag is an `SCString`, which is a byte string. It is not guaranteed to be
valid UTF-8. FleetWatch keeps the exact bytes and shows them as lowercase hex.
Two tags that print the same but differ in bytes are different tags. Tags are
never trimmed, normalized or case-folded.

## Identity is network-scoped

The same contract ID string can exist on Testnet and Mainnet. FleetWatch keys
everything by network passphrase:

- contract: `(network, contract_id)`
- reference: `(network, owner_contract_id, tag_hex)`

## What FleetWatch reports

| Event on the ledger | Finding |
| --- | --- |
| Owner writes a new hash for a tag | Reference change, with old hash, new hash and every watched member using the tag |
| Owner writes A→B→A, even inside one transaction | Two separate changes, not "no change" |
| A member switches between Wasm, asset or another owner/tag | Repoint finding for that member |
| A member references an owner/tag with no observed value | Unresolved reference |
| A ledger is missing from the source, or data is ambiguous | Gap or `PARTIAL` result; no clean verdict |

## Why snapshots are not enough

Reading current state twice (`getLedgerEntries` now and later) can miss a change
that was reverted in between, and the reads may come from different ledgers.
FleetWatch therefore uses sequential ledger-close metadata (`getLedgers`) for
history, and labels snapshot captures `PARTIAL` when it cannot prove they come
from one ledger.

## Lifecycle of a watched reference

```text
            asserted in baseline
                    │
                    ▼
   ┌────────── OBSERVED(hash) ◄──────────────┐
   │                │                         │
   │   owner writes new hash in metadata      │
   │                ▼                         │
   │      REFERENCE CHANGE finding ───────────┘
   │
   │   entry evicted / missing from source
   ▼
 GAP or STOP ── processing halts; no inferred deletion or clean verdict
```

Eviction and restore of archived entries is not fully modeled yet. FleetWatch
stops rather than guess. This is tracked as planned work.
