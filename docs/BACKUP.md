# Local database backup and recovery

`backup` copies every bbolt bucket in one read transaction. Replay observations,
findings, outbox records, delivery leases, approval receipts and nonce reservations
are captured together. Unknown future buckets are retained rather than filtered.
The command does not modify the source or send alerts.

```sh
go build -o fleetwatch ./cmd/fleetwatch
# Stop processes using the database; use a NEW destination in an existing directory.
./fleetwatch backup --state /workspace/replay.db \
  --output /workspace/fleetwatch-backup-001.db > /workspace/backup-001-receipt.json
```

The [JSON receipt](../schema/backup-receipt-v1.schema.json) records `schema_version`, `bytes`, `sha256`,
`storage_format=BBOLT_TRANSACTION_SNAPSHOT` and
`semantic_validation=NOT_PERFORMED`. Exit 0 means the snapshot was saved and its
receipt written. Exit 3 means invalid arguments; exit 5 means unavailable/locked
source, copy/publication failure, or receipt output failure.

The SHA-256 covers the exact **backup** file, not the source's filesystem bytes:
bbolt generates consistent metadata pages and copies the transaction's data pages.
This is not logical compaction, encryption, a signature, or proof of chain
authenticity. The snapshot can include free pages with remnants of historical
records. Treat it and its receipt as private operator data. Keep them outside the
repository and copy them to an owner-managed backup location using the normal
access controls for that location. FleetWatch does not upload backups.

## File safety and bounds

The destination's parent directory must exist and be trusted. The local filesystem
must support hard links, advisory locks and fsync. A mode-0600 temporary file is
created in that directory, filled from bbolt's transaction snapshot, fsynced and
closed. A hard link publishes the complete file without replacing any existing
destination; this also rejects a destination created between validation and
publication. The parent directory is then fsynced. Do not use network filesystems
or object-store mounts without independently verifying these guarantees.

Existing files, directories, symlinks and the source path are never overwritten.
Missing source files are never created. The standalone CLI opens the source read
only and times out after one second waiting for its file lock; stop writable
replay/delivery processes first. Its store API can also snapshot an already-open
database, retaining a consistent read transaction while copying. Long read
transactions can delay writers that need to grow the database.

Snapshots larger than 512 MiB are refused, including bbolt's allocated data pages;
there is no automatic compaction or retention. Cancellation and a two-minute CLI
deadline are checked between writes and before publication. These checks cannot
interrupt a filesystem system call already blocked in the kernel. Ordinary
pre-publication failures remove temporary files; a process or machine crash can
leave a private `.fleetwatch-backup-*` temporary file, which is not a completed
backup. After confirming no backup process is active, operators may remove such
temporary files.

If publication succeeds but directory synchronization or receipt output fails,
the destination is retained. Exit 5 therefore does **not** guarantee the path is
absent. Inspect it before retrying and choose a new destination. The command never
deletes a published backup to hide an uncertain outcome.

## Recovery drill

1. Stop all processes using the original database and preserve it. Never run the
   original and restored histories as independent active writers or dispatchers.
2. Verify the selected backup's byte count and SHA-256 against its securely
   retained receipt. A matching hash checks accidental changes; an attacker who
   can replace both the database and receipt can recompute it.
3. Leave the selected backup immutable. Create a new working copy using the same
   command, for example `fleetwatch backup --state backup-001.db --output recovered.db`.
   This preserves all namespaces together and refuses to overwrite an existing
   recovery path. Store its newly generated receipt separately.
4. Run `doctor --state recovered.db --network EXACT_NETWORK --source SOURCE` and
   the documented `report` queries for each known network/source. Confirm the
   expected cursor, findings and delivery counts. `doctor` checks local replay
   and outbox consistency, not the approval indices. For a known bound event,
   repeat its exact `bind-approval` with the current trusted envelope/root:
   `new_binding=false` and its original receipt confirm that association survived.
   A revoked trust root can correctly reject this diagnostic; do not un-revoke it
   to make recovery pass. There is no automatic full-database semantic validator.
5. Reconcile any observations, bindings and delivery acknowledgments after the
   snapshot. Replay the original journals to check retained observations and
   stable event IDs before resuming work. Do not infer that replay recreates
   approval receipts or delivery acknowledgments.

Old backups cannot know nonce consumption or acknowledgments committed later.
Restoring one can make a later-used nonce appear unused, or make an acknowledged
alert pending again. Local replay prevention applies to the restored history
only. If newer approval records cannot be reconciled, treat the rollback as an
operator incident and resolve trust/release state before accepting new bindings;
do not silently assume global nonce uniqueness. Copied INFLIGHT leases retain
their original deadlines and fencing tokens. Receivers must continue to dedup
stable alert IDs because recovery can repeat delivery.

Tests exercise cold restore of cursor/findings, all four delivery states, retry
metadata, idempotent approval receipts, same- and cross-source nonce rejection,
preservation of unknown namespaces, and an intentionally older history's lack of
later nonce knowledge. Failure injection covers cancellation, byte bounds,
pre-publication aborts and destination races. Local fsync/link behavior is tested;
hardware power-loss durability, external backup storage, production retention,
cross-version migrations and distributed recovery remain unverified.
