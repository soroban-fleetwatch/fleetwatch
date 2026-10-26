# Soroban FleetWatch — Complete Build Plan (Start → Release → Maintenance)

**Edition:** 2026-10-26 | **Status:** planning/handoff, NOT executed on your remote GitHub repo. | **Owner:** repository maintainer | **Implementation agent:** Codex.
**Project:** FleetWatch, an independently deployed off-chain monitor for Soroban CAP-85 executable-reference fleets. **Language:** Go first. **License intention:** Apache-2.0 (subject to verifying repository ownership and existing license). **No production smart contract required.**

## A. Product intent and differentiator

Soroban CAP-85 allows an instance's executable to be a reference to `{executable_owner, opaque_tag}` instead of a direct Wasm hash. The owner contract maintains a persistent tag-keyed ledger entry containing the Wasm hash; updating the owner's tag can change the executable resolved by **all** contracts referencing that tag. Existing pre-deployment tools compare individual Wasm builds. **FleetWatch watches actual observed code associations across ledger changes**, calculates blast radius, checks approved-release intent, and preserves evidence with warnings when coverage is incomplete.

Non-goals: custody, transaction signing, automated incident containment, attacking contracts, asset movement, generalized blockchain explorer, smart-contract compiler, guaranteed exhaustive fleet discovery when the source cannot support it, or replacing an independent audit. Do not claim live-chain coverage until implemented and verified.

### Users / core jobs

| Persona | Job | Proof they can do it |
|---|---|---|
| Protocol maintainer | Watch a known fleet for unexpected code changes | Register fleet, observe a controlled code update, receive precise alert |
| Security responder | Determine what code was running and which instances were impacted at a ledger | Query `{owner,tag}` history, impacted contract IDs, tx/ledger evidence |
| Release engineer | Confirm a change matches explicitly approved artifact and authorization policy | Sign/verify change approval against observed before/after hashes |
| Operator/SRE | Keep the watcher running and understand uncertainty | Restart/resume, show last processed ledger, data gaps, health, dedup |
| Contributor | Reproduce bugs from deterministic data | Clone, test, replay fixture, run docs example |

### Non-negotiable constraints

1. Source credibility: captured chain state must have an evidence source and height. `getLedgerEntries` is current-state lookup; multiple calls may not constitute a single snapshot if the ledger advances. The collector must record and validate consistency, or mark `PARTIAL`/`UNKNOWN` and withhold clean verdicts. Continuous history requires sequential ledger-change ingestion.
2. Identities are network-scoped. Do not use unqualified contract IDs, normalize opaque tags or infer owner admin authority from executable ownership.
3. Never conflate `approval policy matched` and `signature verified`; evidence of a code change is not evidence of maliciousness.
4. Distinguish true missing state from failure to observe. Incomplete fleet coverage must be visible in UI, CLI, API, reports, and notifications.
5. No secret or signing key is necessary for read-only operation. Testnet fixture signing keys must be test-only and excluded from tracked code.
6. One dedicated repo/organization independent of Project 9 and SoroIndex. External adapters can integrate later, but remain optional.

## B. Dripwave skill adaptation — phase-by-phase

The uploaded Dripwave skill specifies 13 phases. Here is the direct mapping without forcing an irrelevant smart contract/frontend onto an off-chain security service.

