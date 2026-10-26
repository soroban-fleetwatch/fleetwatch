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
| S3 Real Testnet proof | IN PROGRESS: read-only metadata evidence | Native Go collected/decoded two protocol-29 Testnet ledgers; historical endpoint denial resolved for this client | Controlled CAP-85 deployment/update and actual fleet fixtures remain |
| S4 Sequential historical ingest | IN PROGRESS: collector and offline scaffolding | Bounded getLedgers collection, snapshot replay, atomic local state and restart fixtures | Metadata commitments, change application, trustworthy baseline and production Postgres remain |
| S5 Watcher/alerts | IN PROGRESS: local outbox and diagnostics | Atomic enqueue, lease/retry dispatcher, local TLS delivery tests and read-only health report | Continuous watcher, live telemetry and proxy-aware delivery transport remain |
| S6 Signed approvals | IN PROGRESS: standalone verifier | Ed25519 statement/trust/threshold tests and public synthetic vector | Signed trust distribution, trustworthy chain evidence and policy integration remain |
| S7 CLI/API integration | IN PROGRESS: experimental CLI | Capture/replay/outbox/approval commands, doctor and at-ledger fleet report with JSON Schemas | Stable configuration, full help/exit compatibility and optional API decision remain |
| S8 Security/performance | NOT STARTED | None | Fuzz/fault testing |
| S9 Docs/repo hygiene | NOT STARTED | None | Documentation and CI review |
| S10 Release candidate | NOT STARTED | None | Clean-room test and owner review |
| S11 Wave readiness optional | NOT STARTED | None | Eligibility and submission evidence |
| S12 Maintenance | NOT STARTED | None | Runbooks and triage |

## Initial baseline checks (historical)

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

1. Extend diagnostic ledger metadata collection into sequential change application: trustworthy baseline, transaction/fee/upgrade/eviction/restore ordering, raw evidence, exact member/reference reconstruction and within-ledger transient changes. Never relabel current-state polling or supplied snapshots as comprehensive chain history.
2. Decide independently trusted header anchoring, metadata/transaction commitment verification and provenance requirements before allowing chain-safe verdicts or approval-based severity changes. Source authentication and fleet completeness remain unverified.
3. Prepare controlled CAP-85 Testnet contract fixtures and actual update evidence. Read-only metadata retrieval now works, but no contract deployment/transaction was performed and the full Testnet acceptance gate is not passed.
4. Implement continuous worker/configuration, production storage, operational recovery, trust distribution and release validation. Existing local replay, outbox, approval, report and backup features remain experimental.

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

## Offline signed approval increment

- Added custom v1 domain-separated Ed25519 envelopes, exact network/owner/tag/hash binding, reviewed artifact digest, ledger validity, mandatory nonce, scoped public trust roots, distinct-signer thresholds, revocation and rotation.
- Added verify-approval CLI. Unsigned policies cannot be loaded as signed envelopes. Existing compare/replay findings and outbox authorization remain unchanged; final chain verdict stays UNDETERMINED for every currently supported observation source.
- Shared strict JSON input now serves snapshots and approval files, rejecting case-alias duplicate fields and invalid raw UTF-8 while retaining existing size/depth/unknown-field checks.
- Added public-only synthetic envelope/trust/observation/signing vector; its temporary private key was not saved. Three new JSON Schemas and fixtures validate. No production key or external signing action.
- Tests cover valid/forged/unknown signatures, every signed field, network/transition mismatch, ledger expiry, incomplete input, revocation/rotation and distinct threshold enforcement.
- Nonce consumption, durable incident binding, signed trust distribution, independently authenticated chain evidence and severity-downgrade integration remain unfinished. See docs/APPROVALS.md and ADR 0003.

Approval checkpoint: 83 top-level unit tests pass; one intentional crash-helper skip. Race/vet/module verification/build pass. Compiled public synthetic checks return 0 verified-for-fields, 4 partial, 2 revoked/tampered, with final verdict UNDETERMINED in every case.

