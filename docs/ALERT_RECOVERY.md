# Manual dead-letter recovery

`requeue` returns one failed alert to PENDING without sending it. It supports
snapshot sources (`synthetic`, `user_snapshot`) and reconstruction streams
(`rpc_ledger_meta` plus `--stream ID`). Use the exact network and source used to
create the alert. Stop other commands using this local DB before maintenance.

Inspect `alerts` and correct the delivery configuration first. `DEAD` alerts are
eligible, as are INFLIGHT alerts whose fifth attempt lease has expired. PENDING,
DELIVERED and live INFLIGHT alerts are refused. An absent `recovery_cycle` means 0.

```sh
fleetwatch alerts --state replay.db --network TESTNET-SYNTHETIC
fleetwatch requeue --state replay.db --network TESTNET-SYNTHETIC \
  --alert-id ALERT_ID --request-id ops-20261026-001 --expected-cycle 0 \
  --reason 'Receiver configuration corrected'
fleetwatch recoveries --state replay.db --network TESTNET-SYNTHETIC \
  --alert-id ALERT_ID
```

For reconstruction, add `--source rpc_ledger_meta --stream ID` to each command.
`recoveries` opens the existing database read-only, validates retained evidence
and returns the selected alert's receipts in recovery-cycle order. Snapshot
findings are checked against replayed observations; reconstruction queues are
rederived from their retained raw evidence. Historical snapshot gap diagnostics
retain their deterministic issue identity even after repair.

Supply a fresh request ID (1–128 ASCII letters, digits, `.`, `_`, `-`) and a
nonblank reason (at most 512 UTF-8 bytes, no control characters). Do not put
credentials in the reason: it is retained and printed. The reason records an
operator statement; it does not authenticate an operator identity.

The receipt and alert update commit in one bbolt transaction. Each recovery
increments `recovery_cycle`, resets the current cycle's attempts to 0, clears
backoff and lease state, and preserves ID, payload and bound sink. The receipt
retains the prior attempt count, status, fixed error code, expired lease deadline
when applicable, sink, payload SHA-256, reason and UTC recovery time. Lease tokens
are never exposed. Old workers cannot acknowledge the requeued alert.

The expected cycle must match the stored cycle. Concurrent requests with the same
expected cycle cannot both recover it. Repeating the exact request ID and
parameters returns its original receipt with `new_recovery:false`, even after
subsequent delivery. Changed parameters under that ID are refused. If output
fails after commit, retry the identical command to retrieve its receipt without
resetting attempts again. Exit 0 means a receipt was printed; 3 means missing or
invalid CLI arguments; 5 means state, recovery validation or output failure.

Each cycle allows at most five delivery attempts. At most ten manual recoveries
are allowed per alert, with at most 10,000 receipts per network/source/stream
queue and 4 KiB per receipt. There is no pruning, bulk retry, sink reassignment or
force option. Missing or inconsistent audit history blocks further recovery.
The audit and hashes check local consistency; a party able to rewrite the whole
DB can rewrite them. Preserve the entire DB when backing up. Restoring an older
backup can forget later attempts, recoveries and acknowledgments; reconcile
those externally before sending again. Older binaries may reject records with
the new `recovery_cycle` field; automatic downgrade is not supported.

Delivery remains at least once. The receiver may have accepted a previous
attempt whose acknowledgment was lost, so it must deduplicate the unchanged
alert ID. Recovery does not change HMAC payloads, source assurance, approval
status or chain verdicts. It does not resolve historical diagnostics or establish
current source health. See [webhook constraints](WEBHOOKS.md) before delivery;
production proxy-backed delivery and a continuous worker remain unfinished.

See [result/receipt schema](../schema/alert-recovery-v1.schema.json). Runtime also
enforces identity, scope, audit sequence and state-transition constraints.


For reconstruction streams, `doctor --source rpc_ledger_meta --stream ID`
validates all recovery sequences together with the checkpoint and outbox and
prints aggregate counts. It rejects receipts for missing alerts. Use `recoveries`
for individual operator reasons and request IDs; requeue also returns its receipt.
Snapshot-mode doctor continues to inspect replay/outbox consistency without
validating recovery indices.
