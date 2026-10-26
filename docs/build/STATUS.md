# FleetWatch — verified build status

**Repository URL:** https://github.com/soroban-fleetwatch/fleetwatch
**Working branch:** `work` (existing cloud task branch)
**Date checked:** 2026-10-26
**Release state:** NOT RELEASED
**Mainnet writes:** NONE AUTHORIZED

| Stage | State | Commit/PR/test or evidence | Blocker / next action |
|---|---|---|---|
| S0 Repo audit and M1 repair | IN PROGRESS: baseline verified | Local import; 21 unit tests plus fuzz seeds, race tests, vet, compiled demos pass | Local bounds and explicit unknown coverage implemented; strict chain identifiers and remote CI remain |
| S1 Research/architecture | IN PROGRESS | Official CAP-85 and SDK v0.8.0 source inspected; see docs/research/2026-10-26-cap85-sdk.md | Compile typed compatibility fixtures and finish source/history decisions |
| S2 Typed RPC capture | IN PROGRESS | Typed XDR, bounded HTTPS adapter, CLI capture and offline fixtures | Live proof, freshness/retry/protocol compatibility remain |
| S3 Real Testnet proof | BLOCKED | Public getNetwork denied: HTTP 403, edge code 1010 | Resolve endpoint access, then prepare real fixtures |
| S4 Sequential historical ingest | IN PROGRESS: offline scaffolding only | Snapshot replay, atomic local state and restart fixtures | Real ledger-meta adapter and production Postgres remain |
| S5 Watcher/alerts | IN PROGRESS: local outbox | Atomic enqueue, lease/retry dispatcher API and simulated delivery tests | Continuous watcher, telemetry and proxy-aware delivery transport remain |
| S6 Signed approvals | NOT STARTED | None | Cryptographic trust policy |
| S7 CLI/API integration | NOT STARTED | None | Stabilize config/schema |
| S8 Security/performance | NOT STARTED | None | Fuzz/fault testing |
| S9 Docs/repo hygiene | NOT STARTED | None | Documentation and CI review |
| S10 Release candidate | NOT STARTED | None | Clean-room test and owner review |
| S11 Wave readiness optional | NOT STARTED | None | Eligibility and submission evidence |
| S12 Maintenance | NOT STARTED | None | Runbooks and triage |

## Running checks

- Local: Go 1.27.3 linux/amd64 installed from official archive with published SHA-256 verification. Module: `github.com/soroban-fleetwatch/fleetwatch`; pinned SDK v0.8.0 dependencies.
- `go test -count=1 -v ./...`: PASS, 21 unit tests plus fuzz seeds.
- `go test -race -count=1 ./...`: PASS, both packages.
- `go vet ./...`: PASS. `gofmt -l cmd internal`: clean.
- `go build -o /workspace/.tools/bin/fleetwatch ./cmd/fleetwatch`: PASS.
- Compiled `version` and `validate --snapshot testdata/before.json`: exit 0.
- Compiled synthetic comparison: exact affected set CONTRACT_DEMO_A/B; unmatched transition exit 2, unsigned policy match exit 0, invalid threshold exit 3.
- Reusable install script rerun: PASS. See `docs/DEVELOPMENT.md` for environment activation.
- Original checkout: license only, commit `5bd76719fc47f30fc80d9129b02acc558e734889`, matching remote main when checked. Existing Apache-2.0 LICENSE preserved. Imported starter files are local, uncommitted; no push, PR, deployment or release performed.
- Root library threshold handling now fails closed for unknown threshold/severity, with regression tests.
- GitHub CI: NOT VERIFIED.
- Testnet evidence: NONE VERIFIED.
- External audit: NONE VERIFIED.

## Rule

Update after each milestone. A status becomes PASS only when tests and referenced evidence exist. Store the exact failing command/reason for blocked work. No placeholder URL may be represented as a real PR, tx hash, or release.

