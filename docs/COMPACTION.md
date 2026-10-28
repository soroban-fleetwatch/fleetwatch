# Offline follow database compaction

`follow-compact` copies a stopped follow database into a new file, preserving
retained evidence, original assertion, findings and operational history while
reclaiming unused bbolt pages/allocation. It does not prune history, rotate the
baseline, change logical retention budgets or replace the live database.

```sh
fleetwatch follow-compact --config follow.json --output NEW_COMPACTED.db
```

The source is opened read-only and fully replay-validated. The command copies
every reachable bucket, key, value and bucket sequence, including extra buckets
and empty buckets/values. It checks a deterministic logical digest of the whole
copy and reopens the new database for complete follow evidence/journal/cycle
validation before publication. Future unknown data are preserved byte-for-byte;
matching their bytes does not validate their application-specific meaning.

The source is unchanged. Existing destinations, including directories and
symlinks, are refused; a destination created during copying is not overwritten.
The copy is staged privately in the destination directory, file-synced, published
with a non-overwriting hard link and followed by directory fsync. A failure
before publication cleans ordinary staging returns. A killed process may leave
a `.fleetwatch-compact-*` directory for stopped operator inspection. A failure
after publication leaves the destination in place, including receipt-output or
directory-sync failures. Inspect before retrying. Local filesystem locking,
hard-link and fsync semantics are required; network filesystems are unsupported.

The JSON receipt includes original and compacted physical byte counts, the
new file's SHA256, logical SHA256, ledger/finding counts, and
`operational_history: PRESERVED`. Unlike archive recovery, compaction preserves
poll/gap counters, timestamps and endpoint high-water claims. A fresh follower
still starts not-ready until its own source check succeeds. No findings are
acknowledged, delivered or cleared, and no service is started.

Keep the original and a verified backup. Prepare a separate configuration
pointing at the compacted database with the same original baseline, run
`follow-inspect`, compare head/findings/cycle, and review before choosing which
copy to run. Do not run both copies as independent workers. The command never
edits configuration or automatically switches state paths.

Compaction can help physical disk pressure or a backup limited by physical file
size. It cannot resolve a **logical** payload, ledger or finding-capacity stop.
Receipt counts and logical payload remain identical, so capacity checks still
apply. Savings depend on the source's page allocation and are not guaranteed;
future writes can grow the new file again. The retained seventeen-ledger Testnet
sample shrank from 16,777,216 to 5,799,936 bytes with identical inspection output.
That small offline check is not a sizing or throughput benchmark.

## Bounds and validation digest

The command has a ten-minute cancellable deadline, a final physical-file cap of
512 MiB, and traversal limits of 64 bucket levels, one million bucket/value
entries and 512 MiB of reachable key/value bytes. Existing follow limits apply
independently during replay. Destination transactions target 16 MiB of copied
key/value bytes and do not split a single value; this is not a peak-memory bound
or a disk reservation. Cancellation checks between entries cannot interrupt a
blocked kernel filesystem call. The host needs temporary space for both files.

The logical SHA256 starts with UTF-8 `fleetwatch.bbolt.logical.v1\n` and walks
bbolt key order recursively. A bucket contributes byte `B`, its name prefixed
by an unsigned 64-bit big-endian length, its unsigned 64-bit big-endian sequence,
its children, then byte `E`. A value contributes byte `V`, its length-prefixed
key and length-prefixed value. Empty values remain distinct from empty buckets.
This detects mismatched copies; it is not a signature or chain proof.

Exit 0 means publication and receipt output succeeded, 3 means invalid
command/config/baseline, and 5 means source, copy, validation, capacity or output
failure. Source/baseline authentication remains UNVERIFIED, commitments remain
NOT_VERIFIED, membership/history UNKNOWN and final verdict UNDETERMINED.
Production retention rollover, archive chaining and hosted operation remain open.
