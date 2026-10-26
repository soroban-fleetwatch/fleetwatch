# Release Evidence and Acceptance Matrix

A milestone is `PASS` only after proof exists in the actual repo/CI. Codex must update `docs/build/STATUS.md` per stage with command output summaries, links, and commit refs. This matrix starts as `NOT RUN` for all **remote** work; included M1 code is a local prototype, not a CI verification.

| Stage | Required checks | Evidence / rejection rule |
|---|---|---|
| S0 Baseline | `git status`, inspected remote, `go test`, `go test -race`, `go vet`, `gofmt`, M1 demo | Actual SHA, test logs, correct Apache license/module. Reject if repository unclear or starter overwrote newer commits. |
| S1 Research | Official CAP-85, SDK/XDR, RPC retention, Protocol 30 and Wave sources reviewed; ADRs | Date-stamped URLs and verified type/function names; reject invented APIs or unsupported novelty claims. |
| S2 RPC capture | Valid instance/ref typed XDR; consistency checks; mock failure matrix | Tests for invalid/non-UTF8 tag, owner/tag resolution, mixed heights, wrong network, stale/partial, timeout; reject silent default COMPLETE. |
| S3 Testnet demo | Tag A→B and two contracts, actual ledger and tx evidence; repeatable scripts | Real network proof or `BLOCKED`; synthetic alone cannot PASS live Testnet. |
| S4 Sequential history | A→B→A both observed; idempotent double replay; restart; gap | History fixture and DB assertions; reject if polling misses intermediate transition but is described as comprehensive. |
| S5 Watch/alerts | Durable idempotent incident, retries, dedup, HMAC, source gap alarm | Crash/restart and malicious URL tests; reject duplicated alerts, silent failure, or leaks. |
| S6 Signed approval | Positive and forged/expired/revoked/rotated fixtures; exact tuple | Verified signatures and trust root; reject ambiguous policy-match-as-authorization output. |
| S7 CLI/API | Help/version, config, JSON schema, golden examples, auth if API | Exact commands exercised, negative cases, stable exit code contract. |
| S8 Harden | Fuzz/race, malformed XDR, property suite, dependency scanner, resource limits | Real CI logs, attack corpus and measured baseline; reject 'audited' claim without external audit. |
| S9 Hygiene/docs | Docs site builds, README examples pass, security policy real, issue script --dry-run | Valid links only, actual CI job names, no fake community handles. |
| S10 RC | Clean-room build and smoke, cross-platform binaries, checksum, migration/backup simulation | Artifact SHA + smoke logs, draft release notes. Publishing gated on user approval. |
| S11 Wave | Live approved-list and eligibility check, demo, form-ready real links | Application *draft* only; acceptance/submission is external and owner-controlled. |
| S12 Sustain | Maintenance and response plan, protocol upgrade matrix, issue triage | Owner-ready runbook and first maintenance checklist, not fictional operations metrics. |

## Focused adversarial test inventory

1. Raw tag bytes: `00`, `ff0061`, Unicode valid UTF-8, invalid byte sequences, same bytes uppercase hex, identical printable text but different bytes, empty tag rejected per proven CAP constraints.
2. Multi-network: same contract ID and tag across Testnet/Mainnet must never coalesce; wrong passphrase must fail closed.
3. Mixed-ledger current-state RPC responses: return PARTIAL/UNKNOWN with details, no false approved/no-risk verdict.
4. Owner ref missing, archived, stale and failed HTTP: distinct errors where known; no inferred on-chain deletion.
5. Contract membership changes simultaneous with ref change: show before and after sets and label the impact scope.
6. Replay A→B→A: two transitions, two stable IDs, correct final hash even after duplicate ingest and crash.
7. Direct wasm→external_ref, external_ref→external_ref, external_ref→wasm: unique repoint findings with source evidence.
8. Update source conflicting with read replica at same height: SOURCE_DIVERGENCE and quarantine.
9. Replay/alert injection and SSRF: reject arbitrary internal webhook targets, no secret in error messages.
10. Approval forgery/expiry/revocation: fail decisively; unverified unsigned policy never recorded as authorized.
11. Unbounded list/tag/file and malformed protobuf/XDR/JSON: bounded parse, context cancellation, fuzz nonpanic.
12. Old protocol ledger and pre-P30 vs P30 ledger header decoding: SDK/XDR behavior recorded per fixture.

## Suggested CI gates

- Unit: tests, race tests (supported OS), vet, gofmt, staticcheck as justified, go mod tidy verification.
- Integration: Postgres ephemeral service or isolated test database; no external secret for offline tests.
- Fixtures: local Soroban test harness optionally Rust; Testnet E2E opt-in, tagged separately to avoid flaky PR CI.
- Security: go vuln scan plus dependency licensing and secret scanning; updates via pinned actions.
- Docs: MkDocs build/link checks and checked example outputs.
- Release dry run: nonpublishing artifacts and SBOM metadata.

Never add a green badge until its link is real and working; never invent numeric test coverage or availability guarantees.
