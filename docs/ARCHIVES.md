# Follow evidence archives

The experimental follow store can export a portable, compressed evidence file
and verify it without the original database, configuration or RPC endpoint.
This is a prerequisite for retention work. It does **not** prune the live store,
rotate its baseline, acknowledge findings or extend its configured capacity.
Keep the database and a consistent database backup for restart and recovery.

Stop the follower, then use a new destination on suitable local storage:

```sh
fleetwatch follow-archive --config follow.json --output NEW_EVIDENCE.ndjson.gz
fleetwatch verify-follow-archive --input NEW_EVIDENCE.ndjson.gz
```

Both commands print a JSON receipt containing the compressed archive SHA256,
content SHA256, original baseline digest/header, baseline and final ledger,
retained-ledger count and reconstructed-finding count. Export fully validates
the existing store through an OS read-only handle, writes a private temporary file, fsyncs it, independently
replays that file, then publishes it using a non-overwriting hard link and
fsyncs the containing directory. The destination directory must already exist
on a filesystem supporting these operations. The source database is unchanged.
An existing file or symlink is rejected, including a destination created during
export. Failure before publication removes the temporary file. A failure after
publication leaves the destination available for inspection; do not assume an
error means no archive exists.

Verification checks ordered raw evidence and derived records by replaying from
the **original operator assertion**. It compares the resulting full projection,
retained payload accounting and findings count with the final head. It derives
the finding payloads from the accepted events with their original provenance;
it does not trust a cached final projection as a replacement baseline. Missing,
duplicate, reordered or corrupt ledgers, modified derived events, mismatched
heads/digests, malformed JSON and truncated gzip input fail verification.

The file includes the baseline/watchlist, raw headers/metadata and all retained
applied groups/events. It contains no RPC configuration or signing key. Treat
watched identities and source evidence as potentially private; filesystem mode
0600 is not encryption. SHA256 detects changes relative to a trusted receipt;
it is not a signature, proof of chain inclusion or proof of producer identity.
Preserve the receipt through a trusted channel when moving a file to another
machine. No upload or external delivery is performed.

## Recover a separate database

Use the compressed archive SHA256 from your trusted export receipt, and a new
database path in an existing local directory. No original configuration, source
database or RPC endpoint is required for this offline command:

```sh
(
  umask 077
  set -C
  fleetwatch restore-follow-archive --input EVIDENCE.ndjson.gz \
    --output NEW_RECOVERED.db --sha256 EXPECTED_ARCHIVE_SHA256 \
    > NEW_RESTORE_RECEIPT.json
)
```

Recovery streams verified records into a private staging database with the
original assertion and original finding identity. It verifies the complete
archive, gzip integrity and expected compressed-file digest, closes the staged
database, reopens it for full retained-evidence/journal validation, and fsyncs
the file before atomically publishing it without replacing any existing file or
symlink. A digest mismatch, incomplete archive, cancellation or validation error
prevents publication and cleans staging on ordinary error returns. A killed
process or host crash may leave a private `.fleetwatch-restore-*` directory;
inspect it while stopped before removing it. A failure after publication leaves
the destination available for inspection. Do not assume an error means no file
exists, especially if only receipt output or directory fsync failed.

The JSON restore receipt includes the archive validation receipt, the exact
original `baseline`, the restored file's SHA256, and
`operational_metadata: RESET_NOT_ARCHIVED`. Keep it private. To save the original
assertion for a separately reviewed configuration without overwriting a file:

```sh
python3 - <<'PY'
import json, os
os.umask(0o077)
with open('NEW_RESTORE_RECEIPT.json') as source:
    receipt = json.load(source)
with open('NEW_RECOVERED_BASELINE.json', 'x') as destination:
    json.dump(receipt['baseline'], destination, indent=2)
    destination.write('\n')
PY
```

Create a separate follow configuration pointing at that original baseline and
new database; review its network/RPC and retained-data budgets before running
`follow-inspect`, `follow --check`, or starting a follower. No service is started
by restore. The recovered store has the **same** retained evidence and finding
count; it does not free capacity. There is no automatic replacement of live
state or automatic resume. Catch-up still needs the subsequent source ledgers.

Poll counts, gap-episode history, last-success time and endpoint high-water
claims were not archived and are reset, with a fresh `STARTING` cycle. A zero gap
counter after recovery does not mean there were no earlier gaps. Retain original
operational evidence separately. New-process readiness remains false until its
own successful source check. Findings are reconstructed, never acknowledged or
marked delivered. The original assertion stays unverified.

Restore has a ten-minute overall deadline, existing logical retention bounds
and a 512-MiB final physical-database cap. Temporary disk usage and replay cost
still need host sizing. Exit 0 means a new database and receipt were produced,
3 means invalid arguments or input-file requirements, and 5 means rejected
archive/digest, recovery, storage or output failure. The expected checksum binds
bytes to the supplied receipt; it does not authenticate that receipt's author.

## Version 1 format and bounds

The format is gzip-compressed canonical NDJSON. Each line is one compact Go
JSON encoding followed by a newline, with exactly one envelope member:

1. `header`: schema_version=1, kind=`fleetwatch_follow_evidence_archive`, and
   the original `baseline` assertion.
2. Zero or more `record` lines, each containing the retained `evidence` and
   `applied` ledger record, in strictly contiguous ledger order.
3. `trailer`: the final `head` and `content_sha256`, the lowercase SHA256 of
   every preceding expanded line including its terminating newline.

The receipt's `archive_sha256` covers the complete compressed file, including
its trailer. Unknown, duplicate, case-aliased, noncanonical or trailing JSON is
rejected. The verifier reads through gzip end-of-stream, checking its integrity
trailer. Neither compressed nor expanded input may exceed 512 MiB. Each line
is bounded by the existing 25-MiB JSON limit; each retained record is at most
12 MiB. Existing follow limits of 100,000 ledgers, 256 MiB logical payload and
10,000 findings apply independently. Input is streamed one record at a time;
the current projection/reducer and individual decoded record still use memory.
This is not a measured peak-memory or throughput guarantee.

The commands handle SIGINT/SIGTERM and have a ten-minute overall deadline.
Export's initial store validation retains the existing two-minute deadline.
Use exit 0 for a successful export/verification, 3 for invalid command/config
or a missing/nonregular/oversized verification file, 4 for rejected archive
content, and 5 for state/export/output failures. Errors omit raw private source
details. Inspect the destination if export or receipt output fails.

## What verification does not establish

The receipt explicitly reports offline=true,
semantic_validation=`REPLAYED_FROM_ORIGINAL_ASSERTION`, baseline/source
authentication `UNVERIFIED`, metadata commitment `NOT_VERIFIED`, membership and
history coverage `UNKNOWN`, and final verdict `UNDETERMINED`.

The archive is evidence for independent local replay, not a database snapshot.
It excludes operational poll/gap counters and does not contain delivery receipts
or acknowledgements. Recovery rebuilds a separate database; there is no automatic
rollover command. Do not delete the source database after exporting it, relabel
an archived tip as an authenticated baseline, or claim continuous retention has
been solved. Production archive chaining, deletion policy, lifecycle
reconciliation and hosted validation remain separate work.