## Durable approval association increment

- Added bind-approval: derives a transition from a persisted incident, rechecks signatures/current scoped trust, and atomically saves a public receipt and owner/network nonce reservation. Source/tag changes cannot bypass that reservation within the same database.
- Exact repeat binding is idempotent after reopen. Another incident cannot reuse the nonce; another statement/trust version cannot silently replace an event receipt. Incomplete or rejected observations do not reserve it.
- Tests cover concurrent competition, revocation after binding, gap repair, source switching, corruption rejection, order-independent trust identity, transactional rollback and pre-commit process crash.
- Findings and outbox authorization/severity are unchanged; all final chain verdicts stay UNDETERMINED. No external signing, production keys, transactions or alert sends.
- Added a synthetic binding journal and receipt JSON Schema. Replay protection is local to a single DB history; distributed coordination, binding recovery/migration, authenticated chain evidence, trust distribution and production storage remain unfinished. See docs/APPROVAL_BINDING.md.

Binding checkpoint: 95 top-level unit tests pass; one intentional crash-helper skip. Race/vet/module verification/build pass. Fresh compiled processes verify first bind, idempotent retry and nonce-reuse rejection; positive receipts validate against JSON Schema.

## Read-only health increment and requested push

- User explicitly authorized pushing the saved work. `git push -u origin work` failed with HTTP 403: `remote: Write access to repository not granted.` No remote branch update, PR, release or merge succeeded. Existing local checkpoint before this increment was `d968527`; repository write access through the configured GitHub integration is required to push it and subsequent commits. Do not replace platform authentication or request raw tokens in chat.
- Added `doctor --state FILE --network TEXT [--source synthetic|user_snapshot]`, using a read-only database open and one consistent transaction. It validates retained observations/findings, coverage diagnostics, outbox event correspondence and issue identities, and counts pending/scheduled/inflight/expired/delivered/dead records.
- Missing scope and incomplete sequence return 4; delivery problems return 2; unavailable/locked/invalid database returns 5 without a success report. Missing files are not created. Tests verify database bytes remain unchanged and write operations on the read-only handle fail.
- Repaired gaps stop affecting current coverage while historical alerts remain counted. Active final-attempt leases may finish; exhausted expired attempts require attention. No retry, acknowledgment, reclamation, alert send or remote source request occurs.
- `OK_OFFLINE` is a local consistency status, not an absence-of-findings or chain-safety verdict. All reports retain UNKNOWN chain history, UNVERIFIED_INPUT evaluation, UNDETERMINED final verdict and null ledger lag. Approval-index checks, continuous monitoring, live metrics, backup verification and authenticated historical ingestion remain unfinished.
- Added docs/HEALTH.md and schema/health-v1.json. Fresh compiled CLI checks verify gap exit 4, repair exit 0, absent scope exit 4, missing-file exit 5, byte preservation and schema-valid JSON.

Health checkpoint: 101 top-level unit tests pass; one intentional crash-helper skip (parent invokes its subprocess). Full race suite passed; affected store/CLI race checks rerun after final adjustments. Vet, module verification, build, gofmt and diff checks pass. No new runtime dependency, service, secret or startup prerequisite was introduced; existing reusable environment setup remains applicable.

## At-ledger observed fleet report and successful push

