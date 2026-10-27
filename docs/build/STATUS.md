# FleetWatch — verified build status

**Repository URL:** https://github.com/soroban-fleetwatch/fleetwatch
**Working branch:** `feat/portable-linux-follow` (from Phase 8 checkpoint `85dd0ec`)
**Date checked:** 2026-10-27
**Release state:** NOT RELEASED
**Mainnet writes:** NONE AUTHORIZED

**Dripwave Phase 9: zero-host browser delivery implemented; public hosting gate unverified.**
The user explicitly rejected a Linux-machine/account prerequisite. A self-contained
Go WebAssembly browser client now performs qualified foreground Testnet observation
and evidence export without those inputs. See [browser instructions](../BROWSER.md),
[zero-cost decision](../ZERO_COST_DEPLOYMENT.md) and [acceptance](../PHASE9_ACCEPTANCE.md).
It is current-state observation while open, not the native historical follower.
Public static-site activation and unattended-service deployment remain unverified;
Phase 9 is not declared complete from a local package or pushed branch.

| Stage | State | Commit/PR/test or evidence | Blocker / next action |
|---|---|---|---|
| S0 Repo audit and M1 repair | IN PROGRESS: baseline verified | Local import; 21 unit tests plus fuzz seeds, race tests, vet, compiled demos pass | Local bounds and explicit unknown coverage implemented; strict chain identifiers and remote CI remain |
| S1 Research/architecture | IN PROGRESS | Official CAP-85 and SDK v0.8.0 source inspected; see docs/research/2026-10-26-cap85-sdk.md | Compile typed compatibility fixtures and finish source/history decisions |
| S2 Typed RPC capture | IN PROGRESS | Typed XDR, bounded HTTPS adapter, CLI capture and offline fixtures | Live proof, freshness/retry/protocol compatibility remain |
| S3 Real Testnet proof | PASS: scoped controlled demonstration | Actual two-member upgrade/rollback/direct repoint; 15 checked receipts; real metadata/restart evidence; local gap/TLS tests | Source/baseline authentication and production operations remain separate gaps; see 2026-10-27-testnet-validation evidence |
| S4 Sequential historical ingest | IN PROGRESS: continuous single-host pilot plus legacy bounded ingestion | Original-assertion continuation, complete cached preimages/absence, atomic raw records/findings/cursor; 40 synthetic and 17 live ledgers | Authentication, lifecycle reconciliation, production archive/retention and shared SQL remain |
| S5 Watcher/alerts | IN PROGRESS: continuous pilot, queue sync, dispatcher and opt-in pinned proxy route | Follow/restart/gap checks; durable sync/startup repair; local signed HTTP/HTTPS proxy delivery and audited recovery | Production proxy permission/receiver validation, gap-notification policy, retention rollover and hosting remain |
| S6 Signed approvals | IN PROGRESS: standalone verifier | Ed25519 statement/trust/threshold tests and public synthetic vector | Signed trust distribution, trustworthy chain evidence and policy integration remain |
| S7 CLI/API integration | IN PROGRESS: experimental CLI | Capture/ingest/watch/replay/outbox/approval commands, bounded JSON worker config, doctor and fleet reports with schemas | Stable configuration, full help/exit compatibility and optional API decision remain |
| S8 Security/performance | IN PROGRESS: parser and dependency gates | RPC guards/fuzz, module/source/binary scans, compression advisory fixed and race/fault tests | Ongoing advisory monitoring, representative load tests and external review remain |
| S9 Docs/repo hygiene | IN PROGRESS: CI and runbooks | Pinned compiler/actions, read-only/time-bounded jobs, workflow lint and security evidence | Remote CI, docs site and full repository/release review remain |
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

1. Extend bounded diagnostic reconstruction checkpoints into authenticated continuous ingestion: trusted baseline, metadata commitments, full lifecycle reconciliation, durable gap/delivery handling, production retention and restart recovery. Never relabel current-state polling or supplied snapshots as comprehensive chain history.
2. Decide independently trusted header anchoring, metadata/transaction commitment verification and provenance requirements before allowing chain-safe verdicts or approval-based severity changes. Source authentication and fleet completeness remain unverified.
3. Controlled CAP-85 Testnet fixtures and scoped update/restart/alert evidence now pass (see the 2026-10-27 Testnet validation checkpoint). Preserve those regressions while addressing source authentication and production operation; do not relabel the diagnostic as authenticated chain approval.
4. Publish/verify the zero-host browser client; native unattended deployment is a separate option. Validate the portable follow pilot only if that mode is selected. Continuous advancement and loopback operational telemetry now work locally; production archive/retention, follow-journal delivery, trust distribution and release validation remain unfinished.

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

## Executable metadata change projection increment

- Continued from pushed checkpoint `d9fd859`. Added offline `changes --capture FILE` and a pure normalizer that projects persistent contract instances and CAP-85 executable-tag entries from captured LedgerCloseMeta v0/v1/v2 and TransactionMeta v0/v1/v2/v3/v4. No database write or RPC call is made by this command.
- Ordering follows SDK v0.8.0 ingest/ledger_change_reader.go and ingest/ledger_transaction.go: ALL fee groups first, each transaction's before/operation/after groups, ALL post-application fee groups, then upgrades. Transaction, operation, upgrade and original vector change indices are retained. Transaction hashes remain unverified metadata claims. The reserved v1 unused vector must be empty.
- STATE preimages remain distinct from CREATED/UPDATED/RESTORED/REMOVED. Intermediate A→B→A writes within one transaction are retained without compaction. Eviction keys are emitted separately as EVICTION_UNORDERED markers, never assigned an invented transaction position or applied as deletion.
- Records retain typed contract/reference values, exact raw tag bytes as hex, re-encoded key/entry XDR, last-modified bounds, ledger header hash and original metadata digest. Versioned IDs bind network, header/metadata evidence, phase/indices and the full record. Modified descriptors/raw evidence and inflated assurance flags are rejected. Invalid executable-related data aborts the entire offending ledger; earlier valid ledgers remain in a PARTIAL report.
- Added testdata/history-changes-capture.json, explicitly synthetic XDR (not a consensus-valid ledger or live Testnet fixture), CLI examples, schema/metadata-changes-v1.schema.json and docs/METADATA_CHANGES.md. The fixture's fee-phase reference preimages only exercise grouping order; they make no claim about actual Stellar fee-side contract mutations.
- Bounds: existing strict input/capture/XDR limits; 10,000 output records across the capture, 100,000 scanned changes per ledger, bounded transaction/operation/upgrade/eviction vectors and 20 MiB compact normalized output; 30-second normalization context checked between work units. All reports retain CHANGED_ENTRIES_ONLY scope, UNKNOWN membership/history completeness, UNVERIFIED source, NOT_VERIFIED commitments, NOT_APPLIED state and UNDETERMINED final verdict.
- `go test -timeout=45s -count=1 -json ./...`: 128 top-level unit tests PASS; one intentional crash-helper skip (parent executes its subprocess). Full race suite, vet, module verification and CLI build PASS; affected history/CLI race checks rerun after final tests. A short normalizer fuzz smoke exercise passed (4 executions; not substantial fuzz coverage). Fresh compiled CLI emits 11 ordered records plus one separate eviction, validates against the new schema, preserves raw tags and intermediate writes, produces unique stable IDs, and is rejected by snapshot replay. The synthetic input also validates against the history capture schema.
- No new live observation, transaction, deployment, secret, dependency, service or environment startup requirement. This is an extraction step toward reconstruction: trusted-baseline application, preimage consistency, membership reconstruction, incident/blast-radius evaluation, metadata commitments, durable chain ingestion and continuous monitoring remain unfinished. Remote CI is unverified.

## Offline asserted-baseline watched reconstruction increment

- Continued from pushed checkpoint `8b71db4`. Added `reconstruct --baseline FILE --capture FILE` and bounded, pure in-memory application to a fixed watchlist. No RPC request, database write, approval binding, external delivery, transaction or deployment performed.
- The operator-asserted baseline requires 1–50 checksum-valid canonical contract IDs, complete currently used references, a nonzero lowercase header hash and the exact preceding ledger/network. The first validated metadata header must link to that supplied hash. The pin is not an independent trust anchor: baseline/source authentication stays UNVERIFIED and metadata commitments stay NOT_VERIFIED.
- Raw capture evidence is revalidated and normalized before application. Executable-field STATE assertions must match the retained projection; UPDATE requires a STATE in the same group. Multiple mutations of a key in one group are rejected rather than inventing internal execution order. Conservative rules stop watched removals, unresolved repoints, conflicting restores and unsupported fee-side executable mutation. Metadata can resolve previously unknown references for later watched use, without authenticating them.
- All mutations in a group are applied before comparison. Added a pure same-ledger operation-state comparison API; ordinary snapshot comparison still enforces strictly increasing ledgers. Actual ledger numbers are retained. Tests show two separate A→B→A findings within one transaction, correct observed before/after membership union through repoints, deterministic evidence-bound group/event IDs and continuous active-projection digests within a ledger.
- Each ledger uses an isolated candidate; conflicts or limits discard the entire offending ledger while retaining previously accepted state/events. Unordered evictions of watched instances or references used at any group boundary stop that ledger. Other eviction markers invalidate cached reference knowledge without inventing deletion/order. The watchlist never expands from unrelated instance records.
- Reports expose the final active watched snapshot, canonical baseline digest/local header pin, per-ledger source hashes/digests, group positions/change IDs and derived findings. All retain UNKNOWN membership/history completeness, UNVERIFIED_PROJECTED_STATE evaluation, EXECUTABLE_FIELDS_ONLY preimage assurance, OBSERVED_MEMBERSHIP_UNION impact and UNDETERMINED final verdict. No policy or approval-based severity change. Snapshot replay rejects these diagnostic reports.
- Added explicitly synthetic reconstruction baseline/capture fixtures (not consensus-valid ledgers or Testnet observations), two JSON Schemas, docs/RECONSTRUCTION.md and updated CLI/docs. Extra bounds: 1,000 baseline reference values, 10,000 cached present/absent reference keys, 10,000 derived events and 20 MiB compact accepted-ledger output; existing capture limits and a 30-second checked context apply.
- `go test -timeout=45s -count=1 -json ./...`: 137 top-level unit tests PASS; one intentional crash-helper skip (parent executes its subprocess). Vet, module verification, CLI build and full race suite PASS; affected history/core/CLI race checks rerun after final tests. Fresh compiled CLI verifies transient findings, event identity, exact final hash/member set, baseline hash/range/network rejection, typed fixture/report schemas and replay rejection. CLI exit 0 for unrelated-reference changes still yields an UNDETERMINED chain verdict.
- No new dependency, secret, service or environment startup prerequisite. This implements a conservative diagnostic subset, not production chain ingestion: independently trusted baselines, full entry/metadata commitments, archive/eviction lifecycle support, within-operation transitions, durable chain cursors/evidence/outbox, live controlled CAP-85 proof and continuous monitoring remain unfinished. Remote CI is unverified.

