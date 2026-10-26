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
| S4 Sequential historical ingest | IN PROGRESS: durable diagnostic reconstruction | Bounded getLedgers collection, asserted-baseline application, operation-boundary findings and raw-evidence checkpoints with crash/restart tests | Authenticated baseline/metadata commitments, lifecycle reconciliation, production retention and Postgres remain |
| S5 Watcher/alerts | IN PROGRESS: snapshot/reconstruction outboxes | Atomic outboxes, leased retries, manual recovery/audit, reconstruction health and signed local TLS delivery tests | Continuous watcher, live telemetry and proxy-aware delivery transport remain |
| S6 Signed approvals | IN PROGRESS: standalone verifier | Ed25519 statement/trust/threshold tests and public synthetic vector | Signed trust distribution, trustworthy chain evidence and policy integration remain |
| S7 CLI/API integration | IN PROGRESS: experimental CLI | Capture/replay/outbox/approval commands, snapshot/reconstruction doctor and fleet reports with JSON Schemas | Stable configuration, full help/exit compatibility and optional API decision remain |
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

1. Extend bounded diagnostic reconstruction checkpoints into authenticated continuous ingestion: trusted baseline, metadata commitments, full lifecycle reconciliation, durable gap/delivery handling, production retention and restart recovery. Never relabel current-state polling or supplied snapshots as comprehensive chain history.
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
