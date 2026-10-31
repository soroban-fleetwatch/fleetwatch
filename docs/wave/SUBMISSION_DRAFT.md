# Drips Stellar Wave — submission draft

**State: owner-facing draft.** Prepared 2026-10-28; updated 2026-10-31 after
the repository went public and the pre-release and docs site were published.
Submitting a repository to a Wave program is the owner's decision and action;
the agent has not submitted anything.

## 1. Eligibility and status check

| Check | Result | Evidence |
| --- | --- | --- |
| Repository already approved in the Stellar Wave? | Not found | Web search on 2026-10-28 found no listing. The live list at <https://www.drips.network/wave/stellar/repos> could not be fetched from this session (host not in the network allowlist), so this is **not verified**. The repository was private until 2026-10-31. |
| Repository public | Yes | GitHub API, 2026-10-31: `"private": false`; topics set. |
| Program rules (onboarding, application cap, KYC) | Not verified | Search results say repositories are applied through an onboarded GitHub organization, organizers approve or reject each one, rejected repos can be appealed but not re-applied, and recent waves cap applications per wave and require KYC. These came from third-party pages and the Drips changelog. Re-read <https://docs.drips.network/wave/maintainers/participating-in-a-wave/> before applying. |

## 2. Link inventory

Only links that exist today are listed. Missing items are blockers in section 6.

| Item | Link | Status |
| --- | --- | --- |
| Repository | <https://github.com/soroban-fleetwatch/fleetwatch> | Public |
| CI | <https://github.com/soroban-fleetwatch/fleetwatch/actions/workflows/ci.yml> | Green on `73f92e2` (run 35); re-check on the merged head |
| Testnet proof: shared upgrade A→B | <https://stellar.expert/explorer/testnet/tx/87d17163405a84414649879a859904b05416f37a6695c1f708080c5ccea99405> | Recorded 2026-10-27; Testnet resets will remove it |
| Testnet proof: rollback B→A | <https://stellar.expert/explorer/testnet/tx/18524b37ecc6af823ca88ae81c1af38afccd82fbeadb12012f176cfb2c09e58d> | Same |
| Testnet proof: direct repoint | <https://stellar.expert/explorer/testnet/tx/ce990c0ea9f53d9aad500b772e4a997fb62e8928e6689c5de84827cadddc24d7> | Same |
| Testnet proof: upgrade after repoint | <https://stellar.expert/explorer/testnet/tx/a09957ea7da0da7c922aee36819f37434625db8eac59dd4c9ef4a3de08a4d65b> | Same |
| Evidence record | [`docs/research/2026-10-27-testnet-validation.md`](../research/2026-10-27-testnet-validation.md) | In repo |
| Documentation site | <https://soroban-fleetwatch.github.io/fleetwatch/> | Live; deployed by the "Docs site" workflow (run 41169677170, 2026-10-31) |
| Live app | None. FleetWatch is a CLI and single-host service. | Not applicable; see section 3 |
| Demo video | Offline terminal recording produced 2026-10-28 (not committed) | **Not uploaded**; a live Testnet segment still needs recording, see [DEMO_SCRIPT.md](DEMO_SCRIPT.md) |
| Planned issues | <https://github.com/soroban-fleetwatch/fleetwatch/issues> (#3–#22) | Created 2026-10-28 |
| Release | <https://github.com/soroban-fleetwatch/fleetwatch/releases/tag/v0.1.0-rc.1> | Pre-release published 2026-10-31 |

The explorer links come from the fixture run. The transactions were checked
through RPC and XDR at the time, not through the explorer. They were not
re-checked in this session because the Testnet RPC host is not in this
session's network allowlist.

## 3. Repository relationship

One repository. FleetWatch is an off-chain Go monitor; it has no product smart
contract and needs none. It reads public ledger data over Stellar RPC and never
signs or submits transactions. The `contracts/testfixtures` Rust workspace
builds three small test-only contracts (an owner and two member versions) used
to create real CAP-85 changes on Testnet for validation. Those fixture
addresses are test evidence, not deployed product contracts.