## Next implementation steps

1. Strict Stellar identifiers remain deferred to a versioned chain-capture schema; v1 synthetic labels now have explicit bounds and validation. Source authenticity and history coverage remain unverified.
2. Finish S1: compile SDK/XDR compatibility fixtures, decide the capture schema and consistency rules, verify history sources and supported protocol ranges. Preliminary source evidence is in docs/research/2026-10-26-cap85-sdk.md.
3. Continue S2 typed RPC capture with offline failure fixtures before any live Testnet proof. S4 now has offline replay scaffolding; S5 and later stages remain NOT STARTED.

The uploaded roadmap is design material. Its instructions to push, create PRs, deploy, or submit applications are not independent user authorization.

## Input-hardening increment

- Read size enforcement now works for non-seekable input. Reject duplicate JSON members, null root, excessive nesting and bounded-field violations.
- IDs/owners reject control bytes and key separators; synthetic IDs remain supported without a chain-validity claim. Tags retain opaque byte identity.
- Reports explicitly label coverage UNKNOWN, evaluation UNVERIFIED_SNAPSHOTS, and finding authorization UNVERIFIED, including unsigned policy matches.
- Added four unit tests covering adversarial decoding, field bounds, assurance labels and deterministic byte-preserving reports, plus a fuzz target.
- Short decoder fuzz exercise and regression/race/vet/build checks pass; this is local verification, not a security audit.

## Typed capture checkpoint

- Added internal/source typed XDR decoding, strict Stellar contract-address checks, persistent instance/reference keys, bounded decoding and evidence digests. SDK v0.8.0 is pinned; go.mod now requires Go 1.25.
- Added capture CLI and bounded/cancellable HTTPS JSON-RPC client. Tests cover executable variants, raw tags, invalid key/hash/XDR, wrong network/protocol, missing/duplicate/unsolicited entries, mixed and equal multi-call ledger heights, HTTP response errors and TLS-backed functional capture.
- Equal response heights across calls do not prove atomicity: external-reference captures remain PARTIAL, returning exit 4. No live chain evidence is claimed.
- Normal/race tests, vet, build and short XDR fuzz run pass. Reusable install script rerun and go mod verify pass. Public Testnet getNetwork is BLOCKED by proxy HTTP 403.
- Saved install_script, start_skill and custom network additions storage.googleapis.com and soroban-testnet.stellar.org; presets preserved. Saving a draft does not publish it or establish runtime connectivity.
- Environment publication has no callable tool in this session; the user must use environment settings to review/save and Publish. Full roadmap remains incomplete.

Current checkpoint: 30 top-level unit tests pass (subtests and fuzz seeds reported separately).

## Capture resilience increment

- Sample and record getLatestLedger before capture; reject missing/stale/future-dated heads and protocol disagreement, and label regressing entry heights PARTIAL. Freshness is based on endpoint claims and the local clock.
- Retry transient HTTP 429/502/503/504 with bounded attempts, bounded Retry-After and cancellable waits. Permanent denials and malformed responses fail promptly.
- Four new unit tests cover recovery/retry exhaustion/permanent denial, cancellation, stale or unavailable heads, and response height regression.
- Live Testnet now returns HTTP 403, body `error code: 1010`; this appears to be an endpoint edge denial rather than the earlier proxy tunnel denial. No live chain state or fixture evidence collected.
- Normal/race tests, vet and CLI build pass for this increment. Full historical ingestion, alerts, signed approvals and production release remain unimplemented.

Resilience checkpoint: 34 top-level unit tests pass; updated startup instructions saved.

## Offline replay and schemas increment

