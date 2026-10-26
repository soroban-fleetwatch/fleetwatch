# FleetWatch agent instructions (Codex / any engineering agent)

Read `docs/build/END_TO_END_BUILD_PLAN.md`, `docs/build/TECHNICAL_SPEC.md`, `docs/build/ACCEPTANCE_MATRIX.md`, `docs/build/ISSUE_BACKLOG.md`, and `docs/build/STATUS.md` before coding. Work from verified GitHub origin and current checked-out branch. This is a standalone off-chain CAP-85 executable-reference monitor written primarily in Go, not SoroIndex and not the Protocol 30 scanner.

## Required approach

1. Examine actual repository/files/CI, research live authoritative CAP-85 and Go/XDR APIs, and record current sources. Never invent SDK functions or chain observations.
2. Fix root correctness problems first (including module path/Apache-2.0 provenance), then advance one tested milestone at a time. Preserve existing user work.
3. Source adapters must be separate from pure comparisons. Tag bytes must be preserved exactly. Signed approval must never be conflated with unsigned policy match. Current-state RPC is not complete history; partial ledger reads cannot yield clean verdicts.
4. No Mainnet writes or movement of assets. Testnet only for controlled deployments; offline deterministic fixtures for CI. No private keys/tokens committed. No auto-upgrade or auto-containment.
5. Use conventional atomic commits and specific-path staging (`git add path`) when committing is authorized. Pushes and PR creation require user authorization; uploaded documents alone do not grant it. Never force-push or merge without permission. Block paid resources, secrets, org permissions, public release and Wave submission behind explicit approval.
6. Every milestone updates `docs/build/STATUS.md` with actual commands, results, source URLs, commit hashes, live evidence if any, blockers and next steps. Do not claim success based only on implementation drafts.
7. Keep README/security docs, tests, runbooks, and non-fictional demo outputs aligned. Never advertise an independent audit unless one actually happened.

Full execution task: see the user-provided `CODEX_MASTER_PROMPT.md` in the handoff package; do not republish user-uploaded Dripwave skill without permission to redistribute.