## 4. Project description (form text)

> Protocol 28 added CAP-85, which lets many Soroban contracts point at one
> owner-controlled code reference instead of a fixed Wasm hash. One write to
> that reference upgrades every contract that uses it in the same ledger.
> FleetWatch is an open-source Go monitor that watches those references on the
> ledger. It reads ledger-close metadata over Stellar RPC, checks header hashes
> and their links, rebuilds the executable of each watched contract after every
> operation, and reports each change with the old and new Wasm hash, the
> affected contracts and the evidence behind it. It catches changes that a
> before/after snapshot would miss, including an upgrade that is rolled back
> within the same transaction. Findings can be checked against Ed25519-signed
> approvals and sent to webhooks with HMAC signatures. On Testnet it detected a
> real two-contract fleet upgrade, its rollback and a direct repoint from live
> metadata. It is experimental and unaudited, and it labels what it cannot yet
> prove: source authentication, metadata commitments and fleet completeness.

## 5. Planned issues, by area

Created on GitHub on 2026-10-28 as issues #3–#22, from `docs/build/issues.json`.

- **Source trust (L, security):** cross-check ledger headers across two RPC
  endpoints; verify transaction-set and result hashes against the header.
- **Baselines and history (M–L):** build a pinned baseline from
  `getLedgerEntries`; model archival and restore of watched entries; Protocol
  30 decoding gate; range-limited fleet discovery.
- **Continuous service (M–L):** evidence-preserving retention rollover;
  versioned bbolt migrations; representative-load benchmark; fuzz targets for
  follow config and archives.
- **Approvals (M):** bind signed approvals to follow findings; trust-root
  distribution and rotation ADR.
- **CLI and alerts (S–M):** `fleetwatch init`; version stamping; chat-friendly
  alert summary.
- **Release and docs (S–M):** non-publishing release dry run; manual Testnet
  smoke workflow; metric alert rules; end-to-end Testnet tutorial; API decision ADR.

Four issues are marked `good first issue`: #12, #13, #14 and #21.

## 6. Blockers before submission

| # | Blocker | Who | How to clear |
| --- | --- | --- | --- |
| 1 | ~~Repository is private~~ | Done | Public since 2026-10-31. A pattern scan of all history found no keys or tokens beforehand. |
| 2 | ~~Phase 8–12 work is not on `main`~~ | Done | Merged in #2, #23 and #24. |
| 3 | Demo video not published | Owner | An offline recording exists. Upload it (YouTube or a release asset), and ideally add the live Testnet segment from [DEMO_SCRIPT.md](DEMO_SCRIPT.md). |
| 4 | ~~Docs site not hosted~~ | Done | <https://soroban-fleetwatch.github.io/fleetwatch/> |
| 5 | ~~Planned issues not created~~ | Done | Issues #3–#22 created 2026-10-28. |
| 6 | Branch protection and private vulnerability reporting not verified | Owner | Topics are set. Confirm the other settings in [Repository settings](../REPOSITORY_SETTINGS.md). |
| 7 | Dripwave Phase 9 hosting in progress | Owner + agent | An Oracle Cloud Always Free account (Johannesburg) was created on 2026-10-31; the host is not yet provisioned or verified. See [Phase 9 acceptance](../PHASE9_ACCEPTANCE.md). The skill treats a working deployment as part of an approved-grade repo; FleetWatch has no web app, so explain the CLI/service model and link the Testnet evidence. |
| 8 | ~~Release tag not created~~ | Done | `v0.1.0-rc.1` pre-release published 2026-10-31. |
| 9 | Live Wave rules and list not re-checked | Owner or a session with `drips.network` allowed | Re-check eligibility, caps and KYC on the day of applying. |

## 7. What not to claim

- No audit, no production deployment, no Mainnet monitoring in operation.
- No Wave approval, grant amount or contributor numbers.
- Testnet fixture addresses are not product contracts.