## Durable diagnostic reconstruction checkpoint increment

- Continued from pushed checkpoint `bef685f`. Added `reconstruct --state FILE --stream ID` and read-only `checkpoint --state FILE --network TEXT --stream ID`. The optional persistence path writes the original asserted baseline, accepted contiguous raw metadata, projected state, groups and findings in one synchronous bbolt transaction under a separate versioned namespace. No RPC call, approval binding, external send, transaction or deployment performed.
- Scopes are exact network plus operator-assigned stream ID, bound to the original canonical baseline digest/header pin. Captures may overlap exactly or extend the next ledger; conflicting metadata/descriptors, changed baselines, unknown gaps and cumulative bounds cannot overwrite accepted evidence. Stream names are local aliases, not authenticated provider identities. Snapshot replay/outbox/approval scopes are unchanged.
- Cold load revalidates raw evidence and recomputes the entire retained report from the original baseline before trusting cached state. This preserves previously observed non-active reference cache/absence knowledge across restart without treating the final active snapshot as a complete baseline. Corrupt reports fail closed. Group/event identity and prior ledger findings remain stable through idempotent replay and extension.
- Source/application failure returns the merged attempt's PARTIAL report while committing only any accepted prefix. The stored report is recomputed for that smaller complete prefix; failed suffixes/attempt diagnostics are not retained. `checkpoint` explicitly labels RETAINED_PREFIX_ONLY, and an incomplete latest source request remains PARTIAL even if cached evidence covers its range. No continuous-source health or freshness guarantee is inferred.
- Tests cover cold resume, exact overlap, immutable baseline, scope isolation, concurrent attempts, raw-evidence/report corruption, partial-prefix commit, gap repair, non-active cache preservation through backup/restore, injected rollback/cancellation and actual subprocess exit before transaction commit. A read-only CLI inspection preserves DB bytes, and output failure after commit does not lose the checkpoint. Backup automatically includes the new namespace.
- Added a synthetic next-ledger capture, checkpoint view JSON Schema, one additive attempt error code, docs/RECONSTRUCTION_CHECKPOINTS.md and CLI/recovery guidance. All output retains UNVERIFIED baseline/source, NOT_VERIFIED commitments, UNKNOWN membership/history completeness and UNDETERMINED final verdict. Findings are stored but deliberately not enqueued in the older snapshot outbox or accepted by its approval-binding command.
- Bounds across each accumulated stream: existing 16-ledger/capture/projection budgets, 25 MiB combined encoded baseline/capture/report and 128 streams per DB. Recompute runs under the 30-second CLI context within the transaction. No pruning, migration, automatic baseline rollover or unbounded backfill. Semantic revalidation protects local consistency, not an adversary able to replace the complete DB and evidence.
- Verification: 146 top-level unit tests PASS; one intentional crash-helper skip (parent executes its subprocess). Full race suite, vet, module verification and build PASS; affected store/CLI race checks rerun after final tests. Fresh compiled processes verify checkpoint 100→101, stable event IDs, idempotence, read-only byte preservation, missing scope, backup restore, partial-source reporting and both reconstruction/checkpoint JSON Schemas. The synthetic extension capture validates against its schema.
- No new dependency, secret, running service or environment startup requirement. Production retention/migrations, durable failed-attempt/gap alerts, reconstruction delivery integration, authenticated metadata/baselines, continuous watcher and controlled live CAP-85 proof remain unfinished. Remote CI has not been verified.

## Reconstruction alert and durable attempt integration increment

- Continued from pushed checkpoint `445b6b5`. Reconstruction now saves accepted raw evidence/state/findings, a bounded latest-attempt marker and finding/diagnostic alerts in one bbolt transaction. Failure-only attempts create a diagnostic and bind the stream to its asserted baseline before any checkpoint exists. Conflicting accepted overlap preserves the prefix and records EVIDENCE_CONFLICT; malformed inputs, invalid baseline assignments and cumulative bounds are refused.
- Added inner alert payload v2 for rpc_ledger_meta streams, with separate reconstruction_event/diagnostic shapes and mandatory UNVERIFIED baseline/source, NOT_VERIFIED commitments, UNKNOWN membership/history completeness and UNDETERMINED final verdict. Snapshot payload v1 and outer webhook/MAC v1 framing are preserved. Stream-scoped alert IDs bind full event evidence; diagnostic IDs bind exact typed attempt summaries. Fixed codes/input digests replace arbitrary failed-body/upstream text retention.
- Reuse of fenced leases, retries/dead letters and HMAC sender now supports `alerts`/`deliver --source rpc_ledger_meta --stream ID`. Listing is read only; claims check queued findings against the rederived report and require the latest partial diagnostic. Payload/identity tampering or missing records prevents dispatch. Claims inherit parent cancellation and account for writer/revalidation wait before granting a lease; legacy queue implementations remain supported by the dispatcher.
- Successful repair updates checkpoint.last_attempt but preserves historical diagnostics and acknowledgment status. Exact replay/cold resume does not requeue delivered findings. Old pre-integration checkpoints backfill findings on their next explicit reconstruction call. Legacy snapshot approval/outbox scopes stay separate; no signed-policy downgrade or chain authentication is inferred from HMAC delivery.
- Capacity remains 10,000 alert rows per stream and 1 MiB/record; enqueue failure rolls back checkpoint/attempt updates. Attempt summaries are bounded to 4 KiB, and accepted/failure-only scopes share the 128-stream limit. No pruning, automatic historical resolution, full failed-body retention or operator dead-letter recovery. Current diagnostics describe offline invocations, not source-freshness probes; old diagnostics may be delivered after repair.
- Tests cover atomic rollback and queue-full refusal without cursor/attempt advancement, actual pre-commit process exit, failure-only baseline binding, gap repair/historical retention, corrupted assurance/finding/missing-alert rejection, migration backfill, idempotent acknowledged resume and delayed-writer lease validity. Local TLS end-to-end delivery verifies exact-body HMAC, stream/assurance labels, idempotency keys, delivery acknowledgments and pre-network rejection of forged APPROVED assurance. Certificates are verified against generated test roots; no live receiver contacted.
- Verification: 154 top-level unit tests PASS; one intentional crash-helper skip (parent runs its subprocess). Full race suite, vet, module verification and CLI build PASS; affected store/alert/CLI race checks rerun after context/lease tests. Fresh compiled CLI verifies finding payloads, durable partial diagnostics, latest-attempt repair, historical rows, read-only byte preservation, backup restore and alert/attempt/checkpoint JSON Schemas. The delayed-writer lease check passed again with a two-second lease after a longer writer wait.
- Added docs/RECONSTRUCTION_ALERTS.md, two wire/attempt schemas, additive checkpoint last_attempt data/error codes and updated receiver/checkpoint guidance. No new dependency, secret, runtime service, external message, transaction, deployment or environment startup prerequisite. Production retention, continuous worker, proxy-backed delivery, operator recovery, independently authenticated chain/baseline evidence and controlled live CAP-85 upgrade proof remain unfinished. Remote CI is unverified.


## Manual dead-letter recovery increment

- Continues published checkpoint `cafe3b6` on branch `work`. Added explicit `requeue` and read-only `recoveries` commands for snapshot queues and reconstruction streams. No automatic sends, retargeting or bulk recovery.
- Each recovery atomically stores an append-only local receipt and returns the selected alert to PENDING. Payload, stable ID, assurance labels and bound sink remain intact; current-cycle attempts/backoff/lease clear. The receipt retains prior attempts/status/error/sink, payload SHA-256, expired lease deadline where applicable, supplied reason and UTC time. It is a local consistency record, not authenticated operator identity.
- Only DEAD or fifth-attempt INFLIGHT records with expired leases qualify. Exact network/source/stream and expected recovery cycle fence stale requests. Request IDs are queue-scoped; exact repetitions return the original receipt even after delivery, while changed parameters conflict. Old worker tokens remain fenced. Evidence is revalidated before recovery; missing/inconsistent selected-alert audit history blocks recovery.
- Bounded to ten recoveries per alert (five attempts per cycle), 10,000 receipts per queue, 4 KiB per receipt and a 512-byte noncontrol UTF-8 reason. Existing records default to cycle zero. Older binaries may reject recovered records; no automatic downgrade/migration or pruning is provided. Restoring an older backup still requires external reconciliation of later sends and recovery actions.
- Added tests for all four payload variants (snapshot finding/gap, reconstruction finding/diagnostic), historical diagnostics after source repair, payload/evidence corruption, repeated requests, concurrent cycle fencing, active/exhausted leases, sink binding, stale acknowledgments, retry-cycle/quota limits, cancellation, rollback, process crash before commit, backup restore, missing audit and CLI output failure followed by an idempotent receipt retry.
- `go test -timeout=45s -count=1 -json ./...`: 166 top-level unit tests PASS, four fuzz targets' seed runs PASS; one intentional crash-helper skip (parent executes its subprocess). Full `go test -race -timeout=60s -count=1 ./...`, `go vet ./...`, `go mod verify` and compiled CLI build PASS. Two final payload-variant tests then passed in the full regular suite and targeted recovery race suite. `git diff --check` PASS.
- Fresh compiled CLI smoke checks PASS: recovery, identical repeat, read-only audit, unchanged alert ID with PENDING/zero attempts/cycle one, and stale-cycle rejection. Recovery result validates against the new Draft 2020-12 schema with date-time checking. CLI tests cover missing DB without creation and failed output after a committed action. No external webhook, transaction, deployment or message was sent.
- Added docs/ALERT_RECOVERY.md and schema/alert-recovery-v1.schema.json; updated outbox, webhook, reconstruction and README guidance. No new dependencies, services, secrets or startup requirements. Continuous ingestion, authenticated chain/baseline/metadata evidence, proxy-backed production delivery, retention, controlled CAP-85 live proof and release validation remain unfinished. Remote CI is unverified.


## Reconstruction health inspection increment

