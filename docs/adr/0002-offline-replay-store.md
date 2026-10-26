# ADR 0002: bounded local replay persistence

Date: 2026-10-26. Status: accepted for offline development only.

Use bbolt v1.4.3 for the optional local `replay --state FILE` path. This provides
synchronous atomic transactions and exclusive file locking without running a
server. It does not replace the roadmap's production Postgres/ledger-meta design.

A scope key hashes the exact network, source kind, and domain version. Each
transaction saves the contiguous observation prefix, deterministic findings and
processed-through cursor together. Restart extensions merge with stored evidence;
old matching observations are idempotent, conflicting observations abort without
changing state. Events and cursor cannot commit separately. Failed or interrupted
transactions preserve the preceding state. NoSync and other unsafe durability
options are not enabled.

Retain at most 1,000 per-ledger snapshots, 10,000 derived events and 25 MiB of
encoded state per scope. These explicit prototype bounds make recomputation and
validation practical; this is not an unbounded event store. Source authenticity
and history completeness remain unverified. No outbox or service coordination is
implemented. Production migration requires a distinct design and acceptance gate.

Tests exercise cold reopen, segment resume, repeated replay, gaps, conflicts,
network/source separation, exclusive file locking, rollback after a write hook,
subprocess exit before commit, malformed/foreign paths and altered findings.
Application tests demonstrate process-crash recovery, not arbitrary power-loss
or filesystem corruption guarantees.
