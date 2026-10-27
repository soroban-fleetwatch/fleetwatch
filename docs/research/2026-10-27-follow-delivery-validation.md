# Follow delivery bridge validation — 2026-10-27

The new offline bridge connects replay-validated follow findings to a separate
durable delivery outbox. This report covers local validation, not an externally
hosted notification service. No new chain transaction, RPC request or external
receiver call was performed.

The local TLS test reused the previously captured controlled Testnet transition
baseline at 5407791 and twelve raw ledger records through 5407803. A fresh native
follow store applied that history and produced three findings. The test imported
them, dispatched to a fixed loopback TLS receiver, checked exact-body HMAC,
idempotency keys, schema-3 identity and unverified assurance, then reopened and
resynced the outbox. Received requests: three. Reopened dispatch attempts: zero.
A payload with forged APPROVED assurance was refused before another request.
The production public-host/DNS-pinning/proxy restrictions remain unchanged;
loopback routing and the explicit test CA use the existing isolated test seams.

The test invocation set `FLEETWATCH_FOLLOW_TEST_BASELINE` and
`FLEETWATCH_FOLLOW_TEST_CAPTURE` to the existing transition-baseline.json and
transition-history.json under testnet-runs/live-validation-2, with
`FLEETWATCH_FOLLOW_DELIVERY_REPORT` under follow-runs/delivery-validation-1,
then ran `go test -race -count=1 ./internal/alert -run
'^TestFollowDeliverySignsPayloadAndRetainsAcknowledgements$'`. Without those
optional inputs, the same test uses deterministic synthetic fixtures. It never
targets an external receiver. The report is created privately without overwrite.

Separately, the CLI synced and resynced the existing seventeen-ledger pilot
through 5413758 into a separate outbox. Both receipts match with added=0 and
retained=0, as that historical interval has no finding. The original native DB
SHA256 is unchanged. Zero imported findings is not an all-clear verdict.

Synthetic tests cover original event identity and two findings across forty
ledgers, preservation of acknowledged and dead-letter state, expired/stale lease
fencing, audited recovery, cold reopen, all-bucket backup, source-path collision,
rewound source, atomic rollback, omitted findings, conflicting baseline/head,
stale import binding, forged authorization and missing queue/manifest rows.
Each sync scans the complete hash-ordered native journal, so a new hash need not
sort after the last imported ID. No native journal finding is cleared.

Full default tests (239 top-level tests, six fuzz seed targets, four intentional
skips), affected store/alert/follow/CLI race suites, vet, module checks, pinned
module/source-test/binary vulnerability gates, workflow lint, schema syntax and
package checks PASS locally. Remote CI remains unverified. This is not a load
test or audit. Manifests retain event claims, not raw source history; native
replay is required at import. All chain/baseline/commitment/coverage uncertainty
remains and delivery HMAC is not upgrade authorization.

Unattended sync scheduling, gap-diagnostic notifications, production receiver/
proxy routing, archive chaining/retention rollover and permanent-host validation
remain open. The follower must be stopped for this manual offline sync step.