| Original phase | Required FleetWatch adaptation | Gate/document |
|---|---|---|
| 1 Ecosystem reconnaissance | Live CAP-85, SDK/RPC, approved Wave repos, threat landscape, competitive comparison | `docs/research/`, evidence links |
| 2 Idea generation | Preserve rationale for selecting FleetWatch over alternative security tooling; no need to reselect five ideas | `docs/product/positioning.md` |
| 3 Critical review | Evaluate data completeness, source lag, dynamic coverage, security risks, competitor overlap; explicit go/no-go | `docs/product/feasibility.md` |
| 4 Naming/scoping/repo | Confirm real repo; Go-only monitor in one repo; module/LICENSE/branch plan | `docs/adr/0001-repository.md` |
| 5 Contract architecture | **Replace** on-chain contract specification with off-chain data model, ingestion, evidence, policies, alerts and trust boundaries. Only write tiny Testnet fixture contracts as needed | `docs/ARCHITECTURE.md`, ADRs, schemas |
| 6 Contract coding-agent prompt | **Replace** with complete parser/XDR/ingestion/ledger-fixture instruction package | `CODEX_MASTER_PROMPT.md`, stage tasks |
| 7 App coding-agent prompt | **Replace** with Go service, monitoring, CLI/API, persistence, tests and docs instructions | `CODEX_MASTER_PROMPT.md`, stage tasks |
| 8 Local environment/deployment | Pin compatible Go/SDK; unit tests, simulated ledger, reproducible Testnet verification | CI/test evidence |
| 9 Hosting/topology | Stateful collector and DB with read-only RPC/ledger feeds; optional docs hosted separately; no frontend dependency | deployment runbook, network diagram |
| 10 Repo hygiene | Apache license, security policy, contributor guide, real CI, topics, protected PR flow, issue generator | GitHub checks + evidence |
| 11 Docs site | CLI reference, data provenance, CAP-85 mechanics, operations guides, security/limitations, API guide | MkDocs or equivalent |
| 12 Wave submission | Optional: **live** eligibility/status search, proof-backed demo, links and planned issues; submit only with approval | `docs/wave/SUBMISSION_DRAFT.md` |
| 13 Post-approval | Triage, reliability incidents, releases, contributor issues, SDK protocol updates | maintenance policy and release plan |

*Never mark a Dripwave phase complete because a document mentions it: the phase requires actual tested outputs. Do not claim Wave eligibility or approval simply because a repo is open source.*

## C. Architecture and data flow

```text
                        Stellar network (Testnet first)
                         ledger data, contract state
                                   │
       ┌───────────────────────────┴────────────────────────────┐
       │                                                        │
       ▼                                                        ▼
Current-state Stellar RPC                                    Sequential ledger meta
(getLedgerEntries / network info)                            (Ingest SDK / Galexie / archive)
       │                                                        │
       ▼                                                        ▼
Typed CAP-85 resolver                                       Ledger change processor
       │  source=RPC; completeness=qualified                     │  source=history; checkpointed
       └───────────────────────────┬────────────────────────────┘
                                   ▼
                    Normalization + provenance validator
                                   │
                      ┌────────────┴────────────┐
                      ▼                         ▼
               Contract/executable DB     Gaps/coverage ledger
                      │                         │
                      └────────────┬────────────┘
                                   ▼
                         Transition detector
                        /              \
                  approval engine    blast-radius index
                        \              /
                         └──────┬───────┘
                                ▼
                           Alert outbox
                                │
                   ┌────────────┼─────────────┐
                   ▼            ▼             ▼
                 CLI/JSON    Webhooks       REST API (optional)
                                │
                              Operator
```

- **Read path:** do not make extra chain calls from the core comparator; inject `Source` adapters.
- **Identity:** contract key `(network_passphrase, contract_id)`; external tag key `(network_passphrase, owner_contract_id, tag_hex)`; version hash 32 bytes.
- **Completeness:** cover a configured fleet precisely first. Discovery of *every* deployed contract belongs to a separately proven full-ledger index, not to a network-wide promise based on current RPC.
- **Persistence:** store both source observation and derived finding with evidence/version so replay after a parser correction is possible.
- **Alert evaluation:** only a complete, reliable transition can yield a definitive upgrade finding. Partial observations create `OBSERVATION_GAP` or `REQUIRES_VERIFICATION`, never a fabricated `NO_RISK`.

## D. Milestones / execution order

### S0 — Repo assessment, code import, baseline repair

**Inputs:** verified destination repo; M1 ZIP (`starter-code/`) and this handoff.
**Work:** read actual remote before touching it. Determine empty/nonempty, default branch, PR rules, current CI and license; compare M1 source to existing files. If empty, import M1; otherwise merge carefully. Repair placeholder Go module and imports; align Apache-2.0 notice with legal provenance. Check old code for correctness issues: `ShouldFail` unknown threshold handling; unbounded or mismatched owner/contract address validation; explicit completeness; byte-exact case folding; report determinism; CLI exit status; tests. Preserve M1 behavior until replacements have tests. Create `docs/build/STATUS.md` and ADR-0001.