- Continues published checkpoint `7f15d1c` on branch `work`. Added `doctor --source rpc_ledger_meta --stream ID` with a separate reconstruction-health v1 schema. Existing synthetic/user_snapshot health JSON and exit behavior remain compatible.
- One read transaction rederives the retained raw-evidence projection and checks checkpoint/latest-attempt/outbox agreement. The shared queue inspector now exposes its already-validated checkpoint and attempt internally, avoiding a second reconstruction pass. Shared delivery counters retain the existing lease/backoff/exhaustion boundaries.
- Report includes cursor, retained ledger/event counts, historical diagnostic count, latest bounded attempt summary, outbox counters and aggregate recovery receipt/recovered-alert counts. A failed first attempt has no checkpoint; a failed extension retains the accepted prefix but requires attention. COMPLETE latest input and valid local queues can report OK_OFFLINE even with security findings or pending delivery. Historical dead diagnostic rows still require delivery attention after source repair.
- Whole-queue audit validation reads at most 10,000 bounded receipts once, then checks each alert's recovery sequence, cycle count, payload digest and sink. Missing/orphaned/inconsistent receipts fail inspection with no partial success report. Shared recovery receipt decoding now uses the strict JSON reader, rejecting duplicate/case-aliased members as well as unknown/trailing input. It remains local consistency checking, not tamper-proof provenance or operator authentication.
- Exit 4 for absent checkpoints or incomplete/unknown latest attempts takes precedence over delivery-attention exit 2. Legacy pre-integration checkpoints explicitly report ALERTS_NOT_INTEGRATED/LATEST_ATTEMPT_UNKNOWN instead of silently claiming ready alerts. Read-only inspection never backfills, claims, acknowledges, sends or writes attempt markers. CLI deadline is 30 seconds plus the existing one-second file-lock timeout.
- Every report retains RETAINED_PREFIX_ONLY coverage, UNVERIFIED baseline/source, NOT_VERIFIED metadata commitments, UNKNOWN membership/history/source freshness, UNDETERMINED verdict and null ledger lag. Reports omit individual alert payloads, bound sinks, lease tokens, request IDs and operator reasons. No source endpoint is contacted by this command.
- Added six store tests and two CLI tests covering unchanged DB bytes, scope isolation, complete/partial/absent states, source repair with historical dead-letter recovery, fifth-attempt lease boundaries, old checkpoints, missing findings/attempts, corrupt evidence, missing/orphaned/duplicate/mismatched audit, clock/scope/cancellation errors, CLI status precedence and failed output.
- `go test -timeout=45s -count=1 -json ./...`: 174 top-level unit tests PASS; four fuzz targets' seed runs PASS; one intentional crash-helper skip (parent executes its subprocess). Full `go test -race -timeout=60s -count=1 ./...`, `go vet ./...`, `go mod verify`, CLI build and `git diff --check` PASS.
- Fresh compiled CLI smoke checks PASS for complete, absent and failed-extension streams, unchanged database hashes during inspection, new reconstruction-health schema/date-time validation and unchanged snapshot-health schema validation. Added schema/reconstruction-health-v1.schema.json and updated README/health/reconstruction/recovery runbooks.
- No new dependencies, services, secrets, external receiver messages, transactions, deployments or startup requirements. Continuous worker/configuration, live telemetry, authenticated baseline/chain/metadata evidence, proxy-aware production delivery, retention, controlled live CAP-85 proof and release validation remain unfinished. Remote CI is unverified.


## Bounded resumable RPC ingestion increment

- Continues published checkpoint `5df366e` on branch `work`. Added explicit `ingest --rpc-url HTTPS_URL --baseline FILE --state FILE --stream ID --to-ledger N`, connecting the existing exact-range metadata collector to durable watched reconstruction. It fetches from retained cursor + 1, preserves the original baseline binding and commits accepted evidence, projected findings, latest attempt and outbox atomically. It does not send alerts or perform on-chain writes.
- Added a read-only reconstruction planner and cursor-checked commit entry point. Planning revalidates retained evidence and queue/attempt agreement; baseline snapshot digest, header pin and ledger must match both accepted and failure-only scopes. The write transaction checks the expected cursor again after fetching, refusing a stale batch without overwriting an intervening checkpoint/attempt. Offline reconstruct overlap/replay semantics remain unchanged.
- Source/application failures can commit a validated prefix plus diagnostic and return PARTIAL/exit 4. Cold resume fetches only the rejected/missing suffix. Cancellation before commit preserves the earlier state; SIGINT/SIGTERM cancel CLI work. Overall operation deadline is two minutes, with 30-second planning and commit/revalidation deadlines and the existing RPC request/retry bounds. Context failures propagate through validation as cancellation rather than fabricated success.
- Targets remain after the original baseline and at most 16 ledgers beyond it; existing evidence/storage/outbox/scope limits remain. This is a finite diagnostic ingestion command, not an unbounded worker. An already-retained target performs no RPC query or DB write, reports ALREADY_RETAINED/NOT_QUERIED and preserves the observed latest-attempt status, including PARTIAL for an attempted extension beyond that target. It does not clear old diagnostics or backfill old queues merely by inspection.
- Results retain UNVERIFIED baseline/source, NOT_VERIFIED metadata commitment, UNKNOWN membership/history completeness and UNDETERMINED final verdict. COMPLETE endpoint capture can still yield PARTIAL application if the asserted baseline link or state projection fails. Fetch errors use fixed codes without raw endpoint/error/credential text. The baseline remains operator asserted, and no live source freshness or authenticated chain-safety claim is introduced.
- Added three store, six ingestion and two CLI tests covering planned ranges, baseline/range/scope binding, atomic pre-commit rollback, stale cursor/range fencing, cold reopen/resume, outbox deduplication, already-retained no-op with unchanged DB, partial fetch/repair, source-query failure diagnostics, concurrent advance during fetch, cancellation, baseline-header link failure, CLI output failure after commit, invalid arguments without DB creation and health integration.
- `go test -timeout=45s -count=1 -json ./...`: 185 top-level unit tests PASS; four fuzz targets' seed runs PASS; one intentional crash-helper skip (parent executes subprocess). Full `go test -race -timeout=60s -count=1 ./...`, `go vet ./...`, `go mod verify`, compiled CLI build and `git diff --check` PASS.
- Fresh compiled CLI against a local HTTPS JSON-RPC fixture with certificate verification and a combined trusted CA file PASS: ledger 101 unavailable after accepted ledger 100; partial checkpoint/diagnostic persists; repaired endpoint resumes exactly at 101; three findings and one historical diagnostic remain; doctor returns OK_OFFLINE; repeated target causes no RPC call and leaves DB hash unchanged. Partial/complete/no-op results validate against the new standalone schema. No live Testnet endpoint or receiver was contacted in this increment.
- Added docs/INGESTION.md and schema/ingestion-result-v1.schema.json; updated history, checkpoint and README guidance. Existing RPC SDK wire fields inspected in pinned v0.8.0 source: https://github.com/stellar/go-stellar-sdk/blob/v0.8.0/protocols/rpc/get_ledgers.go . No new dependencies, services, secrets, deployments or environment startup requirements.
- Continuous watcher/configuration, production retention/telemetry, authenticated baseline/metadata/consensus evidence, proxy-aware production delivery, controlled live CAP-85 upgrade proof and release validation remain unfinished. Remote CI is unverified.


## Bounded polling worker and configuration increment

- Continues published checkpoint `412b64c` on branch `work`. The cloud executor disconnected after initial implementation, then reconnected; unfinished files were inspected and preserved before testing resumed. Added `watch --config FILE` to poll toward one fixed target, retaining existing baseline/cursor fencing, raw evidence, latest-attempt and atomic finding/diagnostic outbox behavior. Webhook delivery remains explicit and separate.
- Added strict nonsecret watch JSON configuration: required version/endpoint/baseline/state/stream/target and polling/attempt/runtime budgets. Config is bounded to 16 KiB, rejects duplicate/case-aliased/unknown/trailing input and invalid/overflowing bounds, resolves paths relative to its own directory and refuses state/config/baseline path collisions. CLI validates baseline/scope and the original 16-ledger interval bound before opening a DB. No defaults, environment expansion or embedded signing/delivery configuration.
- Polling intervals 1–60 seconds, attempts 1–120 and overall context budget 1–3,600 seconds. Each attempt retains ingestion's existing timeout/validation bounds, capped by the remaining worker deadline. Context-aware waits and SIGINT/SIGTERM cancellation stop work without discarding prior committed progress. The worker holds the existing exclusive bbolt lock throughout its run; maintenance commands require it to stop first.
- Retries are restricted to bounded source availability/missing-range codes and require all decoded records to have been accepted. Wrong networks, unsupported/invalid evidence, rejected projected prefixes and state failures stop. A concurrent cursor advance replans on the next budgeted attempt. Hard per-attempt timeouts stop instead of being retried. The original asserted baseline and 16-ledger interval are never automatically rotated or expanded.
- NDJSON progress includes per-process sequence, attempt/partial/conflict/failure counters, returned decoded-record counts, accepted cursor advances, last observed cursor/fixed-target remaining count, observed finding count and fixed source/error codes. Unknown cursor/finding/lag values remain null. Counters reset at restart and omit unreported fetches on failed commits; they are not persistent metrics, RPC call counts or live source lag. Fixed projection-assurance values now share a constructor with ingestion/storage.
- Already-retained targets query no source and write no checkpoint; fresh process finding counts stay unknown and a prior PARTIAL latest attempt is preserved. Output failure stops retries immediately, even when a checkpoint already committed. Progress omits endpoint/path/raw payload/receiver/token/operator-reason fields, and retains UNVERIFIED baseline/source, NOT_VERIFIED metadata commitment, UNKNOWN membership/history/source freshness and UNDETERMINED verdict.
- Added eight monitor unit tests, two integration tests, two configuration tests and three CLI tests covering retry-to-completion and counters, permanent rejection, accepted-prefix requirements, cursor conflicts, attempt limits, cancellation before calls/during waits, deadlines, output failure, no-op semantics, invalid options/result scopes, cold reopen/resume, outbox deduplication, strict/path/bounds config, CLI output failure after commit and invalid config without DB creation.
- `go test -timeout=45s -count=1 -json ./...`: 200 top-level unit tests PASS; four fuzz targets' seed runs PASS; one intentional crash-helper skip (parent executes subprocess). Full `go test -race -timeout=60s -count=1 ./...`, `go vet ./...`, `go mod verify`, compiled CLI build and `git diff --check` PASS.
- Fresh compiled CLI over a local HTTPS JSON-RPC fixture with certificate verification/combined test CA PASS: partial ledger-100 prefix, retry/resume at ledger 101, three findings plus one historical diagnostic, already-retained no-op with no requests and unchanged DB hash, relative paths from a different cwd, attempt/deadline exhaustion and wrong-network stop. SIGTERM during a wait exits 4 with CANCELLED; SIGKILL after a committed RETRYING record leaves a readable partial checkpoint; both restart from the accepted cursor without duplicate outbox records. Config/example and every emitted progress record validate against the new schemas. No live Stellar endpoint or webhook receiver contacted.
- Added docs/WATCH.md, examples/watch.synthetic.json (reserved placeholder endpoint), watch-config-v1 and watch-progress-v1 schemas; updated README/ingestion guidance. No new dependencies, secrets, services, deployments or startup prerequisites. This remains a finite diagnostic worker: production retention, unbounded service operation, durable telemetry, authenticated baseline/chain/metadata evidence, proxy-aware production delivery, controlled live CAP-85 proof and release validation remain unfinished. Remote CI is unverified.


