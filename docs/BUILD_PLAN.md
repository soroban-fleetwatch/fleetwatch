> **Historical M1 plan only.** Superseded for future engineering by `docs/build/END_TO_END_BUILD_PLAN.md`; this file remains as a record of the initial prototype.

# Soroban FleetWatch — staged build plan

**Status:** M0/M1 local prototype complete (only once the included tests pass). No network reader, GitHub organization, deployment, or official Wave submission has been created.

Dripwave adaptation: phases 1–3 = evidence and differentiation; phase 4 = tentative name/one repo; phase 5 = **off-chain data architecture instead of smart-contract design**; phases 6–7 = codex/agent instructions; phases 8–9 = CLI and eventually observer service deployment; phases 10–11 = repository and docs hygiene; phases 12–13 = eligibility check, submission, and maintenance. A Soroban contract is deliberately **not** required for an off-chain monitor.

## Problem statement

A contract fleet sharing a CAP-85 managed executable can change code simultaneously when a tagged Wasm reference owned by a manager contract is updated. Security teams need a reliable account of what changed, which contracts were affected, and whether the change matched an approved release.

## Users

1. Soroban protocol maintainer monitoring managed contract families.
2. Incident responder reconstructing blast radius after an unexpected shared reference update.
3. Release engineer checking that observed production state matches a reviewed upgrade.

## Design decisions

- Go core and CLI; Rust only if fixtures/test contracts require it.
- One repository, because there is no smart-contract component in the MVP.
- Standalone by default; do not couple functionality to SoroIndex or the other four projects.
- Snapshot-diff core is independent of source, so later adapters can consume Stellar RPC or ledger archives.
- External executable identity is `(owner, opaque tag bytes)`; the hash is resolved at a ledger.
- No `admin` identity claims until role/authorization evidence is available.
- No automatic claims that an approval file is cryptographically trustworthy.

## Dataflow

```text
CAP-85-capable Stellar network
     │ [M2: trusted collection at exact ledger sequence]
     ▼
Normalizer → contract/executable/tag snapshots → diff engine → approval matcher
                                                     │
                                                     ├─ CLI / JSON / CI [M1]
                                                     └─ alert event sink [M4]
```

## Milestones

### M0 — Evidence and scope (complete)
- Read the implemented CAP-85 protocol specification.
- Compare to pre-deployment Wasm analysis tools; focus on observed post-deployment fleet changes.
- Choose no on-chain contract for MVP. Define immutable byte representation for tag keys.
- Publish claim limitations, input security model, and source links.

### M1 — Local detection engine and CLI (working prototype)
- Implement strictly decoded schema-v1 JSON snapshots and approval policy.
- Validate duplicates, invalid hashes, opaque tag bytes and cross-network comparisons.
- Detect hash transitions and calculate sorted fleet impact.
- Flag direct executable repointing and observation gaps separately.
- Support deterministic machine-readable JSON and CLI exit status thresholds.
- Add unit tests, CI config and two-contract synthetic demo.
- **Acceptance:** `go test ./...`, `go vet ./...`, and both approved/unapproved demo scenarios pass.

### M2 — Read real Stellar state (not started)
- Pin and verify a CAP-85-compatible release of the official Stellar Go SDK.
- Add adapter to fetch contract instances and `SCV_EXECUTABLE_TAG` persistent owner entries via Stellar RPC `getLedgerEntries` at a consistent ledger.
- Do **not** assume arbitrary historical ledger lookup from current-state RPC; consult actual retention and RPC method semantics.
- Provide explicit partial-capture status, retries, bounded concurrency, and network-passphrase checks.
- Generate verified Testnet fixture with an actual external-reference manager and two contracts.
- **Acceptance:** demonstrate actual fleet change detection on Testnet, with transaction/ledger evidence.

### M3 — Durable history and integrity (not started)
- Add ledger-ingestion/backfill source as needed for transitions between snapshots.
- Track ledger checkpoints, idempotency, data completeness, contract and reference changes, storage expiry.
- Persist normalized observations with provenance in PostgreSQL or lightweight local DB after benchmarking.
- **Acceptance:** no missing transitions in a bounded historical replay fixture; restart recovery proven.

### M4 — Continuous watcher and alerts (not started)
- Add scheduling/stream ingest with retries and durable event queue.
- Add webhook/Slack/incident platform sinks with deduplication, severity and context.
- Rate-limit and authenticate alert configuration.
- **Acceptance:** local orchestrated Testnet demonstration triggers one alert per observed change.

### M5 — Approval provenance and risk enrichment (not started)
- Signed release approvals, trusted signer roster, key rotation, and revocation/expiry handling.
- Link code hashes to independently verified builds and audits (without copying integrity monitor #3).
- Identify observable upgrade controllers when authority source is available, labeled with confidence.
- **Acceptance:** forged/expired approval fixtures rejected, incident evidence reproducible.

### M6 — Productization and release (not started)
- Threat model review, fuzz tests, profiling, replay/chaos testing, and external security review.
- Docker image if hosting requires it; release binaries and compatibility matrix.
- API/OpenAPI, docs site, alert runbooks, on-call procedures, community contributor issues.
- Check current Drips Wave acceptance list and criteria immediately before any application.
- **Acceptance:** repeatable build and deployment process, CI checks and reviewed release.

## Repo structure

```text
cmd/fleetwatch/              command-line entrypoint
internal/fleetwatch/         normalized data model, comparator, policy evaluation
 testdata/                  synthetic captures and approval examples
 docs/                      specs, build plan and threat model
 .github/workflows/          CI
 AGENTS.md                   Codex task instructions
```

## Planned issues (to create later in the new GitHub organization)

1. feat(ingest): pin Stellar Go SDK and decode CAP-85 external-ref instance entries.
2. feat(ingest): resolve executable owner/tag via typed XDR ledger keys.
3. test(fixtures): deploy CAP-85 Testnet fixtures and record verifiable ledger evidence.
4. feat(ingest): validate consistent ledger-sequence capture and partial coverage.
5. feat(storage): implement append-only observation history and replay checkpoints.
6. feat(detector): track intermediate changes and identify reversible transitions.
7. feat(policy): support signed, expiring, reviewer-approved changes.
8. feat(alerts): add durable deduplicated webhook notifications.
9. test(security): add fuzz tests and adversarial input corpora.
10. docs(wave): prepare verifiable demonstration and accurate project submission evidence.

## Definition of done for v0.1 (local prototype)

M0/M1 code and tests pass, the warnings above are visible in docs and output, the demo works offline, and no one represents snapshots as authentic live ledger state. Live Testnet monitoring is a **separate milestone**.