**Evidence:** recorded repo URL/default branch and tree, `git diff`, `go version`, exact tests, CI job names.
**Exit gate:** clean Go formatting, `go test ./...`, `go vet ./...`, `go test -race ./...`; M1 synthetic examples reproduce expected unapproved and approved findings; no mock data misrepresented as on-chain. License/module path match verified repository, PR created from working branch when possible.

### S1 — Research, feasibility and testable architecture

**Work:** verify current CAP-85 source, typed XDR variants and actual Stellar Go SDK APIs; confirm pre-/post-Protocol-30 decoding; search live Wave list and current comparator/monitor alternatives. Explain trust of public RPC, historical source selection, ledger finality, field constraints, deploy/read permissions. ADRs for source selection, source consistency, persistence, alert delivery, approvals. Precisely document version and protocol ranges tested. Define JSON Schemas v2 for source evidence, reference observations, contract membership and finding/status. Enumerate error codes.

**Exit gate:** links and date-stamped research; a competitor gap table; model/code boundaries, fixtures and acceptance matrix signed off in PR; no invented API paths.

### S2 — M2: typed current-state Stellar reader (CAP-85)

**Work:** implement a `Source` interface with a strict `MockSource`, `RPCSource` and typed XDR decoding based on actual SDK. Read instance executable kind (direct Wasm, Stellar Asset, external-ref) and persistent owner/tag ledger entry with `SCV_EXECUTABLE_TAG` key; validate value is an approved Wasm-hash representation per CAP-85. Preserve raw bytes. Verify address/network passphrase, rate-limit, retry with contexts, timeouts and bounded queue. Detect source ledger advances during a multi-call capture: enforce verifiable single-ledger state where possible; otherwise declare inconsistent/partial instead of assuming consistency. Provide CLI `capture` with explicit watchlist input and evidence envelope. No blind all-contract discovery.

**Exit gate:** typed fixture corpus and fake RPC tests cover every executable kind, missing owner reference, malformed Wasm, invalid tags, mixed heights, HTTP failures, revoked endpoint, unsupported protocol, delayed ledger and wrong network. Default demo remains offline; successful captures store actual metadata and have honest source consistency semantics.

### S3 — Live Testnet validation of CAP-85

**Work:** check actual Testnet protocol version is at least 28. Build minimal reproducible *test-only* Rust fixtures or reuse verified fixtures (tag owner + two external-reference contracts). Deploy with Testnet funding only if required, store contract IDs and transaction hashes generated by execution, not placeholders. Capture before state, update one owner tag to a second valid Wasm hash, capture after; verify both contract members are impacted. Independently inspect chain transaction/ledger evidence. Demonstrate direct-repoint and observation-gap cases. Commit scripts but never commit secrets. If the network upgrade is not active or endpoints fail, use deterministic locally simulated ledgers as fallback and mark LIVE acceptance BLOCKED.

**Exit gate:** copy-paste `make demo-testnet` or `scripts/demo-testnet.sh` succeeds from clean Testnet setup, with a recorded Testnet ledger sequence/tx hash, sanitized console transcript and documented limits. No Mainnet writes.

### S4 — M3: complete historical ingestion and durable state

**Work:** implement sequence-based ledger meta input through verified Stellar Ingest SDK/Galexie/archive path; parse ledger entry create/update/remove and executable updates, map owner tag changes, contract instance relations, membership transitions. Treat data retention windows explicitly. Add idempotent append-only observation/event store and checkpoints. Choose Postgres for service, migration tool and indexes; optional SQLite/offline read path if worthwhile and justified. Ensure A→B→A is captured and replayed. Handle ledger gaps, out-of-order duplicates, endpoint failures and restarts. Build scope-aware blast radius: known fleet members with exact coverage, separately label unobserved contracts.

**Exit gate:** replay fixed range twice without duplicates; A→B→A produces two transitions; deliberate skipped ledger produces a GAP incident and suppresses all-clear; cold restart resumes at correct committed cursor; bounded historical Testnet ledger range is independently checked.