## RPC response ambiguity/resource hardening increment

- Continues published checkpoint `4fce115` on branch `work`. Replaced permissive RPC response unmarshalling with bounded whole-document structural validation shared with operator JSON inputs, while retaining a separate extensible mode for additive provider fields. Operator files still reject unknown fields; only provider responses use extensible decoding.
- All four RPC methods now reject duplicate/case-aliased member pairs in nested results and ignored extensions, invalid UTF-8 bytes, excessive nesting, trailing values, invalid envelope names/version/ID, nonobject results and nonnull RPC errors. Canonical envelope member names are required; `error:null` and nonconflicting provider extensions remain compatible. Failed public RPC methods return zero values instead of partially decoded structs. Fixed errors do not expose raw upstream/read/type-error details.
- The shared JSON reader now bounds each object's duplicate-key set to 10,000 members, with exact-boundary/nested-overflow tests. Existing 25 MiB limit-plus-one reads and nesting limit remain. Response bodies flow directly into this bounded reader without a second complete body read. Result semantics, XDR validation, network/header checks, source trust labels and chain verdicts remain separate and unchanged.
- Inspected Go 1.27.3 bundled encoding/json Unmarshal documentation describing duplicate replacement/merging, case-insensitive field matching, partial type-error decoding and invalid UTF-8 replacement. Reference: https://go.dev/src/encoding/json/decode.go . Added explicit regressions instead of relying on the standard decoder's permissive behavior.
- Added two shared-reader tests and four RPC tests covering extensible/strict separation, duplicate/case aliases in envelope/results/extensions, canonical reserved fields, malformed IDs/types/nulls, invalid bytes, deep/oversized responses, reader-error redaction, zero-valued failures across all RPC methods and accepted provider extensions. New FuzzRPCJSONResponse seeds exercise protocol/identity invariants; a metadata-sized synthetic response benchmark measures only JSON parsing.
- `go test -timeout=45s -count=1 -json ./...`: 206 top-level unit tests PASS; five fuzz targets' seed runs PASS; one intentional crash-helper skip (parent executes subprocess). Full `go test -race -timeout=60s -count=1 ./...`, `go vet ./...`, `go mod verify`, CLI build and `git diff --check` PASS. A 30-second RPC parser fuzz exercise with two workers completed 171,088 executions without failure.
- BenchmarkRPCJSONMetadataSizedResponse with GOMAXPROCS=2, three iterations, Go 1.27.3 linux/amd64 / AMD EPYC 9V74 measured 46,329,675 ns/op, 45.27 MB/s, 25,697,389 B/op and 131 allocs/op. Payload is a network result plus a synthetic 2 MiB string extension; other checks shared the instance. Allocated bytes/op are not peak memory, and this is not a production ingestion benchmark or SLA.
- Fresh compiled CLI/local verified-HTTPS smoke PASS: ambiguous duplicate envelope yields a failed query diagnostic with no checkpoint; duplicate ledger-101 field retains only accepted ledger 100; corrected responses resume from 101 with three findings/one historical diagnostic; nonconflicting envelope/result/ledger provider extensions remain accepted. Result schemas and doctor consistency checks pass. No live Stellar endpoint, receiver, transaction or deployment contacted.
- Added docs/RPC_JSON.md, refreshed the outdated threat model to describe current source/store/worker/approval/delivery boundaries, and updated README/development guidance. No new dependencies, secrets, services or startup requirements. Unbounded retention/service operation, authenticated baseline/chain/metadata commitments, controlled live CAP-85 proof, dependency scanning, representative load tests, proxy-aware production delivery and release validation remain unfinished. This internal hardening/fuzzing is not an independent audit. Remote CI is unverified.


## Dependency advisory remediation and reproducible CI increment — 2026-10-27

