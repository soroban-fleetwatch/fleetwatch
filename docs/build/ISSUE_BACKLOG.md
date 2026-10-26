# Implementation Backlog — dependency-ordered GitHub issues

These are **planned**, not yet created. Codex may prepare `scripts/create-issues.sh --dry-run` and corresponding `.github/ISSUE_TEMPLATE` files. Bulk creation is approval-gated. Complexity: S=1–2 days, M=3–5, L=6+; these are estimates. All tickets must add actual file paths and issue/PR references after implementation.

## Foundation and evidence

**FW-001 — chore(repo): reconcile M1 source, module path and Apache license** — S. Depends: verified remote. Acceptance: no overwrite of remote; exact module path; agreed license; tests pass; status report and PR; docs no longer claim live integration.

**FW-002 — fix(core): bound and validate snapshot inputs and failure thresholds** — S. Depends: FW-001. Acceptance: unknown `fail-on` rejected; malformed IDs and tags exercised; fuzz corpus; deterministic comparator reports; no regression.

**FW-003 — docs(research): verify CAP-85 and pre/post-P30 Go/XDR compatibility** — M. Depends: FW-001. Acceptance: dated authoritative references, pin candidates and decision, XDR type mapping with tests, no invented methods.

**FW-004 — docs(adr): define chain evidence, scope, history and operator trust boundaries** — M. Depends: FW-003. Acceptance: data contracts, completeness states, network-scoped keys, diagrams and threat model reviewed.

**FW-005 — feat(schema): version evidence and reporting formats** — M. Depends: FW-004. Acceptance: JSON Schema, migration strategy, golden synthetic fixtures, no silent v1 breakage.

## Actual chain ingestion

**FW-006 — feat(source): add testable typed Stellar RPC interface** — M. Depends: FW-003, FW-005. Acceptance: mock RPC and supported Go SDK, retry/cancel, network passphrase checks, bounded concurrency and fixture tests.

**FW-007 — feat(source): decode all contract executable variants** — M. Depends: FW-006. Acceptance: Wasm, Stellar Asset, CAP-85 external-ref XDR test cases, malformed XDR fails explicitly.

**FW-008 — feat(source): resolve persistent owner/tag entry with opaque bytes** — M. Depends: FW-007. Acceptance: NUL/invalid UTF-8 roundtrip; exact ledger key; 32-byte hash and errors verified.

**FW-009 — fix(source): enforce ledger consistency and capture scope** — M. Depends: FW-008. Acceptance: mixed/stale/partial multi-call captures never emit COMPLETE or false safe verdict; observed/captured scope labeled.

**FW-010 — test(testnet): provide reproducible CAP-85 fixture contracts and scripts** — L. Depends: FW-009. Acceptance: controlled two-instance shared tag with real before/after tx IDs and ledger proof on Testnet; fallback local fixture labeled if blocked.

## Sequential evidence and history

**FW-011 — feat(ingest): ingest ordered ledger-meta change stream** — L. Depends: FW-003, FW-005. Acceptance: real supported source, documented retention, create/update/remove conversion, full-interval fixtures, mixed protocol versions.

**FW-012 — feat(store): persist versions, evidence and atomic checkpoints** — L. Depends: FW-011. Acceptance: migrations, network separation, uniqueness, partial updates rollback, backup/restore and double replay.

**FW-013 — feat(detector): emit ledger-accurate reference/repoint transitions** — M. Depends: FW-011, FW-012. Acceptance: A→B→A two events, direct↔external repoint scenarios, exact affected observed membership, durable findings.

**FW-014 — feat(reconcile): flag missing ledgers and divergent sources** — M. Depends: FW-012, FW-013. Acceptance: gap alarm, no false negative all-clear, gap replay recovery, source mismatches quarantined.

## Watcher, policy and alerts

**FW-015 — feat(watch): implement resumable worker with health metrics** — L. Depends: FW-012, FW-014. Acceptance: restart, bounded catch-up, context cancel, signals, lag/gap metrics, no duplicate events.

**FW-016 — feat(alert): transactional outbox and signed webhooks** — L. Depends: FW-015. Acceptance: idempotency, retries, dead-letter, signatures, HTTP egress controls/SSRF tests, redaction and local receiver.

**FW-017 — feat(policy): separate unsigned exact match from trusted approval** — S. Depends: FW-005, FW-013. Acceptance: status model and CLI clearly show unverified match vs signed authorization.

**FW-018 — feat(policy): add signed attestations and trust policy** — L. Depends: FW-017. Acceptance: exact contract/network/tag/hash binding, valid/tampered/expired/revoked/rotated fixtures, signature verification explained.

**FW-019 — feat(analysis): add traceable blast-radius queries** — M. Depends: FW-013. Acceptance: known/unknown member boundaries, at-ledger query, confidence and proof link.

## Product and ship

**FW-020 — feat(cli): finalize capture/watch/replay/report/doctor interface** — M. Depends: FW-009, FW-015. Acceptance: stable examples, help, errors, exit codes, config validation, compatibility tests.

**FW-021 — feat(api): implement optional read-only security API** — M. Depends: FW-019. Acceptance: documented decision whether needed; if included, auth, rate limiting, pagination and OpenAPI tests.

**FW-022 — test(security): fuzz XDR, resilience and source-trust failures** — L. Depends: FW-016, FW-018. Acceptance: race+fuzz corpus, resource limits, dependencies, measured perf and updated threat model.

**FW-023 — docs(site): publish complete docs source and examples** — M. Depends: FW-020, FW-022. Acceptance: MkDocs local build, diagrams, tutorial, operator/SRE runbook, security disclosure, contribution guidance.

**FW-024 — chore(ci): harden Actions and create release-candidate dry run** — M. Depends: FW-022. Acceptance: actual green CI names, unprivileged PR checks, checksums, artifact/provenance build, no auto-publication.

**FW-025 — docs(demo): create real operator demo and reproduction evidence** — M. Depends: FW-010, FW-020, FW-023. Acceptance: actual Testnet evidence, clean machine steps and recording script, redacted logs.

**FW-026 — docs(wave): prepare evidence-led submission (optional)** — M. Depends: FW-023, FW-025. Acceptance: current eligibility/approved repo search, no fictitious on-chain contract, links real, submission NOT sent.

**FW-027 — chore(maintenance): define SLA proposal, incident response and version policy** — S. Depends: FW-024. Acceptance: maintenance runbook, backlog and version matrix; do not claim actual SLA performance.

## Proposed GitHub labels

`type:feature`, `type:bug`, `type:security`, `type:docs`, `type:infra`, `type:test`, `complexity:S`, `complexity:M`, `complexity:L`, `priority:P0`, `priority:P1`, `priority:P2`, `needs:evidence`, `blocked:external`.

**Issue template (required each):** Summary; Why it matters; Dependencies; Acceptance Criteria as checkboxes; Design details and relevant source links; Test evidence needed; Tech Stack; Explicit non-goals. Issues must not claim they are approved grant bounties. Before mass creation, search exact titles to skip duplicates and print a dry-run preview; owner must authorize `--apply`.