- On the user's renewed push request, `git push -u origin work` succeeded. GitHub created the `work` branch with checkpoint `09260f9` and local upstream tracking. The earlier write-access blocker above is resolved. No force-push, merge, PR creation or release was performed; GitHub Actions results have not been verified.
- Added `report --state FILE --network TEXT --ledger N --owner ID --tag-hex HEX [--source synthetic|user_snapshot]`. The read-only query requires the exact retained ledger and returns observed reference hash/members, canonical snapshot digests, adjacent membership differences and related persisted replay event IDs.
- Joined/left/retained/union sets keep input membership changes separate from comprehensive impact claims. Repoints into and out of a reference are included; unrelated events are excluded. Baselines have no fabricated previous observation; gaps and out-of-range queries never interpolate or query unprocessed suffixes.
- All reports retain UNKNOWN membership coverage, OBSERVED_SNAPSHOT_MEMBERS scope, UNVERIFIED_INPUT evaluation and UNDETERMINED final verdict. Missing reference observations retain known members and a null hash. Successful empty member queries do not establish an empty on-chain fleet. Query exit 0 is success even when related findings are critical; missing/unresolved/partial input returns 4.
- Tests cover raw invalid-UTF8/NUL tag bytes represented as hex, hex case equivalence without byte normalization, adjacent membership changes, cold reopen, source/network isolation, corrupt findings, missing baselines/ledgers/references, output failure and read-only byte preservation. Added docs/FLEET_REPORT.md and schema/fleet-report-v1.schema.json; no new dependencies, service or environment startup prerequisites.
- `go test -timeout=45s -count=1 -json ./...`: 109 top-level unit tests PASS; one intentional crash-helper skip (parent runs it as a subprocess). `go vet ./...`, `go mod verify`, CLI build and `go test -timeout=60s -race -count=1 ./...`: PASS. Fresh compiled CLI verifies A→B→A queries, related event identity, baseline, missing scope/ledger, gap, unresolved reference, empty membership and raw tag cases; all output documents validate against JSON Schema and all queried DB bytes remain unchanged.
- FW-019 now has a bounded offline implementation; authenticated ledger/transaction evidence, complete fleet discovery, real ledger-meta ingestion, production storage and continuous watch remain unfinished. Remote CI and live Testnet proof remain unverified.

## Consistent local backup and recovery increment

- At the start of this request, `git push origin work` reported Everything up-to-date at `88522f0`. The earlier fleet-report increment is saved on GitHub's work branch. No PR, merge or release performed; remote CI has not been verified.
- Added `backup --state FILE --output NEW_FILE`. It opens an existing database read only and copies all namespaces in one bbolt transaction snapshot, preserving replay state, outbox delivery/retry/lease records, approval receipts, nonce reservations and opaque future buckets together.
- Snapshot publication uses a mode-0600 temporary file in the destination directory, file fsync/close, a non-overwriting hard link, and parent-directory fsync. Existing files/directories/symlinks (including a destination introduced during copying) and the source are not replaced. Ordinary pre-publication failures clean up temporary files. A published file is retained if later synchronization or receipt output fails; exit 5 is explicitly ambiguous about destination existence.
- Bounds: 512 MiB transaction image; CLI two-minute cancellation deadline checked between writes and before publication. Local filesystems must support hard links/advisory locks/fsync; kernel-blocked filesystem calls cannot be interrupted by these checks. Abrupt process death can leave an unpublished temporary file. No hardware power-loss test or external storage validation was performed.
- Receipt gives exact snapshot byte count and SHA-256, BBOLT_TRANSACTION_SNAPSHOT format and NOT_PERFORMED semantic validation. Backup is not a whole-database semantic validator, encryption, compaction, provenance proof or a global nonce registry. Restoring older history still requires reconciliation of later reservations/acknowledgments; tests explicitly show that older snapshots cannot know later nonce use.
- Tests verify cold reopen and recovery of cursor/findings, all delivery states, retry metadata, active fencing tokens, idempotent approval receipts and same-/cross-source nonce rejection. Failure tests cover copy bounds, cancellation, pre-publication abort, destination races, existing paths, output failure and byte-preserving read-only source access. Added docs/BACKUP.md, a backup-receipt JSON Schema, recovery guidance, and ignores for local database/backup artifacts.
- `go test -timeout=45s -count=1 -json ./...`: 115 top-level unit tests PASS; one intentional crash-helper skip (parent runs its subprocess). Vet, module verification, CLI build and full race suite PASS. Targeted backup tests passed after adding the fencing-token assertion. Fresh compiled CLI backup-to-recovery drill verified SHA-256/size/mode, receipt schema, unchanged source bytes, restored doctor status, original receipt identity and nonce rejection; overwrite/missing-source cases fail without replacing/creating the source.
- No new dependency, runtime service, secret or environment startup prerequisite. Live Testnet proof, authenticated ledger-meta ingestion, production Postgres, continuous watcher, external backup storage, automated semantic recovery/migrations and release-candidate validation remain unfinished.

