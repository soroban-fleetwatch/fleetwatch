# Operations, publication, and handoff controls

## Runtime expectations

- `fleetwatch watch` is a long-running read-only process with least-privilege network access to Stellar RPC / sequential ledger source, a database, and configured alert destinations. No wallet/private keys are required.
- Config: network passphrase, ledger source identity/URL, watchlist, DB URL from environment, webhook secret from environment, approved HTTPS targets, log level, backfill start sequence, page/batch size. Ship `.env.example` without credentials and an annotated YAML config example.
- Track ledger-gap markers and exact resume cursor. Stop claiming coverage while lagging past retention. On restart, reconcile pending outbox messages before advancing.
- All data source outages should surface in CLI/API health and alerts (not silent green). Maintain data-integrity checks to distinguish missing observation from deleted ledger entry.

## Operational runbook (deliver exact commands after implementation)

1. Install signed/checksummed release, verify version and network compatibility.
2. Start PostgreSQL/secured datastore, apply migrations in a rollback-reviewed manner.
3. Run `fleetwatch doctor` with Testnet settings, confirm network passphrase and source range.
4. Capture baseline from explicit watchlist; verify completeness and evidence.
5. Start historical replay from source-supportable sequence and wait until current.
6. Enable watch; inspect metrics (last ledger, gap count, lag, outbox queue).
7. Trigger a controlled Testnet upgrade; verify correct blast radius and exact one alert.
8. Induce temporary RPC outage and DB restart; verify backoff, gap/health incident, resumed cursor, replay idempotence.
9. Backup store, restore in isolated environment, and replay to known snapshot hash.
10. Upgrade/downgrade a nonproduction version with schema compatibility checks.

## Incident classes and response

- `UNAPPROVED_CODE_CHANGE` — immediately verify chain evidence and signer policy, do not automatically accuse administrator or execute mitigation.
- `OBSERVATION_GAP` — restore source, reconcile exact missing range via ledger meta, suppress clear verdict until resolved.
- `SOURCE_DIVERGENCE` — compare independent same-height sources, quarantine alerts requiring certainty and escalate to operators.
- `WEBHOOK_FAILED` — retry via outbox, dead-letter with operator visibility, avoid infinite burst.
- `POLICY_SIGNATURE_INVALID` — reject approval as untrusted, inspect trust store and key rotation, do not degrade to unsigned approval implicitly.

## Release checklist

- Verified default repo and main branch, correct Apache-2.0 licensing, third-party license notices.
- Go SDK and protocol range documented and pinned, with clean test/scan/build on CI.
- Every user-facing status definition documented and CLI output proved from fixtures.
- Data migrations tested, exact DB backup/restore commands documented and executed.
- Changelog, installation, README, vulnerability contact, maintenance policy, schema support and docs site build.
- Versioned artifacts tested across supported OS/arch, SHA-256 checksums, optional signed provenance and SBOM.
- Actual Testnet demonstration and ledgers/transaction evidence recorded; if blocked, RC stays pending.
- Final review includes unsolved risks and explicit external audit disclaimer.
- Owner approves publication (release/tag if public, hosting, org settings, Wave submission) separately.

## Drips Wave readiness (not actual submission)

Recheck current Drips Wave eligibility and approved repos when preparing. Provide one-paragraph problem/approach statement, exact actual GitHub repo URL, docs site URL (if deployed), demo video, real Testnet proof and planned real issues. On-chain explorer links only if fixture contracts really deployed. Do not make up deployment IDs, program acceptance, grant amount, contributor allocation, or engagement metrics. Wave is optional, independent of the security readiness bar.

## End-of-session report template

```text
Working repo: <actual verified owner/name>
Working branch: <name>; head SHA: <real>
PRs: <real URLs or NOT OPENED>
Stages: S0 PASS, S1 PASS, S2 IN PROGRESS ... (provide evidence)
Tests run: <command> — <real result>
Live Testnet status: <real tx/ledger, or NOT VERIFIED>
Release status: NOT PUBLISHED / DRAFT / PUBLISHED (actual)
Unresolved risks: <specific>
External approvals needed: <specific>
Next unblocked task: <exact>
```

A green local test is not a green CI run; a signed approval is not proof of secure code; publishing a docs site is not product readiness; applying for Wave is not Wave approval.
