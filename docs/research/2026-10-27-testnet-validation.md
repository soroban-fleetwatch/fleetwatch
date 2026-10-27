# Controlled Testnet validation — 2026-10-27

Dripwave Phase 8's scoped demonstration passed. This is an experimental monitor
validation, not an audit, independently authenticated chain proof, production
deployment, release or Wave approval. The machine-readable
[evidence](2026-10-27-testnet-validation.json) contains actual identities,
transaction hashes, observed ledgers, findings and recovery results.

The environment's current network policy permitted the Testnet faucet. Its
funding response reported success; subsequent fixture transactions succeeded.
The official CLI initially failed while writing its default read-only cache;
the runner now passes its supported `--no-cache` flag. No TLS, checksum or network
policy check was disabled. RPC uses the previously verified local adapter and
the managed proxy; the Go evidence reader uses its own existing strict HTTPS
RPC path to the same upstream endpoint.

## Actual transactions and observations

Owner: `CCSC3UOXCCNB3VV7MM2P63IPNVKHQQ3TUMBCIW6OW42F75FBGMI4Y3VA`.
Members, in the order used for version/repoint checks:

- `CDL6H5M2VYLOR536DDUG7SQXKJNLSADSIKNNMH5EW3R5R55QZJ2KXOZY`
- `CD6GZKPCBF2OFUV7VAGBFMWM4H2S2BBPI66CQ674RMFQAVLSVZOAWVSM`

Tag bytes: `666c65657477617463682d64656d6f` (`fleetwatch-demo`). The exact tested
Wasm hashes are recorded in [fixture preparation](2026-10-27-testnet-fixtures.md).

| Action | Ledger | Submitted transaction |
| --- | --- | --- |
| Shared A→B, both members affected | 5407730 | [87d17163…](https://stellar.expert/explorer/testnet/tx/87d17163405a84414649879a859904b05416f37a6695c1f708080c5ccea99405) |
| Rollback B→A, both members affected | 5407793 | [18524b37…](https://stellar.expert/explorer/testnet/tx/18524b37ecc6af823ca88ae81c1af38afccd82fbeadb12012f176cfb2c09e58d) |
| First member repointed to direct A | 5407796 | [ce990c0e…](https://stellar.expert/explorer/testnet/tx/ce990c0ea9f53d9aad500b772e4a997fb62e8928e6689c5de84827cadddc24d7) |
| Shared A→B, only second member affected | 5407797 | [a09957ea…](https://stellar.expert/explorer/testnet/tx/a09957ea7da0da7c922aee36819f37434625db8eac59dd4c9ef4a3de08a4d65b) |

Fifteen fixture transaction receipts were retrieved through Go, their Testnet
envelope hashes recomputed, successful results checked, and transaction metadata
decoded. Explorer URLs above are the CLI's transaction-link format; the checks
used RPC/XDR rather than claiming a separate explorer validation. Submitted
version calls' actual result metadata showed 1/1 after rollback and 1/2 after
direct detachment plus the next shared update. These are not simulation returns.

Initial snapshot capture remains PARTIAL because multi-call atomicity is not
proven. The diagnostic comparison named exactly both watched members. The
asserted baseline at 5407729 was pinned to its retrieved header; real metadata
at 5407730 reconstructed the initial shared update. A fresh process returned
ALREADY_RETAINED without modifying the database bytes.

The second baseline was 5407791. The resumed bounded watcher retained twelve
real ledgers through 5407803 and derived three findings: rollback with both
members, direct repoint of the first, and the next shared upgrade with only the
second. A separate offline reconstruction of the fetched metadata agreed.
Historical partial attempts produced 23 diagnostic alerts; those are not 23
additional upgrades. Reopening/rerunning the completed worker preserved the
outbox exactly.

## Failure and delivery checks

- A live future-target request produced a durable availability diagnostic;
  SIGTERM stopped the worker with exit 4 and restart subsequently reached its
  target. This is not a claim that a closed Testnet ledger was missing.
- A further read-only run proves cancellation after an actual committed prefix:
  baseline 5407826, one ledger accepted, SIGTERM retained 5407827, and restart
  completed through 5407830. No Testnet transaction was needed for this check.
- The opt-in local RPC replay omitted real closed ledger 5407793 from captured
  metadata. Ingestion stopped at 5407792 with MISSING_OR_UNEXPECTED_LEDGER.
  Cold reopen with the complete records repaired through 5407803; a repeat was
  ALREADY_RETAINED with stable outbox identities. This is local fault injection.
- A consistent backup of the real outbox delivered 26 messages (three findings
  and 23 historical diagnostics) to a local verified-TLS fixture. The receiver
  checked exact-body HMAC, IDs and unverified assurance. Reopening the delivered
  backup produced zero further attempts. External deliveries: zero.

Local evidence is retained in ignored `testnet-runs/live-upgrade-2` and
`testnet-runs/live-validation-2`; raw metadata, DBs, generated binaries and signing
configuration are not committed. The disposable signing identity was deleted
after the controlled writes. Repeat the documented workflow with a new identity
and fresh fixture fleet; Testnet retention/reset means old live state is temporary.

## Scope remaining

All baselines and source authentication remain UNVERIFIED; metadata commitments
and consensus/inclusion proofs are NOT_VERIFIED; fleet/history completeness is
UNKNOWN and final verdict UNDETERMINED. Separate read paths to the same endpoint
do not establish independent source trust. Controlled updates do not establish
maliciousness, signed release approval, exhaustive fleet discovery or production
delivery. The original sixteen-ledger bound remains. Protocol 30, continuous
operations/retention, deployment topology, release validation and Wave readiness
remain separate work.
