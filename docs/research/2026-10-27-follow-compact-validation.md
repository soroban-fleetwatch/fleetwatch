# Follow compaction validation — 2026-10-27

This offline check reuses the prior seventeen-ledger Testnet database. No new
chain request, signing key, transaction, receiver call or service start occurred.

`follow-compact --config FILE --output NEW_DB` returned exit 0. It opened the
source read-only, copied all reachable buckets/values/sequences, reopened the
compacted database for complete follow validation and checked the whole-store
logical digest. Physical source size was 16,777,216 bytes; the new compacted file
is 5,799,936 bytes. Its SHA256 is
`c1c8e18852836e32b7c7f06a851c6cc78dbbe04dd27f79912e1f8a4e48c7bf00`.
The logical digest is
`31bfa20457e496b56dc9fdf8110237760252d71ca3684fb5495560357d420cdf`.

`follow-inspect` on a separate configuration for the new copy returned JSON
identical to the original inspection: full head, findings, operational cycle,
pagination and assurance. Baseline remains 5413741, processed through 5413758,
retained ledgers seventeen and findings zero. The original database SHA256 is
unchanged. Exporting a fresh evidence archive from the compacted copy produced
bytes identical to the previously validated archive. Raw files remain ignored
under follow-runs/compact-validation-1; the matching JSON report records exact
receipts and comparisons.

Synthetic tests preserve two upgrade/rollback findings, full projections,
operational history, extra buckets, empty buckets/values, binary values and bucket
sequences. Original and compacted stores continue identically from 24 through
forty ledgers. Copy transactions targeting 1, 1,024 and 16,777,216 bytes produce
the same logical digest. Tests reject corrupt head state, excessive nesting,
missing source, cancellation, prepublication failure and existing/symlink/racing
destinations, preserving source bytes and cleaning ordinary failed staging.

Full tests (233 top-level tests, six fuzz seed targets, four intentional skips),
affected follow/CLI race suites, vet, module verification/tidy-diff, module/
source-test/binary advisory gates, workflow lint and package checks PASS locally.
Remote CI remains unverified. This sample is not a throughput/memory benchmark,
crash/boot durability certification or hosted deployment.

Compaction preserves logical payload/ledger/finding counts and cannot resolve
a logical-capacity stop. No source was deleted, live path switched, limit raised,
baseline reset or finding acknowledged. Future writes may grow file allocation
again. Archive chaining/retention rollover and production operations remain open;
all source/baseline/commitment/coverage/verdict uncertainty is unchanged.
