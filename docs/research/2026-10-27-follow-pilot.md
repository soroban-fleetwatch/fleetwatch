# Phase 9 portable Linux follow pilot — 2026-10-27

The user selected **prepare a portable Linux deployment** rather than supplying
a hosting account. This checkpoint provides a tested local candidate, not a
provisioned production host or a completed Dripwave production-hosting gate.
See [operations](../OPERATIONS.md), [runtime behavior](../FOLLOW.md) and
[ADR 0004](../adr/0004-portable-single-host-follow.md).

## Read-only Testnet execution

The pilot used the already deployed Phase 8 watchlist, with a fresh explicitly
asserted current-state baseline and retrieved header pin. The underlying
multi-call capture stayed diagnostic/PARTIAL. No signing identity was created;
no transaction or external notification was sent.

- Original asserted baseline: ledger 5413741.
- One process continuously retained seventeen new ledgers through 5413758,
  crossing the legacy worker's sixteen-ledger lifetime limit.
- Operational readiness was true; `/readyz` and `/status` returned 200 and the
  Prometheus text contained `fleetwatch_follow_ready 1`.
- Retained payload was 5,741,419 bytes; finding count zero; source gap episodes
  zero during this run. These are observations of this small read-only run,
  not a load benchmark, throughput target or safety verdict.
- SIGTERM returned 0. A new process revalidated retained XDR from the original
  baseline, became ready, preserved cursor 5413758 and zero finding count, and
  stopped cleanly. The durable poll counter advanced from 67 to 68.
- A consistent physical backup restored into a separate file; full replay and
  offline inspection exactly matched the original. The offline report explicitly
  retained unverified/undetermined assurance. No live process was left running.

[Machine-readable evidence](2026-10-27-follow-pilot.json) contains both actual
status reports and the backup receipt. Raw captures, databases and temporary
runtime files remain ignored under `follow-runs/live-1`.

## Deterministic failure and continuity checks

Synthetic typed XDR tests exercise forty sequential ledgers with A→B at 100 and
B→A at 117. The same findings/IDs survive batch sizes 1, 8 and 16 and cold replay;
the final snapshot alone would miss the transient change. Origin provenance and
inactive/absent reference knowledge carry across batches. Separate tests reject
conflicting inactive/absent preimages rather than treating each batch as a new
current-state assertion.

Capacity and injected transaction failures roll back evidence, findings and
cursor together. A subprocess exits before commit and the parent proves a cold
reopen has no uncommitted ledger/finding. Truncated/foreign state, corrupt
journal data, second writers, wrong networks, stale/regressing heads and broken
cross-batch header links are rejected. Source gaps retry with a durable gap
episode count; successful repair becomes ready, while stale status becomes
not-ready with unknown lag. Cancellation and output failure stop the loop.

## Portable candidate checks

The Linux amd64 candidate is built with pinned Go, CGO disabled and trimmed
paths. It includes checksums, source commit/dirty metadata, the installer, a
systemd user-unit template and operator guides. Precommit candidates correctly
reported a dirty source tree; a clean-source candidate is built after committing.
No tag, GitHub release, image push, paid resource or permanent service was created.

Installation into a fresh `/tmp` root passed. It validated config/baseline offline
and did not create/open a state DB or start a process. Existing-directory,
tampered-package and invalid-baseline checks passed without overwriting a prior
installation. Static systemd user-unit verification passed with an isolated
writable XDG_RUNTIME_DIR. The first verifier invocation lacked that directory;
supplying a test runtime directory fixed the **static check**, not a missing
running manager. PID 1 in this cloud environment is not systemd, so target-host
unit enable/start, boot persistence and hardening availability remain unverified.

The new follow store is a distinct format. The existing backup command now has
an explicit copy-only reader for recognized follow/replay roots; legacy health
and replay readers still refuse follow databases. Backup/restore plus bounded
finding pagination are tested. Follow findings are retained for inspection;
they are not silently routed through the legacy webhook outbox.

## Remaining gate

All baseline/source authentication remains UNVERIFIED; metadata commitments
NOT_VERIFIED, membership/history UNKNOWN and final verdict UNDETERMINED. One
endpoint's fresh head/lag is not independently authenticated network health.
No source-trust or signed-approval claim was added.

The continuously advancing pilot still stops at explicit retained-ledger,
logical-byte or finding limits. There is no automatic archive/pruning/migration,
shared Postgres backend or follow-journal notification dispatcher. Production
receiver routing and sizing, independent chain evidence, permanent Linux host
deployment and hosted end-to-end validation remain Phase 9 work. Portable
preparation is ready; production hosting is not marked complete.
