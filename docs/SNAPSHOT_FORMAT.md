# Normalized snapshot and policy schemas (v1)

## Snapshot

```json
{
  "schema_version": 1,
  "network": "TESTNET-SYNTHETIC",
  "ledger": 100,
  "contracts": [
    {
      "id": "CONTRACT_DEMO_A",
      "executable": {
        "kind": "external_ref",
        "owner": "OWNER_DEMO",
        "tag_hex": "ff0061"
      }
    }
  ],
  "references": [
    {
      "owner": "OWNER_DEMO",
      "tag_hex": "ff0061",
      "wasm_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  ]
}
```

This is an **off-chain interchange format**, not Stellar's on-chain XDR format. Its identifiers in test fixtures are synthetic. Do not use the demo snapshots as evidence of actual network events.

`tag_hex` contains the original SCString byte payload of the `SCV_EXECUTABLE_TAG` ledger key as lowercase or uppercase hex. Preserve bytes exactly. Identity matching treats hex case as equivalent. `owner` and contract IDs are treated as opaque strings in this initial prototype and **are not cryptographically validated Stellar addresses**. Future ingestion must validate addresses and network passphrases using official tooling.

A snapshot is intended to represent a consistent view at exactly one ledger sequence. Input completeness and consistency are currently the caller's responsibility. Two captures with different coverage may create false observations; currently flagged gaps and fleet counts are not proof of actual state removal.

The `references` array holds resolved Wasm hashes. In a full RPC observer, each row comes from the owner contract's persistent contract-data entry keyed by `SCV_EXECUTABLE_TAG` with the exact tag bytes; contract instances using an external executable reference point at that owner and tag.

## Exact approval matching

```json
{
  "schema_version": 1,
  "approved_changes": [
    {
      "owner": "OWNER_DEMO",
      "tag_hex": "ff0061",
      "from_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "to_hash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "ticket": "SEC-100"
    }
  ]
}
```

All four material fields (owner, raw tag, from hash, to hash) must match for an upgrade to be treated as approved. No policy supplied means no changes are approved. **Approval authenticity is not established** by reading a JSON file. Signed policy attestations and trusted key management are future work.

## Validation

- Both snapshots must share a network identifier.
- `after.ledger > before.ledger`.
- No duplicate contract IDs or duplicate reference `(owner, tag)` keys within a snapshot.
- Hex tag must be nonempty, complete-byte hex. Wasm hashes must be exactly 32 bytes.
- Unknown fields and trailing JSON are rejected.
- Files are capped at 25 MiB to reduce accidental resource consumption; production hardening remains pending.

## Hardened v1 limits and report assurance

Input files and streams are limited to 25 MiB before decoding. Duplicate JSON
members, unknown fields, trailing values, a null document, and nesting beyond
64 levels are rejected. Each contracts/references/approved_changes array is
limited to 10,000 records. These are local defensive limits, not Stellar
protocol constraints.

Contract IDs and owners remain opaque v1 identifiers for compatibility with
synthetic fixtures: 1–256 UTF-8 bytes, without whitespace, control characters,
or `/`. These checks do not validate Stellar addresses. Network names are
1–256 UTF-8 bytes and tickets 1–1,024 bytes; neither permits control characters
or all-whitespace content. Tags remain nonempty complete-byte hex, bounded to
1,024 decoded bytes, with case-insensitive hex representation and byte-exact
identity. Signed approvals and strict chain identifiers need a future schema.

Reports add `coverage: "UNKNOWN"` and
`evaluation_status: "UNVERIFIED_SNAPSHOTS"`; every finding carries
`authorization: "UNVERIFIED"`. These additive v1 report fields explicitly
apply even when an unsigned policy matches or no findings are emitted. A
successful process exit describes the severity threshold result, not complete
historical coverage or authenticated authorization.
