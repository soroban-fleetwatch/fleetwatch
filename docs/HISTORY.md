# Diagnostic ledger metadata collection

`history` fetches an exact inclusive range of up to 16 ledger-close metadata
records using Stellar RPC `getLedgers`. It is a read-only collection step for
the future sequential ingestion adapter. It does not apply state changes,
advance a database cursor, compare fleets, bind approvals or send alerts.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch history --rpc-url https://soroban-testnet.stellar.org \
  --network-passphrase 'Test SDF Network ; September 2015' \
  --from-ledger FIRST --to-ledger LAST > history-capture.json
```

Replace FIRST/LAST with a retained ledger range. A once-valid example may expire
from the provider's retention window. `getNetwork` must claim the exact supplied
passphrase and protocol 28 or 29. This pinned decoder intentionally rejects
other network/header protocols until compatibility is investigated. Current-state
`getLedgerEntries` is not used to reconstruct past observations.

The collector requests one ledger at a time by exact `startLedger`, with a page
limit of 1. It validates the response's oldest/latest range and refuses missing,
duplicate, unsolicited or out-of-order records. After each accepted record it
requests the next sequence directly; an opaque pagination cursor cannot skip
unobserved ledgers. Provider retention bounds are claims, not an availability SLA.

For each record the collector:

1. Decodes `headerXdr` as **LedgerHeaderHistoryEntry**. This differs from
   `getLatestLedger.headerXdr`, which contains LedgerHeader.
2. Checks the reported sequence and close time against the header and computes
   SHA-256 of its inner LedgerHeader XDR, comparing both supplied hash fields.
3. Decodes full LedgerCloseMeta XDR (versions 0, 1 or 2), requiring the embedded
   header history entry to match the separately returned one.
4. Checks each later header's previous hash against the preceding accepted hash.
5. Retains the original canonical base64 header/metadata and SHA-256 of the raw
   metadata bytes, with sequence, protocol, time and metadata version.

This provides structural checks and internal linkage of **endpoint claims**.
There is no independently trusted first-header anchor, quorum/SCP verification,
or validation of all transaction-set/result/state commitments. In particular,
matching an embedded header does not prove that the rest of the metadata belongs
to that ledger: a modified non-header field can retain a valid embedded header.
The tests explicitly preserve this trust boundary. Metadata digests detect local
byte changes relative to a retained digest, not an adversarial provider.

The [capture schema](../schema/history-capture-v1.schema.json) therefore always
retains `source_authentication=UNVERIFIED`, `history_completeness=UNKNOWN`,
`metadata_commitment=NOT_VERIFIED`, `state_changes=NOT_APPLIED`, and
`final_verdict=UNDETERMINED`. `COMPLETE_ENDPOINT_SEQUENCE` means this requested
range was returned and passed those structural/link checks. It does not mean
the fleet, transaction history or executable transitions have been verified.

On failure, only the accepted prefix remains in `ledgers`; the offending record
is not appended. `processed_through` is the last locally accepted sequence, or 0,
and is **not** a persisted ingestion checkpoint. A fixed diagnostic error code
describes the stop; upstream error bodies and arbitrary metadata are not logged.

## Bounds and exits

Metadata is limited to 4 MiB decoded per ledger, header base64 to 8 KiB, aggregate
retained header/metadata base64 to 20 MiB and range length to 16. XDR decoding is
limited to depth 64, 16 KiB header allocation and 16 MiB metadata allocation.
Large or deeply nested legitimate ledgers may exceed these local limits; the
collector stops partial rather than silently omitting them.

The HTTPS transport reuses the capture adapter: inherited proxy/CA trust,
certificate verification, no redirects/URL credentials/query secrets, bounded
25 MiB HTTP responses, request cancellation, and bounded transient HTTP retries.
Each request has a 20-second timeout; the CLI has a two-minute overall deadline.
Old historical close times are allowed; this is not a freshness check.

| Exit | Meaning |
|---|---|
| 0 | Requested endpoint sequence structurally collected and internally linked |
| 3 | Invalid endpoint, network or range arguments |
| 4 | Network/source failure, unsupported protocol, retention gap or invalid evidence |
| 5 | Output failure |

Full raw metadata can include unrelated public account/transaction data. Keep
captures in operator-controlled storage and avoid publishing raw captures in
logs. These diagnostic envelopes are deliberately rejected by snapshot `replay`.
They cannot be relabeled `synthetic` or `user_snapshot` to manufacture chain
assurance. No journal conversion is implemented.

## Verified scope and next work

On 2026-10-26 the compiled command read actual Testnet endpoint metadata for
ledgers 5403432–5403433 (protocol 29, LedgerCloseMeta v2), verified header hashes
and linkage, and emitted schema-valid output. See the dated
[research record](research/2026-10-26-ledger-history.md) and its small evidence
summary. This is not a controlled CAP-85 deployment/update demonstration.

Sequential state ingestion still needs a trustworthy baseline, normalization of
transaction/fee/upgrade/eviction/restore changes with transaction ordering,
reference/member reconstruction, commitment/provenance decisions, durable
evidence/cursor storage, restart reconciliation and continuous worker tests.
Within-ledger A→B→A detection is not provided by this collector or snapshot replay.