### S5 — M4: daemon, alerting and telemetry

**Work:** `watch` loop that advances sequentially, durable outbox/event keys, webhook integration with HMAC signatures, optional Slack connector as ordinary webhook payload, exponential backoff/dead-letter. Severity policy grounded in evidence, threshold config; separate `CRITICAL_CODE_TRANSITION`, `APPROVED_CODE_TRANSITION`, `DIRECT_REPOINT`, `OBSERVATION_GAP`, `POLICY_INVALID`, and `SOURCE_DIVERGENCE`. Health/readiness endpoints only if service mode is enabled, metrics for lag, gap count, alert queue depth and delivery rate. SSRF-resistant webhook endpoint allowlist, HTTPS, safe logging and bounded payloads.

**Exit gate:** run watch through test fixture upgrade; exactly one stable incident per transition despite crash/restart and retries. Alert includes owner, hex tag, before/after hash, affected contract scope, source and ledger proof, policy verification status; no secret in logs.

### S6 — M5: cryptographic approvals and policy provenance

**Work:** separate unsigned policy v1 from a signed trustable approval format. Adopt a verifiable signature container such as DSSE/in-toto (subject to protocol/design ADR), cryptographic Ed25519 verification, network-scoped `owner,tag,from,to`, validity interval, signer roles, key rotation and revocation rules, replay resistance and expiry enforcement; optionally link source commit, Wasm hash, audit reference. Never consider policy-file presence cryptographic approval. Define behavior for unknown signers and partial evidence. Keep the product independent of Audit-to-Deployment Monitor #3.

**Exit gate:** positive/negative signature fixtures, wrong-field binding, expired approval, tampered payload, revoked key, rotated key and trust-bootstrap tests; authenticated match explicitly different from unsigned match; human report explains assurance level.

### S7 — CLI/API/product integrations

**Work:** public CLI (`validate`, `compare`, `capture`, `watch`, `replay`, `report`, `doctor`, `version`) with a stable user config schema and `--help`. Keep legacy M1 snapshot JSON compatible or ship a documented migration path. Add JSON/text/SARIF if worthwhile; stable severity/exit codes. Optional read-only HTTP API for state/fleet/findings, OpenAPI spec, auth and pagination; no dashboard unless end users require one. Add Docker Compose example for watcher/Postgres and minimal alert simulator; isolate docs site from service.

**Exit gate:** integration examples, golden CLI outputs, schema compatibility tests, negative cases, OpenAPI/API tests if included, install-from-binary on Linux/macOS/Windows (or explicitly test/mark unsupported).

### S8 — Security, resilience, performance and QA

**Work:** adversarial parsing/fuzzing of XDR and snapshot input; race test, vet, dependency vulnerability scan, license/SBOM assessment, load testing on a representative fleet. Failure-injection: network partition, stale snapshot, inconsistent heights, DB outage, duplicate ledgers, incorrect policy, bad webhook, large tag/blob, denial-of-service limits. Coverage thresholds by critical package, explicit threat model and external-security-review checklist. Document resource limits and scaling assumptions; do not call internal review an independent audit.

**Exit gate:** repeatable CI verification and failure corpus; documented results (numbers from measured runs only), no unmitigated critical/high issues under agreed model, threat model updated with remaining risks.

### S9 — Documentation, contributor workflow and repository quality

**Work:** README reflecting real behavior and status (not fake badges), architecture, data provenance, quick start; `CONTRIBUTING.md`, `SECURITY.md`, Apache license, CODEOWNERS (if real reviewers), issue templates, PR template, changelog, support matrix. MkDocs site: problem, how CAP-85 works, capture vs ledger history, system architecture, CLI reference, config reference, API, operator install, security controls, alerts, worked Testnet demo, developer contribution, errors/troubleshooting. Add GitHub topics once authorized. Provide `scripts/create-issues.sh --dry-run` using `gh` and duplicate checks, based on backlog. Recommend, rather than unilaterally enforce, branch protection requiring actual CI job names and approvals.