- Continues published checkpoint `981b9e5` on branch `work`. Installed and verified govulncheck v1.9.0 as an isolated development tool; it is not added to the runtime module graph. Initial official vulnerability-DB access returned Forbidden with no verdict. Saved a narrow environment draft adding vuln.go.dev, preserving existing domains/presets. Later direct readiness checks and actual source/binary scans succeeded through inherited proxy/CA routing; no verification/routing bypass was used.
- Initial source/test and binary symbol scans reported zero reachable vulnerabilities, zero affected imported packages and one affected required module. A module-level scan exited 3 for GO-2026-5841, an out-of-bounds read in github.com/klauspost/compress/s2. FleetWatch imports the module's zstd package through Stellar SDK XDR, not s2; this is not evidence of a reachable exploit. Updated the indirect compress requirement from v1.17.6 to the advisory's fixed v1.18.7 and regenerated its Go checksums. Other runtime pins, assurance labels and source/application semantics remain unchanged.
- Added scripts/check-vulnerabilities.sh: checksum verification, pinned temporary govulncheck installation/version, CLI dependency-module gate, full source/test symbol gate, fresh trimmed-path binary gate and cleanup. Text-mode findings/errors propagate nonzero exits; JSON/SARIF/OpenVEX are not used as exit-status gates. Module mode runs from cmd/fleetwatch because root has no Go files and module-only mode accepts no package patterns. The complete script passed after remediation: all three scan levels report no vulnerabilities found against DB update 2026-10-21 00:27:29 UTC.
- Added toolchain go1.27.3 while retaining go 1.25.0 as the module language floor. Updated CI to verified official actions/checkout v7.0.1 SHA 3d3c42e5aac5ba805825da76410c181273ba90b1 and actions/setup-go v7.0.0 SHA b7ad1dad31e06c5925ef5d2fc7ad053ef454303e. Their manifests use Node 24; setup-go documents runner >=v2.327.1 and go.mod toolchain selection. CI uses ubuntu-latest and the pinned compiler, with local toolchain selection, read-only module flags, no persisted checkout credentials, contents:read permissions, explicit timeouts and superseded-run cancellation.
- Go CI now separates test and vulnerabilities jobs. The test job includes module verification/tidy-diff, formatting, unit, vet, race and CLI build; the security job executes the full fail-closed scanner script. No release/tag, merge, deployment, SARIF upload or external message step was added. Remote GitHub Actions execution remains unverified.
- Post-upgrade checks under CI-equivalent GOFLAGS=-mod=readonly, GOTOOLCHAIN=local and GOTELEMETRY=off PASS: go mod verify; go mod tidy -diff; go test -timeout=60s -count=1 -json ./... (206 top-level unit tests plus five fuzz targets' seed runs, one intentional crash-helper skip executed by its parent); go vet ./...; go test -race -timeout=90s -count=1 ./...; trimpath CLI build, compiled version and snapshot validation. Bash syntax, actionlint v1.7.12, formatting and git diff --check PASS. No bespoke tests were added for the manifest-only dependency fix; existing XDR/source/store/worker/approval/delivery regression and fault suites ran in full.
- Added docs/DEPENDENCY_CHECKS.md and docs/research/2026-10-27-dependency-checks.json with advisory, versions, scan outcomes, DB timestamp, official sources and limitations; updated README/development/threat/parser guidance. Results are point-in-time linux/amd64 known-advisory checks, not an independent audit, cross-platform verdict or chain-safety claim.
- Saved refreshed complete start_skill instructions for existing checkout/tool activation, current checks, optional workflow lint, pinned security script/network prerequisites and current source/storage/delivery trust limits. Existing install_script remains applicable: the repository script installs its own temporary scanner, and the existing verified Go 1.27.3 bootstrap remains unchanged. Draft network/start instructions are saved for user review/publication, not applied/published by the agent; fresh-task restoration is unverified. No new service or secret requirement.
- Authenticated baseline/chain/metadata commitments, production retention/unbounded operation, durable telemetry, controlled live CAP-85 upgrade proof, proxy-aware production delivery, representative load/external review and release validation remain unfinished. No live Stellar endpoint, webhook receiver or transaction was contacted in this increment.

Sources: https://pkg.go.dev/vuln/GO-2026-5841 ; https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck ; https://vuln.go.dev ; official action release refs/manifests linked in docs/DEPENDENCY_CHECKS.md.


## PR #1 compiler selection correction

- User-reported GitHub Actions logs show govulncheck v1.9.0 installation failing under Go 1.25.0 with GOTOOLCHAIN=local. This contradicts the earlier assumption that setup-go would prefer go.mod's toolchain recommendation over its language directive. Remote CI was failing; local passes did not establish remote compiler selection.
- Added .go-version with the explicit 1.27.3 compiler version and made both jobs read that dedicated file. Both jobs now print and assert the selected GOVERSION before checks. Retained the Go 1.25 language floor, Go 1.27.3 toolchain recommendation, local-only toolchain policy and pinned vulnerability scanner; no security gates were disabled or downgraded.
- Confirmed PR https://github.com/soroban-fleetwatch/fleetwatch/pull/1 has work as its source: refs/pull/1/head and refs/heads/work both matched published checkpoint 542d57d before this fix. Pushing the correction to work updates the existing PR.
- Local checks PASS with GOFLAGS=-mod=readonly, GOTOOLCHAIN=local and Go 1.27.3: exact compiler assertion, actionlint, module tidy-diff, full unit/fuzz seed suite, vet, full race suite, trimmed-path build and diff checks. scripts/check-vulnerabilities.sh PASS: module, source/test and fresh binary scans each report no vulnerabilities found, scanner v1.9.0 / DB updated 2026-10-21 00:27:29 UTC.
- Updated dependency/development guidance to state the explicit version-file selection. Remote checks must rerun on the new PR head; no remote success is claimed from these local results.

## Phase 8 CAP-85 fixture and Testnet preflight checkpoint — 2026-10-27

- User confirmed PR #1 was merged and authorized starting Testnet validation.
  Fetched origin/main at `0014aaa3943f967912cc08bc8839548804f32557`
  (Merge pull request #1); created `feat/testnet-cap85-demo` from that exact main.
  No existing changes were present or overwritten.
- Added a separate test-only Rust workspace with pinned SDK 29.0.1, Rust 1.99.1
  and Cargo.lock. Inspected published stable SDK/CAP-85 APIs before implementation.
  Owner publication/factory deployment and member direct repoint require the
  stored administrator's authorization. No asset functions or production watcher
  signing capability were introduced.
- Built actual member A/B and owner Wasm through Stellar CLI 28.1.0, including its
  required spec processing. Three host tests PASS: shared A→B→A behavior for two
  members and direct detachment; unauthorized publish/deploy rejection; unauthorized
  member repoint rejection. Repeated build/checks reproduce identical local Wasm
  digests recorded in docs/research/2026-10-27-testnet-fixtures.md. These are local
  host executions, not live Testnet contract deployments.
- Added read-only scripts/testnet-preflight using existing strict RPC and metadata
  readers. Real Testnet passphrase/protocol 29, fresh head and one historical
  metadata record PASS. Saved sanitized sequence/header/metadata digests in
  docs/research/2026-10-27-testnet-preflight.json; metadata version 2, zero
  transactions sent, source UNVERIFIED, commitments NOT_VERIFIED and final verdict
  UNDETERMINED. No independently authenticated baseline or chain claim.
- Added scripts/demo-testnet.sh: default/check builds and tests fixtures then
  performs the read-only preflight. Explicit run mode requires a test-only alias,
  signing config outside the repo and a new output directory, and fixes destination
  and passphrase to Testnet. Prepared live snapshot smoke checks exact uploaded
  Wasm hashes, both external-reference members and the expected affected set.
  It preserves partial/unverified capture envelopes and public transaction logs.
  The live transaction path has NOT RUN successfully and is not an acceptance PASS.
- Added an offline fixture CI job with pinned/checksummed CLI, pinned Rust,
  locked dependencies, formatting and actual-Wasm tests. CI never funds, signs,
  deploys or contacts Testnet. Go formatting also covers scripts. Workflow lint
  passes locally; remote checks on this branch remain unverified.
- Full Go unit/fuzz-seed suite, vet, race suite, trimmed-path CLI build, module
  tidy-diff, Bash syntax, Rust formatting, actionlint and diff checks PASS. Pinned
  vulnerability script PASS: module/source-test/binary scans each report no
  vulnerabilities found. This does not cover Rust advisories or constitute an audit.
- Go funding request to the RPC-advertised friendbot.stellar.org failed with proxy
  Forbidden. A disposable identity generated for diagnosis was never funded or
  used to sign a transaction, its secret was never printed/tracked, and its file
  was removed. Saved a network draft adding only friendbot.stellar.org while
  retaining soroban-testnet.stellar.org, storage.googleapis.com, vuln.go.dev and
  existing presets. Save confirmed; publication/runtime effect not established.
- Full S3 remains BLOCKED on live deployment/update evidence, independent
  transaction checks, history reconstruction, deliberate gap, restart and alert
  acceptance. Baseline/metadata authentication, unbounded retention/operation,
  production delivery and release/Wave readiness also remain unfinished.
- Follow-up transport diagnosis: CLI RPC uses jsonrpsee's direct Hyper connector,
  which does not honor this environment's HTTP proxy. Stopped the exploratory
  reqwest-native-roots build because it cannot fix that path. Added a loopback-only
  fixed-Testnet RPC adapter that retains managed proxy routing, TLS verification,
  strict request/method bounds and no cross-host redirects. Official CLI health
  through the adapter PASS at ledger 5407609; no transaction sent. The live runner
  starts/stops its own adapter. Funding instructions use the proxy/CA-aware curl
  client with a public account address. No destination denial was bypassed.
- Follow-up checks PASS: 207 top-level Go tests and five fuzz targets' seed runs,
  one intentional crash-helper skip; bridge race test, scripts vet, actionlint,
  Bash syntax and diff checks. Full application race suite passed earlier in this
  checkpoint; only developer transport/scripts changed afterward. All three
  vulnerability scan levels rerun and PASS. Demo guards reject unknown mode,
  secret-shaped aliases and repository-local signing configs before network work.
  Test bridge process stopped; no service left running. Complete updated startup
  instructions saved in the same network draft; save confirmed, requires_publish
  true. Public checkpoint `3eae690` is on the new branch; transport follow-up is
  recorded in the subsequent commit. No live deployment or public release.

## Dripwave Phase 8 scoped Testnet validation PASS — 2026-10-27

- Continues published `dd0784a` on feat/testnet-cap85-demo. Current environment
  policy is enforced and includes friendbot.stellar.org. Faucet funding succeeded;
  the source-access blocker is resolved. The official CLI's default cache write
  failed on the read-only filesystem; supported --no-cache fixes it without
  disabling TLS, signatures, RPC validation or managed proxy routing.
- Actual owner/two-member fixtures deployed. Shared A→B at ledger 5407730 affected
  both; B→A rollback at 5407793 affected both; direct repoint of the first at
  5407796 detached it; shared A→B at 5407797 affected only the second. Submitted
  version calls' successful result metadata showed 1/1 then 1/2. These were actual
  Testnet transactions, not simulation results. Fifteen fixture receipts checked
  through a separate Go read path: network-bound envelope hashes, successful XDR
  results and decoded metadata. Same upstream endpoint, not independent consensus.
- Added typed read-only HTTPRPC.Transaction using existing bounded/strict RPC
  decoding, canonical hash validation and zero-valued failure behavior. Regression
  test covers invalid hash rejection before network access and ambiguous receipts.
  Developer testnet-receipt tool rejects unsuccessful/mismatched envelope results;
  it explicitly retains inclusion NOT_VERIFIED and final UNDETERMINED.
- Actual initial upgrade reconstructed from 5407729 asserted baseline and 5407730
  metadata. Cold-process ingest returned ALREADY_RETAINED without DB byte changes.
  Another real twelve-ledger range, 5407792–5407803, reconstructed three controlled
  changes. A cancelled future-target worker preserved an incomplete attempt and
  restarted to COMPLETE; repeated completed watch preserved the exact outbox.
- Additional read-only SIGTERM test proves accepted-prefix persistence: baseline
  5407826, one ledger accepted, cancellation exit 4 retained 5407827, fresh worker
  resumed through 5407830 with exit 0. This is separate from the first cancellation,
  which occurred before any accepted ledger and retained only its diagnostic.
- Opt-in local fault replay omitted actual closed ledger 5407793 from captured
  metadata, stopping at 5407792 with MISSING_OR_UNEXPECTED_LEDGER. Cold reopen and
  complete replay repaired through 5407803; repeat was ALREADY_RETAINED with stable
  outbox IDs. This was injected locally, not a real missing Testnet ledger.
- Opt-in local verified-TLS receiver delivered a backup of the real outbox: three
  findings plus 23 historical diagnostics, exact-body HMAC/IDs/assurance verified,
  zero repeated attempts after reopening. External deliveries zero. Production
  webhook URL/DNS/proxy controls were not weakened. Both opt-in tests PASS when
  explicitly run against captured real data; normal CI skips them.
- Added a repeatable continuation driver scripts/validate-testnet.py for receipt,
  reconstruction, rollback/repoint/version, bounded watcher, gap-replay and local
  receiver checks. It requires an explicit dedicated alias/config outside the repo
  and fresh output directory. Assertion-disabled Python is rejected. The initial
  full run and separate committed-prefix/gap checks passed; later guard/refactoring
  changes were checked locally without repeating unnecessary Testnet writes.
- Evidence: docs/research/2026-10-27-testnet-validation.md and matching JSON, with
  actual contract IDs, transaction hashes, ledger bounds and assurance limitations.
  Raw captures/DBs/binaries remain ignored under testnet-runs. Disposable signing
  identity removed after the controlled writes, never printed/tracked. No Mainnet
  write, public release/tag, hosted production service or Wave
  submission. Testnet faucet credits were used for fixture transaction fees only.
- Checks PASS: 208 top-level default Go tests, five fuzz targets' seed runs, full
  race suite, vet, module verify/tidy-diff, module/source-test/binary vulnerability
  gates, actionlint, Bash/Python syntax and diff checks. Three default skips are
  the intentional crash helper (parent runs it) and the two opt-in evidence tests;
  their separate real-data invocations passed. Affected tests rerun after final
  cleanup/assertion guards. Remote CI on this new checkpoint remains unverified.
- S3 / Dripwave Phase 8 scoped demo is PASS; this does not complete internal S8
  security/performance or the overall roadmap. Next Dripwave phase is operations
  and hosting topology. Baseline/source UNVERIFIED, metadata commitments and
  inclusion NOT_VERIFIED, membership/history UNKNOWN, final UNDETERMINED remain.
  Authentication, continuous retention, production/proxy-aware delivery, load and
  external review, release validation, docs site and Wave readiness remain open.

## Phase 9 portable Linux follow pilot — 2026-10-27

- User chose portable Linux preparation, without supplying a production host or
  hosting account. Continues `85dd0ec` on new branch feat/portable-linux-follow.
  No paid service, user/account permission, remote host, public release/tag or
  repository merge was created. Phase 9 production-hosting gate stays IN PROGRESS.
- Added experimental follow mode with no fixed target/process lifetime. Source
  batches remain at most sixteen ledgers. The original assertion/header provenance
  and complete active/inactive/absent reducer state persist across batches; there
  is no automatic baseline refresh, cursor skip or retention-bound bypass. The
  legacy bounded watch/reconstruct commands keep their existing lifetime limits.
- Separate v1 follow bbolt store atomically retains raw records, groups/findings
  and projection/cursor. Startup replays every retained record from the original
  assertion and checks head/counters/journal; unknown, truncated or corrupt state
  is rejected without resetting it. Configured ledger/payload and 10,000-finding
  budgets stop before deleting evidence. There is no online prune/archive,
  shared-writer SQL backend or follow-journal dispatch/acknowledgement path.
- Added strict 16-KiB config, local-only health/status/Prometheus routes, lag/age/
  capacity-qualified readiness, durable poll/gap-episode metadata, cancellable
  source cycles and graceful shutdown. Fresh processes start not-ready. Stale or
  unavailable claims yield unknown lag; all assurance remains unverified. Progress
  omits endpoint URLs, raw provider errors, watched identities and signing data.
- Added offline follow-inspect with bounded ID-cursor pages and explicit offline
  assurance. Backup now has an explicit read-only, copy-only opener recognizing
  follow/replay roots; legacy replay/health readers still refuse follow state.
  Tests cover snapshot/restore consistency and do not enable cross-engine writes.
- Deterministic typed-XDR tests retained forty ledgers, A→B at 100 and B→A at 117,
  matching IDs for batch sizes 1/8/16 and cold replay. Tests also cover carried
  inactive/absent preimages, skipped ledgers/repair, regressing/stale/wrong-network
  source, cross-batch header mismatch, retention/byte rollback, orphan findings,
  second writers, cancellation/output failure and subprocess exit before commit.
- Read-only real Testnet pilot: fresh explicitly asserted baseline 5413741,
  seventeen new ledgers retained through 5413758, qualified readyz/status HTTP 200
  and metrics ready=1. Retained payload 5,741,419 bytes, zero findings/gap episodes
  in this small run. SIGTERM exit 0; a fresh process fully revalidated and became
  ready at the same cursor. Consistent backup/restore inspection matched exactly.
  No signing identity, transaction or external receiver call. This is not a load
  benchmark, authenticated network-health claim or production deployment.
- Portable candidate: static Linux amd64 build with pinned compiler, per-file and
  archive checksums, commit/dirty metadata, staged non-overwriting installer,
  hardening-oriented systemd USER unit, operations/runtime guides, ADR and schemas.
  Installer validates offline and never starts/enables a unit. Valid install,
  checksum-tamper rejection, no-overwrite and invalid-baseline checks PASS. Static
  user-unit verification PASS with an isolated XDG_RUNTIME_DIR. Cloud PID 1 is
  not systemd; actual unit start, boot persistence and target-host controls are
  unverified. Precommit test packages explicitly carry source_dirty=true.
- Local checks PASS: 222 default top-level Go tests, five fuzz targets' seed runs,
  full race/vet/module verify/tidy-diff, pinned module/source-test/binary advisory
  gates, actionlint, shell syntax, JSON Schemas and actual live status schema checks.
  Four intentional default skips: two subprocess helpers exercised by parents and
  two opt-in Phase 8 evidence tests. Affected follow/CLI race tests rerun after the
  final timing correction; no new dependency or credential requirement. Remote CI
  on this new branch remains unverified; no audit or cross-platform verdict.
- Evidence and topology: docs/research/2026-10-27-follow-pilot.md and JSON,
  docs/FOLLOW.md, docs/OPERATIONS.md and ADR 0004. Raw runtime/DB/binary data stay
  ignored in follow-runs and dist. No pilot process remains running. Portable
  preparation is ready; permanent-host deployment, production archive/retention,
  receiver integration, sizing and fully hosted end-to-end checks remain open.

## Phase 9 host preflight and repeatable package checks — 2026-10-27

- Continues portable preparation from `1a5e2a3` on feat/portable-linux-follow.
  Added packaged/installed `bin/host-check.py`: offline JSON report for Linux
  amd64, required tools, CA path availability, binary/config/baseline validation,
  private user-owned state directory, free filesystem bytes, static user-unit
  verification and user-manager reachability. Raw subprocess output is suppressed.
  No RPC, database open/create, listener, service start or privileged host change.
- Requirements fail with exit 1; warnings permit foreground review with exit 0.
  Every report says production_validated=false. The 1-GiB free-space threshold
  is configurable and is neither a reservation nor a retention estimate. Actual
  service TLS, filesystem suitability, memory/cgroup hardening, reboot/logout,
  source retention, backups and notification delivery remain separate checks.
- Added scripts/check-linux-package.sh and the matching Go CI test-job step.
  The local check PASSes archive checksum/extraction, installation from the
  extracted bundle, offline report and static unit syntax; rejects existing
  destination, invalid baseline, missing install, impossible disk threshold,
  unsafe state-directory modes and tampered payload. No state file or staging
  residue remains. Python assertion-disabled execution is rejected. Only cached
  Go build dependencies are needed; the fixture RPC URL is reserved .invalid.
- Local checks PASS: `go test -timeout=60s -count=1 ./...`,
  `./scripts/check-linux-package.sh`, actionlint, Bash syntax checks for the
  package/installer/check scripts, Python AST parse and `git diff --check`.
  No Go runtime/dependency change; previous race/advisory results remain dated
  evidence rather than newly rerun claims. Remote CI remains unverified.
- Updated OPERATIONS.md and DEVELOPMENT.md with exact commands and report limits.
  No permanent host was provided, provisioned or started. Phase 9 remains
  IN PROGRESS for hosted validation, production retention and follow delivery;
  this does not move the project into a completed Phase 10 or Wave submission.

## Phase 9 independently replayable evidence archives — 2026-10-27

- Continues `84917cc` on feat/portable-linux-follow. Added `follow-archive
  --config FILE --output NEW_FILE` and `verify-follow-archive --input FILE`.
  Export validates existing stopped state through an OS read-only bbolt handle,
  streams original assertion/raw records/applied groups into private gzip NDJSON,
  fsyncs and replay-verifies it before non-overwriting atomic publication. File
  and content SHA256 receipts bind bytes to a separately trusted handoff; they
  do not authenticate producer, consensus, metadata or baseline assertions.
- Offline verification starts from the original assertion, checks every ledger
  and applied event, reconstructs the full projection/finding accounting and
  compares the final head. Cached state cannot substitute for replay. Independent
  compressed/expanded limits are 512 MiB, with the existing record, JSON-line,
  ledger, finding and logical-payload budgets. Cancellation/deadlines apply;
  raw provider/config errors are not exposed in output. No state is created by
  verification or by export when the configured database is missing.
- Tests cover forty synthetic ledgers with upgrade/rollback and two original-ID
  findings, deterministic repeated bytes, read-only access/refused mutation,
  missing/reordered/duplicate evidence, modified events/header/head (also after
  recomputing the digest), malformed/duplicate JSON, truncated gzip/trailer,
  bounds, cancellation, prepublication failure, racing/existing/symlink targets,
  cleanup and CLI negative cases. Archives do not delete or mutate source state.
- Reused previously captured real Testnet pilot entirely offline: original
  baseline 5413741, seventeen ledgers through 5413758, zero derived findings.
  Export and independent verification receipts match; source DB SHA256 unchanged.
  Final read-only export matches the initial archive byte-for-byte. Expanded
  size 5,741,908 bytes; compressed 890,752 bytes. Evidence, exact checksums and
  limits: docs/research/2026-10-27-follow-archive-validation.md and JSON.
  No new RPC call, key, transaction, external receiver or host deployment.
- Local checks PASS: `go test -json -timeout=60s -count=1 ./...` (227 top-level
  tests, six fuzz seed targets, four intentional skips), affected follow/CLI
  race suites, `go vet ./...`, module verify/tidy-diff, actionlint, Bash syntax,
  package/install smoke and diff checks. `FuzzVerifyFollowArchive` ran ten seconds
  with two workers, 833 executions, PASS. Pinned module/source-test/compiled-binary
  vulnerability gates PASS on final runtime changes. Remote CI unverified.
- ARCHIVES.md documents format, commands, assurance, publication/recovery and
  exact bounds; included in package and installer, linked from README/FOLLOW/
  OPERATIONS. This is a usable evidence-preservation prerequisite, not production
  retention completion: no pruning, archive chain/restore/rollover, delivery ack
  or new baseline. Keep source DB and operational backups. Phase 9 remains
  IN PROGRESS for those storage capabilities, follow notification delivery and
  permanent-host end-to-end validation. No release, merge or Wave submission.

## Phase 9 archive recovery into separate state — 2026-10-27

- Continues `65fdb47` on feat/portable-linux-follow. Added
  `restore-follow-archive --input FILE --output NEW_DB --sha256 EXPECTED_DIGEST`.
  The expected compressed-file SHA256 is mandatory. Recovery shares the bounded
  archive verifier, stages accepted records in a private new store, validates
  the complete stream/trailer/digest, closes and fully replays the resulting
  database, fsyncs it, then publishes without overwriting an existing target.
  Cancellation, invalid input/digest and prepublication failure clean ordinary
  staging returns; a killed process may leave private staging for inspection.
  Publication/directory-sync uncertainty never deletes an exposed destination.
- Original assertion/header, complete projection, raw records and exact finding
  IDs/payloads remain. Restore does not rotate the baseline, free capacity, start
  a process, replace live state or acknowledge findings. The receipt includes
  the exact baseline for a separately reviewed config and the restored file's
  SHA256. Evidence replay does not establish chain or producer authentication.
- Operational metadata is explicitly RESET_NOT_ARCHIVED. Recovered cycle starts
  STARTING, with prior poll/gap/success/endpoint-high-water history unavailable;
  a zero recovered counter is not proof of no historical gaps. Fresh readiness
  stays false. Retain operational backups/logs and source evidence separately.
- Synthetic assertion-only and 24-ledger recovery tests preserve heads/findings,
  then continue both original/recovered stores through forty ledgers with exact
  matching results and two upgrade/rollback findings. Negative tests cover wrong
  expected digest, truncated/damaged archive, cancelled/injected publication,
  existing/racing/symlink destinations, temporary cleanup and CLI argument errors.
- Offline real-data recovery PASS: trusted prior archive SHA256, original
  baseline 5413741, seventeen retained ledgers through 5413758, exact head/finding
  comparison and unchanged original DB hash. Recovered DB 16,777,216 bytes;
  exact checksums and reset cycle in docs/research/2026-10-27-follow-restore-
  validation.md and JSON. No new RPC, signing identity, transaction or delivery.
- Local checks PASS: full default suite (229 top-level tests, six fuzz seed
  targets, four intentional skips), affected follow/CLI race suites, vet,
  module verify/tidy-diff, module/source-test/binary vulnerability scans,
  actionlint, package/install smoke and diff checks. Remote CI unverified.
  No dependency, credential or hosting requirement added. Archive format stays
  version 1; prior archive verification and corruption tests pass unchanged.
- Updated README/FOLLOW/OPERATIONS/ARCHIVES with exact commands, protected receipt
  and original-baseline extraction, reset-counter warnings and failure handling.
  Phase 9 remains IN PROGRESS: archive chaining/retention rollover, production
  follow notifications, sizing and permanent-host validation remain unfinished.
  No release, protected merge, host provisioning or Wave submission occurred.

## Phase 9 validated offline physical compaction — 2026-10-27

- Continues `8829523` on feat/portable-linux-follow. Added `follow-compact
  --config FILE --output NEW_DB` for stopped follow state. Opens source read-only,
  replay-validates it, copies all reachable buckets/values/sequences into private
  staged state, and reopens the copy for full follow validation. Whole-store
  logical hashing compares extra buckets, empty values/buckets and sequences,
  not merely the known head. Atomic non-overwriting publication follows file
  fsync; directory-sync/output failures preserve an exposed file for inspection.
- Compaction retains original evidence, baseline, pending finding IDs/payloads
  and operational poll/gap/success/high-water metadata. No baseline rotation,
  acknowledgment, logical capacity increase, source deletion, state-path edit or
  process start. Unlike evidence-only archive recovery, operational history is
  preserved. A fresh follower still requires its own source/readiness checks.
- Cancellable copy transactions target 16 MiB without splitting values. Traversal
  limits: 64 bucket levels, one million entries and 512 MiB reachable key/value
  bytes; final file cap 512 MiB and CLI deadline ten minutes. These are not memory
  or disk reservations. Unknown bucket semantics are not independently certified
  by preserving their exact bytes. Existing logical follow budgets still apply.
- Synthetic tests preserve two upgrade/rollback findings and operational/extra
  data, then continue original and compacted histories identically through forty
  ledgers. Copy chunk sizes 1/1,024/16,777,216 yield equal logical digests. Negative
  tests cover cancellation, corrupt head, excessive nesting, missing source,
  injected publication failure, existing/racing/symlink targets and cleanup.
- Offline real-data check PASS: same original seventeen-ledger Testnet store,
  16,777,216 source bytes to 5,799,936 compacted bytes, unchanged source SHA256,
  entire follow-inspect JSON identical and a fresh evidence archive byte-identical
  to the prior verified archive. No new RPC, transaction or receiver. Exact hashes
  and commands in docs/research/2026-10-27-follow-compact-validation.md and JSON.
- Local checks PASS: full default suite (233 top-level tests, six fuzz seed
  targets, four intentional skips), affected follow/CLI race suites, vet,
  module verify/tidy-diff, module/source-test/binary vulnerability scans,
  actionlint, Bash syntax, package/install smoke and diff checks. Remote CI
  unverified. No dependency or credential change. COMPACTION.md ships in the
  candidate and installer and is linked from FOLLOW/OPERATIONS/BACKUP guides.
- This reduces physical allocation in a separate copy, not logical retention.
  Phase 9 remains IN PROGRESS for archive chaining/rollover, production follow
  delivery, sizing and permanent-host validation. No release, merge, provisioning
  or Wave submission. Keep original state and verified backups until reviewed.

## Phase 9 offline follow delivery bridge — 2026-10-27

- Continues `615eaa6` on feat/portable-linux-follow. Added `follow-sync --config
  FILE --delivery-state FILE --stream ID`. This is an offline operator step:
  source must be stopped; it opens/replays native state read-only, scans all
  hash-ordered findings and atomically imports them into a separate durable
  outbox with a retained event manifest and network/baseline/head/stream binding.
  No sender, source request, configuration change or service start runs at sync.
- Prior sync header must still be present in native history. Rewinds, header/
  baseline conflicts, omitted imported findings and stale expected binding are
  rejected. Every sync scans the full journal; new hash IDs can sort before old
  IDs. Import preserves leases, attempts, acknowledgements, dead letters and
  audited recovery state. Native findings/counters remain unchanged; their
  pending_findings count is not an undelivered-outbox count.
- Added payload schema 3/source follow_ledger_meta with original inner event ID
  and a distinct network/stream/event-bound outer delivery ID. Fixed projected
  uncertainty is validated by storage and sender. Existing deliver/alerts/
  requeue/recoveries source routing supports the imported scope. Claims and
  inspection compare all queued payloads with the retained manifest; manifests
  contain event claims, not raw metadata, and do not independently authenticate
  chain evidence. Legacy payload/queue validation remains separate and passes.
- Reuses existing signed HTTPS sender without weakening public DNS/IP pinning,
  TLS, redirect, response-size, sink-binding or proxy restrictions. Direct-only
  transport still rejects required proxy-backed routing. HMAC authenticates the
  delivered envelope, never baseline/chain/upgrade approval. No external receiver
  is configured or contacted. Schema-3 support is required at future receivers.
- New tests cover forty-ledger upgrade/rollback sync with stable original IDs,
  cold reopen/resync without repeat delivery, backup/ack preservation, dead-letter
  persistence, fenced expired leases and audited recovery, atomic rollback,
  conflicts, omitted findings, manifest/queue corruption, missing source, path
  collisions, prior-source rewind and forged assurance. Native source unchanged.
- Reused captured real Testnet transitions entirely offline: native follow
  baseline 5407791, twelve ledgers through 5407803, three findings delivered to
  the isolated local verified-TLS test receiver. Exact HMAC/idempotency and
  UNDETERMINED assurance verified; cold reopen/resync produced zero new attempts;
  forged APPROVED assurance refused. Separate real seventeen-ledger pilot sync/
  resync correctly imported zero findings and preserved source DB SHA256. Evidence
  in docs/research/2026-10-27-follow-delivery-validation.md and JSON. No new RPC,
  transaction, external delivery or persistent running service.
- Local checks PASS: 239 default top-level tests, six fuzz seed targets, four
  intentional skips; affected store/alert/follow/CLI race suites; separately
  invoked real-data local TLS race test; vet/module verify/tidy-diff; pinned
  module/source-test/binary vulnerability gates; actionlint/Bash/schema syntax;
  package/install smoke and diff checks. Remote CI unverified. No dependency or
  default secret requirement added. Guide and schema ship in the Linux candidate.
- Phase 9 remains IN PROGRESS: unattended sync, source-gap notification policy,
  production receiver/proxy routing, archive chaining/retention rollover, sizing
  and permanent-host validation remain open. No logical retention or source-
  assurance upgrade, release, protected merge, provisioning or Wave submission.

## Phase 9 opt-in automatic local outbox sync — 2026-10-27

- Continues `a670ce3` on feat/portable-linux-follow. Paired follower flags
  --delivery-state FILE --delivery-stream ID enable automatic local queue sync
  at startup and after every nonfatal poll, serialized with native commits.
  Default follower/unit behavior stays off. --check validates paired syntax,
  stream and path separation without opening/creating either database. No
  signing secret, receiver or external dispatch is configured or invoked.
- Extracted shared followdelivery bridge used by manual and automatic sync.
  It validates the original baseline, scans complete bounded findings, checks
  prior sync header ancestry and preserves import/lease/retry/ack/dead-letter
  state. Unchanged bindings/payloads are revalidated without rewriting the outbox.
  The outbox is opened only during sync; temporary dispatcher lock contention
  retries under the two-minute hook deadline, with cancellation between attempts.
- RunWithSync startup repairs native commits not yet imported after interruption.
  Native and outbox commits remain separate transactions. A sync failure persists
  OUTBOX_SYNC_FAILED, clears readiness/endpoint claim, retains the prior success
  timestamp and stops with exit 5. Source evidence/findings remain intact for
  restart repair. Parent cancellation follows graceful STOPPED handling. Fatal
  source/capacity errors retain their own stop behavior; manual sync/restart can
  recover retained findings after their cause is resolved.
- New tests simulate post-source-commit sync failure, fresh-process catch-up
  before healthy emission, startup failure, dispatcher lock retry, cancellation,
  unchanged DB bytes, opt-in offline checks and invalid/path-colliding flags.
  Real captured twelve-ledger range 5407792–5407803 recovers three findings in a
  separate offline test with a synthetic head/RPC adapter. This is not a new live
  freshness/hosting result. No new RPC, transaction or external delivery. Exact
  report and scope in docs/research/2026-10-27-follow-autosync-validation.md/JSON.
- Added optional fleetwatch-sync.service template/installed unit with delivery.db
  under the existing writable state directory and reviewed stable stream pilot.
  Default and alternate unit syntax verified statically; installer enables/starts
  neither and instructs choosing one. Actual host systemd/boot/hardening remains
  unverified. Updated runtime guide, operations, delivery guide, status schema
  and README; no Go dependency or default credential requirement added.
- Local checks PASS: full default suite (242 top-level tests, six fuzz seed
  targets, four intentional skips), affected followdelivery/follow/store/alert/
  CLI race suites, separate captured-data automatic repair test, formatting,
  vet/module verify/tidy-diff, pinned module/source-test/binary vulnerability
  gates, actionlint/Bash/status-schema and package/install checks. Remote CI
  unverified. No source-authentication or logical-retention upgrade is claimed.
- Phase 9 remains IN PROGRESS for external dispatch scheduling, source-gap
  notification policy, production receiver/proxy routing, archive chaining/
  retention rollover, sizing and permanent-host validation. No public release,
  protected merge, host provisioning, actual unit start or Wave submission.

## Phase 9 opt-in repeated webhook dispatcher — 2026-10-27

- Continues `38ab929` on feat/portable-linux-follow. Added deliver-loop with an
  explicit imported network/stream, receiver URL/allowlist and HMAC key environment
  selector. It dispatches only imported follow payloads; no implicit destination,
  source discovery, signing key generation or auto-start. --check verifies local
  options/key/transport policy without opening state, resolving DNS or sending.
- The loop opens existing-only writable delivery state for one bounded batch,
  then releases the file before status/interval waits. Missing and empty files
  wait with unknown queue counts, never created/initialized. Busy/unimported
  state is reported explicitly; nonempty invalid state/manifests stop. Batch
  size 1–5 (default one), interval 1–60 seconds (default five), dispatch deadline
  ninety seconds plus bounded post-batch inspection. Existing sender/lease
  limits, sink binding and retry/dead-letter policy remain unchanged.
- SIGINT/SIGTERM preserve durable attempts and emit STOPPED. At-least-once retry
  after uncertain acknowledgement requires receiver deduplication. Permanent/
  exhausted failures stay DEAD across restart, requiring audited manual recovery.
  Output failure after acknowledgement does not clear/reset delivered rows.
  Fixed NDJSON codes/counts expose waiting, retry, uncertainty, dead letters and
  fatal failure without URL/key/IDs/lease tokens/raw transport errors. Queue
  counts are null when unknown. No chain assurance or retention change.
- Local verified-TLS loop reused the previously captured Testnet transition
  evidence, imported three native follow findings and delivered three requests.
  Exact-body HMAC/idempotency, released file between batches, graceful stop and
  zero acknowledged-repeat attempts after restart PASS. No external delivery or
  new RPC/transaction. Synthetic tests add transient retries, permanent dead
  letters, missing/busy/empty/unimported files, corrupt-state rejection, request
  cancellation, output failure and invalid options/key handling. Evidence in
  docs/research/2026-10-27-delivery-loop-validation.md and JSON.
- Prepared fleetwatch-delivery.service as a separate optional user unit, with
  required private receiver/key environment configuration and local preflight.
  It can accompany one selected follower unit; does not start a follower itself.
  Installer creates no environment file/secret and starts/enables neither role.
  Static unit verification PASS only; real target-host service/boot/hardening and
  receiver setup remain unverified. Direct-only sender still rejects required
  proxy routing; DNS/IP pinning, verified TLS and redirect policy are unchanged.
- Local checks PASS: 247 default top-level tests, six fuzz seed targets, four
  intentional skips; affected alert/store/followdelivery/follow/CLI race suites;
  separately invoked captured-data local TLS loop; formatting/vet/modules/
  tidy-diff; module/source-test/binary vulnerability gates; actionlint/Bash/status
  schema and package checks. Remote CI unverified; no dependency or default
  credential requirement. Runtime guide, schema and rendered unit ship in the
  candidate. Operations topology now includes the separate outbox/dispatcher.
- Phase 9 remains IN PROGRESS for production receiver/proxy routing, source-gap
  notification policy, archive chaining/retention rollover, representative sizing
  and permanent-host validation. No public release, protected merge, provisioning,
  actual service start, new chain write or Wave submission occurred.

## Phase 9 opt-in pinned HTTP/HTTPS proxy transport — 2026-10-27

- Continues `f80c58c` on feat/portable-linux-follow. Added explicit --proxy-mode
  pinned-connect to deliver/deliver-loop and public constructor variant. Default
  reject mode is preserved. Go environment proxy/NO_PROXY selection is respected;
  no proxy setting, network grant, CA trust or credential binding is modified.
- Target DNS/public-address checks run before proxy dialing. CONNECT authority
  and Host both use a locally checked numeric public IP:443, avoiding proxy-side
  receiver DNS. Original receiver hostname remains HTTP Host and TLS SNI/verified
  certificate identity. HTTPS proxies have independently verified proxy-host TLS.
  Both roles keep TLS 1.2 minimum and OS CA trust. No insecure CLI switch exists.
- Selected proxy URL Basic credentials go only on CONNECT, not receiver headers
  or logs. Headers are capped at sixteen KiB, handshake IO at ten seconds or
  earlier context deadline, with cancellation closing the socket. Buffered bytes
  survive CONNECT for target TLS. Denials/redirects/unsafe DNS do not cause direct
  or hostname fallback. Numeric-target proxy policy rejection remains rejection.
- Local fixture tests PASS for HTTP and HTTPS proxy tunnels, numeric authority,
  correct receiver Host/SNI/HMAC/idempotency, credential separation, untrusted
  certificates at either TLS role, private/mixed DNS rejection, denied CONNECT,
  redirects, oversized headers, cancellation, URL/mode validation and no-IO public
  constructor/CLI preflight. Fixtures route only to fixed loopback endpoints;
  no actual public target or external proxy/receiver call occurred.
- Optional delivery unit defaults FLEETWATCH_DELIVERY_PROXY_MODE to reject and
  permits reviewed host environment selection of pinned-connect. Updated proxy/
  delivery/runtime guides and README; new guide ships in the Linux candidate.
  No environment file/secret/recipient is generated and no unit enabled/started.
  Evidence and protocol scope: docs/research/2026-10-27-webhook-proxy-validation.md.
- Local checks PASS: 251 top-level default tests, six fuzz seed targets, four
  intentional skips; affected alert/CLI race suites; formatting/vet/module
  verify/tidy-diff; pinned module/source-test/binary vulnerability gates;
  actionlint/Bash/package/install checks. Remote CI unverified. No dependency
  change or external audit. No source-authentication or retention upgrade.
- Phase 9 remains IN PROGRESS: real proxy permission/receiver compatibility,
  archive chaining/retention rollover, source-gap notification policy, sizing and
  permanent-host validation remain open. Real cloud sidecar numeric CONNECT
  compatibility is unverified and may be denied; no workaround or policy bypass
  is introduced. No release, protected merge, hosting provision or Wave submission.

## Phase 9 zero-cost handoff and explicit deployment gate — 2026-10-27

- User instructed continuing through Phase 9, following Dripwave and using zero-
  cost deployment only. The earlier paid-VM suggestion is superseded; no paid
  resource/account was created. Current environment is a development executor,
  PID 1 tail, with no connected provider identity/capability/SSH host or authorized
  receiver. It is not a permanent production host. The Dripwave gate explicitly
  requires the deployed topology to work end-to-end in production.
- Completed a zero-cost deployment handoff for an existing owner-controlled
  host or a confirmed free persistent VM, without inventing provider eligibility.
  Official Oracle free-tier/documentation reads returned HTTP 403 via the normal
  proxy; free allocation, availability and cost safeguards remain unverified.
  No policy bypass, billable trial, hardware purchase, credential or provider
  permission was introduced. Paid fallback is disallowed by the user constraint.
- Added Linux arm64 alongside default amd64 packaging, with target-specific Go
  dependency notices/BUILD metadata. Both ELF/static targets are inspected;
  only native amd64 executes here. Installer verifies architecture before staging
  or running the binary and copies BUILD.json into the install. Mismatched target
  installation is rejected without creating a destination. ARM runtime, actual
  free host, unit operation and boot behavior remain unverified.
- Added bundled read-only phase9-preflight collector and concrete acceptance
  table. Optional --running contacts only fixed loopback status/readiness; no
  source/receiver request or service start. The collector always emits
  phase9_complete=false and production_validated=false; successful collection
  cannot fabricate host/receiver/cost/retention acceptance. Offline, mock-running,
  public-listener rejection and argument checks PASS.
- Measured five fresh processes per operation on the same captured seventeen-
  ledger pilot: startup inspection median 0.490640 s / max RSS 25,024 KiB;
  archive verification median 0.349744 s / max RSS 23,340 KiB. Warm filesystem,
  isolated copies, original DB unchanged. Detailed samples are retained in
  docs/research/2026-10-27-phase9-sizing.json. Extrapolating this tiny sample gives
  roughly 16.6 minutes at 64 MiB / 66.2 minutes at 256 MiB, not a production
  forecast. A reviewed capacity/retention policy is materially necessary; a
  larger VM alone does not bypass logical limits or prove unattended operation.
- Independent review found one wrong-path integrity defect: writable bbolt open
  could change foreign NoFreelistSync metadata before rejecting its namespace.
  Fixed OpenExisting in `d65f200` with read-only recognition before writable open, pinned
  file identity and no creation. Regression tests preserve foreign/invalid/swapped
  file bytes and confirm valid imported outboxes still persist acknowledgements.
  No proxy defect requiring changes was found; this is not an external audit.
- Local checks PASS: 254 default top-level tests, six fuzz seed targets, four
  intentional skips; affected store/alert/followdelivery/CLI race tests;
  formatting/vet/modules/tidy-diff; pinned module/source-test/binary vulnerability
  scans; actionlint/Bash/Python checks; two-architecture package/ELF metadata,
  native install/preflight, mismatch rejection and all existing package failure
  cases. Remote CI unverified. No dependency or default secret requirement added.
- Phase 9 is BLOCKED, not complete: actual confirmed-zero-cost host/access,
  authorized receiver/key, real routing, service/reboot/backup/resume evidence
  and reviewed host capacity remain outstanding. Shared SQL, full lifecycle and
  independent chain authentication are separate roadmap gaps, not reasons to
  keep inventing unrelated Phase 9 utility milestones. Do not proceed past the
  real hosting gate or claim approval. No source assurance upgrade, release,
  protected merge, service start, chain write or Wave submission occurred.

## Zero-host browser delivery increment

- Continued from pushed 38ddf56 after the user explicitly rejected supplying a
  machine/account. Added a self-contained Go WebAssembly current-state viewer,
  default actual Testnet watchlist, qualified live values, visible-tab refresh,
  stale handling and schema-v2 raw evidence download. No server, key or signup.
  Native sequential state, assurance and outbox semantics are unchanged.
- Browser transport explicitly uses Fetch CORS/omit-credentials/error-redirect;
  the native HTTP transport retains inherited proxy/TLS behavior. Package embeds
  all runtime/dependency notices and records source/dirty state plus hashes.
- All default Go tests PASS. Source race, native/WASM vet, module verify/tidy,
  workflow/JS/Python syntax checks PASS. Chromium/Playwright integration PASS in
  ten deterministic scenarios. CI now cross-builds WASM; remote CI unverified.
- Real Testnet observation at 2026-10-27T19:14:03Z: ledger 5420960, two contracts,
  one owner/tag reference, three raw records. The live browser check used a
  TLS-verified test relay because this managed Chromium lacks the proxy CA;
  direct CORS preflight/POST origin headers were verified separately. File URLs
  are blocked by this managed browser policy; no policy or TLS bypass was used.
  See docs/research/2026-10-27-browser-validation.md and the full qualified capture.
- GitHub Actions was rejected as perpetual serverless hosting based on current
  official terms, default-branch schedule behavior and private billing limits.
  Static Pages is a valid optional host, but activation/visibility cannot be
  inspected via the currently denied GitHub API. Downloadable HTML removes that
  prerequisite for use; public hosting is not claimed from branch publication.
- Phase 9 remains incomplete for a verified public deployment/unattended native
  service. The browser mode does not silently replace sequential history with
  polling or claim permanent monitoring. No Linux machine is requested for it.
  No paid resource, account, secret, real-asset write, external send, merge, tag
  or Wave submission was performed.

- Publication receipt: clean browser source `80c2019`, runnable distribution
  `06252fc` pushed to `feat/portable-linux-follow`; static distribution `91edd4d`
  pushed to new `gh-pages`. Every package SHA verified. No main merge or tag.
  Both branches are saved remotely; Pages activation remains UNVERIFIED. Saved
  the minimal network draft additions api.github.com and
  soroban-fleetwatch.github.io through the supported onboarding tool; confirmed
  saved with requires_publish=true. It has not applied to this running instance.
  Review/save/Publish in environment settings is the product-owned activation
  step; do not extract Git proxy credentials or bypass CONNECT denial. See
  docs/research/2026-10-27-browser-publication.json.
