# Ledger metadata RPC investigation — 2026-10-26

The pinned Go SDK v0.8.0 provides typed getLedgers request/response structures:

- [SDK RPC protocol types](https://github.com/stellar/go-stellar-sdk/blob/v0.8.0/protocols/rpc/get_ledgers.go)
- [LedgerCloseMeta accessors](https://github.com/stellar/go-stellar-sdk/blob/v0.8.0/xdr/ledger_close_meta.go)
- [RPC parseLedgerInfo implementation](https://github.com/stellar/stellar-rpc/blob/dac748d2d793443d9b3595ac50a5e912db8a62a5/cmd/stellar-rpc/internal/methods/get_ledgers.go#L291)

RPC HEAD was observed as dac748d2d793443d9b3595ac50a5e912db8a62a5.
The retrieved main source matched that pinned file byte-for-byte when checked.
Its parser slices LedgerHeaderHistoryEntry directly out of LedgerCloseMeta and
base64-encodes it as headerXdr. It is not the bare LedgerHeader type returned by
getLatestLedger. An initial bare-header decoder failed against real metadata;
the implementation and regression tests now use the actual wire format.

The environment's current network policy was observed enforced with the Testnet
host allowed. Native curl and the Go collector successfully used the inherited
proxy and normal TLS verification. An exploratory Python urllib request received
HTTP 403; it is not evidence that all clients or methods are inaccessible.
Earlier blanket endpoint-denial observations in STATUS.md are historical.

The actual getNetwork response claimed `Test SDF Network ; September 2015` and
protocolVersion 29. A getLedgers call returned oldestLedger 5282476 and
latestLedger 5403435 at observation time. This records one observed retention
window, not a retention guarantee. The compiled FleetWatch history collector then
retrieved sequences 5403432 and 5403433 separately and checked:

| Sequence | Header hash | Previous hash |
|---|---|---|
| 5403432 | 062f5c521a7d31677b88ebfbc03e293117c3855a190407ea28b5843cefca35ed | 4fcfc1669d0886e9c02309c42f4c73fc1456ef24013f592ea0551406390934b6 |
| 5403433 | 274a85ea735736343bc0b77ff4031e289b49e71b9c61ee7d46e1772d5247953a | 062f5c521a7d31677b88ebfbc03e293117c3855a190407ea28b5843cefca35ed |

Both decoded as protocol 29 and LedgerCloseMeta v2. The metadata digests and
header-history-entry base64 are in
[the observation summary](2026-10-26-testnet-history-sample.json). Full transaction
metadata was kept outside the repository. The summary is not a replay journal
or synthetic fixture. Its header hashes can be independently recomputed from the
inner header; its metadata digests cannot be reproduced without the original
metadata or a still-retaining provider.

The compiled capture passed JSON Schema validation and was rejected by snapshot
replay as designed. No transaction was submitted, contract deployed, signer
authority inferred, CAP-85 state change observed, source consensus authenticated,
or database ingestion checkpoint advanced. This evidence establishes bounded
read-only endpoint collection and parsing, not the full Testnet acceptance gate.