**Exit gate:** docs build, every CLI example runnable, no fictional contact handles or community links, real demo video or script, contributor can execute `go test ./...` in clean environment.

### S10 — Release candidate / production hardening

**Work:** pin module and reproducible toolchain versions; tagged build pipeline with GoReleaser or equivalent (after review), checksums, provenance, optional SBOM, Linux/macOS/Windows binaries, Docker images when necessary, signed artifacts if owner-controlled signing available. Exercise install/uninstall and upgrade/migration; backup/restore plan for Postgres, on-call alerts, rollback and disaster recovery. Draft `v0.1.0-rc1` notes and changelog. External vulnerability reporting contact must be real.

**Exit gate:** green tagged dry-run/build, exact checksums, smoke test on clean machine, evidence-linked release notes; user explicitly approves any public publication/deployment before Codex acts. `RC READY` ≠ `PRODUCTION CERTIFIED` or audited.

### S11 — Optional Drips Wave application readiness

**Work:** re-fetch https://www.drips.network/wave/stellar/repos and current eligibility rules; confirm repo not already approved or duplicated; inspect comparable approved repos for real patterns. Prepare one-paragraph problem statement, technical differentiator, realistic planned GitHub issues, docs/demo/test URLs, maintainership/community plan, repo relationship ('one off-chain repo; no on-chain contract required'), authentic demo video, and link inventory. If no on-chain contract, **do not invent on-chain addresses**; explain why none are needed. Never submit anything or assert acceptance without verified eligibility and user approval.

**Exit gate:** fully drafted application with only verified links and no placeholders (or clearly identified blockers), ready for owner's review.

### S12 — Post-release and iterative development

**Work:** owner-approved release notes, support cadence, issue triage labels, security disclosure SLA proposed but not promised, vulnerability response, stale SDK/protocol watch, deprecation policy, data-retention strategy, operational metrics review and regression suite for later Stellar protocol changes. Maintainer checklist for each new feature: threat changes, schema change, source proof, docs, tests, issue, PR, release note.

**Exit gate:** published (only if approved) support guide, repeatable onboarding/incident drills, maintainer issue board, first-cycle maintenance report.

## E. Responsibility and permission gates

**Codex may act without another message** on local implementation, tests, docs, working-branch commits and pushes in the verified repo, and PR creation if rights permit. It should work sequentially, stop at reproducible evidence gates, and resume following a blocker fix rather than restarting from zero.

**Requires explicit user approval:** public release/tag publication if it triggers distribution, writes to Mainnet, paid service/database/hosting, real secret creation or upload, repository visibility or org permission changes, merging into protected branch, mass issue creation, security disclosure to outsiders, grant/Wave submission, any asset transfers.

**No available credentials?** Do all local/code-only and simulated Testnet work; document test plan and blocker; don't pretend live evidence exists. **Wrong repo?** Stop all remote writes immediately; request exact URL.

## F. Definition of Done

A legitimate end-to-end release candidate requires: (1) accurate on-chain CAP-85 Testnet proof; (2) deterministic history replay and gaps, including A→B→A; (3) stable monitored alerts; (4) separately verified signed approvals; (5) tests/fuzz/race/security checklist; (6) reproducible build; (7) full docs and operator runbook; (8) actual GitHub PR/CI evidence. If any is missing, call it a staged prototype, not finished. Wave acceptance is a separate external decision, never a release acceptance requirement.

## G. Sequencing and effort assumptions

Plan effort as engineering estimates, NOT promised calendar dates: S0–S1 (2–5 engineer-days), S2 (4–10), S3 (3–8 depending on tooling), S4 (8–15), S5 (5–9), S6 (5–10), S7 (5–10), S8 (5–12), S9 (4–8), S10 (4–8), S11–S12 (2–6 each). One strong engineer can parallelize documentation and fixtures but should not parallelize source correctness before establishing data invariants. The highest schedule risk is sequential, complete ledger ingestion and reproducible CAP-85 Testnet fixtures, not the CLI.

**Update this plan when fresh protocol evidence changes assumptions. No untested or invented SDK APIs may be used merely because the plan mentions their conceptual names.**
