# Soroban FleetWatch

[![Go CI](https://github.com/soroban-fleetwatch/fleetwatch/actions/workflows/ci.yml/badge.svg)](https://github.com/soroban-fleetwatch/fleetwatch/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
![Status: experimental](https://img.shields.io/badge/status-experimental-orange.svg)
![Not audited](https://img.shields.io/badge/security%20audit-none-lightgrey.svg)

**An off-chain monitor for Soroban contract fleets that share code through CAP-85 executable references.**

Protocol 28 added CAP-85: a contract's executable can be a reference to
`{owner contract, opaque tag}` instead of a fixed Wasm hash. The owner keeps the
current Wasm hash for each tag in one persistent ledger entry. Change that entry
once, and every contract pointing at the tag runs new code in the same ledger.

That is useful for upgrading a fleet. It is also a single point where an
unexpected or unauthorized change reaches every member at once. FleetWatch
watches those references on the ledger and answers four questions:

1. Did a shared reference change, and from which Wasm hash to which?
2. Which watched contracts were affected (the blast radius)?
3. Does the change match an approval you declared or signed in advance?
4. Is there a gap in what FleetWatch actually observed?

FleetWatch is a Go command-line tool and single-host service. It needs no wallet,
no private key and no on-chain contract of its own.

> **Status: experimental prototype, not audited.** Testnet behavior has been
> demonstrated with real transactions (below). Every report still labels source
> authentication `UNVERIFIED`, metadata commitments `NOT_VERIFIED`, fleet
> coverage `UNKNOWN` and the final chain verdict `UNDETERMINED`. Do not use it
> as the only protection for contracts that hold real value.

## Testnet proof

On 2026-10-27 a controlled CAP-85 fleet was deployed on Stellar Testnet: one
owner contract, tag `fleetwatch-demo` (hex `666c65657477617463682d64656d6f`) and
two member contracts. FleetWatch ingested the real ledger metadata and reported
each change.

| Action | Ledger | Transaction | FleetWatch result |
| --- | --- | --- | --- |
| Shared tag A→B | 5407730 | [87d17163…](https://stellar.expert/explorer/testnet/tx/87d17163405a84414649879a859904b05416f37a6695c1f708080c5ccea99405) | Both members affected |
| Rollback B→A | 5407793 | [18524b37…](https://stellar.expert/explorer/testnet/tx/18524b37ecc6af823ca88ae81c1af38afccd82fbeadb12012f176cfb2c09e58d) | Both members affected |
| Member 1 repointed to direct Wasm A | 5407796 | [ce990c0e…](https://stellar.expert/explorer/testnet/tx/ce990c0ea9f53d9aad500b772e4a997fb62e8928e6689c5de84827cadddc24d7) | Repoint finding for member 1 |
| Shared tag A→B again | 5407797 | [a09957ea…](https://stellar.expert/explorer/testnet/tx/a09957ea7da0da7c922aee36819f37434625db8eac59dd4c9ef4a3de08a4d65b) | Only member 2 affected |

Owner `CCSC3UOXCCNB3VV7MM2P63IPNVKHQQ3TUMBCIW6OW42F75FBGMI4Y3VA`, members
`CDL6H5M2VYLOR536DDUG7SQXKJNLSADSIKNNMH5EW3R5R55QZJ2KXOZY` and
`CD6GZKPCBF2OFUV7VAGBFMWM4H2S2BBPI66CQ674RMFQAVLSVZOAWVSM`. These are disposable
test fixtures, not product contracts. Testnet resets periodically, so these
links will eventually stop resolving. Full evidence, including restart and gap
recovery, is in [the validation record](docs/research/2026-10-27-testnet-validation.md).
To repeat it with a fresh fleet, follow [TESTNET_DEMO.md](docs/TESTNET_DEMO.md).

## Quick start

Requires Go 1.25 or later; CI pins Go `1.27.3` (see `.go-version`).

```sh
git clone https://github.com/soroban-fleetwatch/fleetwatch.git
cd fleetwatch
go test ./...
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch help
```

Compare two offline snapshots. The synthetic demo moves tag `release-stable`
from hash `aa…aa` to `bb…bb`:

```sh
./fleetwatch compare --before testdata/before.json --after testdata/after.json
# [CRITICAL] UNAPPROVED_REFERENCE_CHANGE: ...
#   owner=OWNER_DEMO tag_hex=72656c656173652d737461626c65
#   affected=CONTRACT_DEMO_A,CONTRACT_DEMO_B
# exit status 2

./fleetwatch compare --before testdata/before.json --after testdata/after.json \
  --policy testdata/approved-policy.json
# [INFO] POLICY_MATCHED_REFERENCE_CHANGE ...
# exit status 0
```

Reconstruct a watched fleet from captured ledger metadata, offline:

```sh
./fleetwatch reconstruct \
  --baseline testdata/reconstruction-baseline.json \
  --capture testdata/reconstruction-capture.json
# JSON report with per-ledger findings; exit status 2 because findings exist
```

Continuous single-host operation uses `follow --config FILE`. See
[FOLLOW.md](docs/FOLLOW.md) and [OPERATIONS.md](docs/OPERATIONS.md).

## How it works

```text
 Stellar RPC (HTTPS, read-only)
   getLedgers / getLedgerEntries / getLatestLedger / getNetwork
            │
            ▼
 source adapters ── strict JSON + bounded XDR decoding, network/protocol checks,
            │        header hash and previous-header linkage
            ▼
 history projection ── executable and CAP-85 tag entries, in transaction-phase order
            │
            ▼
 watched reconstruction ── operator-asserted baseline + metadata → state per operation
            │
            ├──► findings: reference change, repoint, gap, unresolved reference
            │
            ▼
 local bbolt store ── raw evidence, cursor, findings and outbox in one transaction
            │
            ├──► doctor / report / checkpoint (read-only inspection)
            ├──► signed approvals (Ed25519, scoped trust roots) — kept separate
            └──► HMAC-signed HTTPS webhooks (at-least-once, dead letters, audit)
```

Source adapters are separate from the pure comparison code. Tag bytes are kept
exactly as raw bytes (shown as hex). A matched unsigned policy is never reported
as a signed approval. Missing data produces a gap finding, never a clean verdict.
Read more in [ARCHITECTURE.md](docs/ARCHITECTURE.md) and the
[threat model](docs/THREAT_MODEL.md).

## Commands

Run `fleetwatch help` for full flags, or `fleetwatch COMMAND -h` for one command.

| Area | Commands |
| --- | --- |
| Offline snapshots | `validate`, `compare`, `replay`, `report` |
| Ledger metadata | `capture`, `history`, `changes`, `reconstruct`, `checkpoint`, `ingest`, `watch` |
| Continuous service | `follow`, `follow-inspect`, `follow-archive`, `verify-follow-archive`, `restore-follow-archive`, `follow-compact`, `follow-sync` |
| Alerts | `alerts`, `deliver`, `deliver-loop`, `requeue`, `recoveries` |
| Approvals | `verify-approval`, `bind-approval` |
| Maintenance | `doctor`, `backup`, `version`, `help` |

| Exit code | Meaning |
| --- | --- |
| 0 | Success (including help) |
| 2 | Finding at or above the `--fail-on` threshold, or delivery attention needed |
| 3 | Invalid input or usage |
| 4 | Source unavailable or observation incomplete |
| 5 | State or output failure |
| 6 | Follow retention capacity reached |

The full reference is in [CLI.md](docs/CLI.md).

## Detection rules (snapshot comparison)

| Rule | Severity | Meaning |
| --- | --- | --- |
| `UNAPPROVED_REFERENCE_CHANGE` | Critical | Shared reference changed with no exact policy match |
| `POLICY_MATCHED_REFERENCE_CHANGE` | Info | Change matches the unsigned policy file (not a signature) |
| `CONTRACT_EXECUTABLE_REPOINTED` | High | Contract changed executable kind, hash, or owner/tag |
| `REFERENCE_NOT_OBSERVED` | High | A reference seen earlier is missing from a later snapshot |
| `UNRESOLVED_EXTERNAL_REFERENCE` | High | A contract points to an owner/tag with no observed value |
| `CONTRACT_NOT_OBSERVED` | High | A contract seen earlier is missing from a later snapshot |
| `NEW_REFERENCE_OBSERVED` | Info | First observation of a reference |
| `NEW_CONTRACT_OBSERVED` | Info | First observation of a contract |

Missing input is treated as a collection gap, not as deletion.

## What is not done yet

- Independent authentication of RPC data, baselines and metadata commitments.
- Full fleet discovery. FleetWatch watches the contracts you list.
- A permanent hosted deployment. Dripwave Phase 9 is blocked on a zero-cost
  host; see [PHASE9_ACCEPTANCE.md](docs/PHASE9_ACCEPTANCE.md).
- Production retention beyond the documented per-stream limits.
- A stable release and an external security review. `v0.1.0-rc.1` is a pre-release.

Current state, commands run and evidence for every milestone are in
[docs/build/STATUS.md](docs/build/STATUS.md).

## Documentation

Read the documentation at <https://soroban-fleetwatch.github.io/fleetwatch/>.
Its source is in [`docs/`](docs/index.md) and builds with MkDocs
(`mkdocs build --strict`).

## Contributing

Issues are scoped with acceptance criteria and labels for complexity. Read
[CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request. Every change
needs tests, and claims about chain behavior need recorded evidence.

## Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
This project has had no external security audit.

## Contributors

<a href="https://github.com/soroban-fleetwatch/fleetwatch/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=soroban-fleetwatch/fleetwatch" alt="Contributors" />
</a>

## References

- [CAP-85: Externally managed contract executables](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0085.md)
- [Introducing Adapter, Protocol 28 on Stellar](https://stellar.org/blog/developers/introducing-adapter-protocol-28-on-stellar)
- [Stellar Go SDK v0.8.0](https://github.com/stellar/go-stellar-sdk)

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