## Bounded ledger metadata collection and live read-only Testnet evidence

- Added typed getLedgers transport and `history --rpc-url HTTPS_URL --network-passphrase TEXT --from-ledger N --to-ledger M`. It requests exact consecutive sequences one at a time, validates retention/sequence fields, bounded full LedgerCloseMeta v0/v1/v2 XDR, header-history-entry identity, inner-header SHA-256 and previous-header links. Supported network/header protocols are explicitly 28–29; future/older protocols fail closed.
- Corrected an initial header wire-format assumption against the official RPC implementation and real endpoint response: getLedgers.headerXdr is LedgerHeaderHistoryEntry, unlike getLatestLedger's bare LedgerHeader. Research pins RPC commit dac748d2d793443d9b3595ac50a5e912db8a62a5 and SDK v0.8.0; see docs/research/2026-10-26-ledger-history.md.
- The environment tool reported current, enforced restricted networking with the Testnet host allowed. Native curl getNetwork and the compiled Go collector succeeded through inherited proxy/TLS configuration. An exploratory Python urllib request returned 403; earlier blanket endpoint-denial observations are historical, not the current native Go result.
- Actual read-only Testnet collection: ledger 5403432 hash 062f5c521a7d31677b88ebfbc03e293117c3855a190407ea28b5843cefca35ed; ledger 5403433 hash 274a85ea735736343bc0b77ff4031e289b49e71b9c61ee7d46e1772d5247953a. Both decode as protocol 29, metadata v2, and the second links to the first. Header/digest summary saved under docs/research; full transaction metadata remains outside the repository. No transaction, deployment, asset movement or external alert was performed.
- Capture retains only an accepted prefix on failure. COMPLETE_ENDPOINT_SEQUENCE is an endpoint structural/linkage claim; all captures retain UNVERIFIED source authentication, UNKNOWN history completeness, NOT_VERIFIED metadata commitments, NOT_APPLIED state changes and UNDETERMINED final verdict. Tests demonstrate that altered non-header metadata can still have a matching embedded header; no false authentication guarantee is made. Snapshot replay deliberately rejects these diagnostic envelopes.
- Bounds: 16-ledger range, 4 MiB decoded metadata/ledger, 8 KiB header base64, 20 MiB aggregate retained base64; XDR depth/memory budgets, existing bounded HTTPS/retry transport, two-minute CLI deadline. No current-state-to-history interpolation or database ingestion checkpoint.
- Verification: 122 top-level unit tests PASS; one intentional crash-helper skip (parent executes its subprocess). Full race suite, vet, module verification and CLI build PASS; CLI race checks rerun after the TLS end-to-end test. Metadata fuzz exercise: 2,999 executions across two workers, PASS. Live compiled output validates against schema/history-capture-v1.schema.json and is rejected by replay. TLS tests retain certificate verification and cover complete/partial capture, output failure and exact wire requests; deterministic tests cover mixed protocols, all supported metadata versions, hash/header tampering, gaps/divergence, allocation bombs, retention and cancellation.
- Docs/HISTORY.md describes the limits and next adapter work. No new dependency, service, secret or environment startup requirement. The controlled CAP-85 Testnet upgrade gate, authenticated metadata ingestion, state reconstruction, continuous watcher and release remain unfinished. Remote CI has not been verified.
