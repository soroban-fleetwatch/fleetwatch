# ADR 0004: portable single-host follow pilot

Date: 2026-10-27. Status: accepted for the user-selected portable Linux pilot.
This does not approve a production deployment or replace the roadmap's shared
Postgres design.

Phase 8 proved a controlled Testnet fleet using bounded ingestion. Repeatedly
restarting that worker at the same fixed target cannot continuously advance.
Replacing its asserted baseline every sixteen ledgers would forget inactive
reference preimages and absence knowledge, change provenance and hide history.

Use a separate `follow` runtime/store, one process and local persistent bbolt.
Keep finite source batches (at most sixteen) and existing typed XDR/group reducer,
but carry the complete present/absent reference projection across batches. Bind
event IDs to the original assertion/header rather than the latest batch. Retain
raw records plus applied groups, and atomically commit records, cursor and a
finding journal. Revalidate all retained records from the original assertion on
restart; reject cached-head/journal mismatches. Legacy state and commands remain
separate with their previous lifetime limits and assurance.

Explicit ledger/payload/finding budgets stop before deleting or overwriting
evidence. There is no automatic pruning/archive or invented trusted replacement
baseline. This allows a continuously advancing pilot within reviewed finite
storage capacity; it is not indefinite retention or a production archive design.
Retained payload accounting is distinct from physical bbolt/file/log overhead.

Expose operational status/metrics on loopback only, with readiness qualified by
recent successful endpoint checks, lag and storage capacity. Liveness is separate.
Persist poll/gap-episode metadata; do not emit one durable gap event per retry.
No public API, mandatory frontend, signing key or receiver configuration is added.
Follow's finding journal has no dispatch integration; production delivery is
still a separate gate and existing proxy restrictions are preserved.

Package a static Linux amd64 candidate, checksums, a nonstarting user installer
and a reviewable systemd user unit. The user selected portable preparation rather
than a hosting account. Validate the binary and staged installation locally;
state clearly that target-host boot, systemd support, firewall/CA/proxy setup,
production sizing and end-to-end hosted operation remain unverified.

Tradeoffs: startup replay cost grows with retained history; exclusive locking
requires stopping for offline inspection/backup; finding capacity eventually
stops a pilot. Production work needs reviewed retention/archive, receiver
transport and possibly a transactional shared database. None of this changes
UNVERIFIED baseline/source, NOT_VERIFIED commitments or UNDETERMINED verdicts.