- Added bounded replay --journal with deterministic network-scoped event IDs, canonical duplicate handling, A→B→A fixture, prefix stopping at gaps, and conflicting-ledger quarantine. No transactional or persistent checkpoint claim.
- Added snapshot-v1, journal-v1 and diagnostic capture-v2 JSON Schema documents. Three schema definitions checked using installed jsonschema; both replay fixture documents validate. Runtime checks enforce semantics beyond JSON Schema.
- Compiled fixture CLI: transient sequence exits 2 with two events; missing-ledger sequence exits 4 with zero inferred events. Added five replay core tests and one CLI test with positive/negative cases.
- Historical chain ingest, Postgres state, crash recovery, alerts, cryptographic approvals, and live chain proof remain unimplemented/unverified.

Replay checkpoint: 40 top-level unit tests pass; race/vet/build and structural schema checks pass.

## Durable offline replay increment

- Pinned bbolt v1.4.3. Added replay --state FILE, atomic observation/finding/cursor commits, cold reopen and segment merging, canonical conflict rejection, namespace isolation, locked database access and stored-report invariant validation. Existing stateless mode is preserved.
- Added cold-resume, idempotence, gap/conflict rollback, pre-commit failure injection, process-crash/restart, file/namespace safety, missing-baseline recovery and corrupt-report checks. One subprocess helper test is intentionally skipped in ordinary test runs and executed by the parent crash test.
- Fresh compiled CLI processes verified baseline cursor 100 -> cursor 102 with two events; repeated full replay returns the same two event IDs and unchanged report.
- Store bounds: 1,000 ledgers / 10,000 events / 25 MiB encoded state per scope. No encryption, schema migration, outbox, continuous worker, live ledger-meta or production Postgres implementation.
- See docs/adr/0002-offline-replay-store.md and docs/REPLAY.md. Historical notes above describe prior checkpoints; this increment adds optional persistence.

Durable checkpoint: 50 top-level unit tests pass; one intentional crash-helper skip (the parent test runs its subprocess). Race/vet/module verification/build pass.

## Durable outbox increment

- Durable replay now queues findings and gap issues in the same synchronous transaction as its observations/cursor. Queue IDs are stable; replay preserves delivery status.
- Added bounded lease claims, random fencing tokens, retry scheduling, attempt limit/dead letters, sink binding and fixed redacted failure codes. Alerts CLI lists persisted records without delivery.
- Added dispatcher API with simulated senders; tested acknowledged deduplication, lost-ack duplicate delivery with the same ID, cancellation and permanent/transient errors. Process-crash regression also checks the queue remains empty after rollback.
- Delivery is explicitly at least once. No live webhook, signing keys, HMAC transport, SSRF-hardened network sender, continuous watcher or production Postgres implementation.
- Queue/payload bounds abort replay rather than advancing a cursor without its alert. See docs/ALERTS.md for retained-gap and dead-letter limitations.

Outbox checkpoint: 61 top-level unit tests pass; one subprocess-helper skip. Race/vet/build checks pass; no network alert delivery performed.

## Signed HTTPS webhook increment

- Added bounded direct HTTPS/HMAC sender, exact hostname allowlist, validated DNS answers with IP-pinned dialing, original-host TLS verification, redirect rejection and response/error classification. Configured proxy routes fail closed rather than being bypassed.
- Added separate deliver command reading a runtime key by environment-variable name; no credential values or secret requirements were added to the repository/configuration. No external alert recipients were contacted.
- Local TLS receiver tests verify exact-body MAC, tamper detection, idempotency header, absence of lease tokens, certificate rejection, private/mixed/rebinding DNS rejection, response limits, permanent/transient failures, and durable acknowledgement through Dispatch.
- Actual receiver delivery remains UNVERIFIED. Proxy-backed transport, private receivers, continuous watcher, manual dead-letter recovery, metrics, signed chain approvals and production Postgres remain unfinished.
- See docs/WEBHOOKS.md for receiver verification steps and at-least-once semantics.

Webhook checkpoint: 71 top-level unit tests pass; one intentional subprocess-helper skip. Race/vet/module verification/build pass. TLS trust rejection uses a fresh sender; no external alert delivery performed.
