# Follow evidence archive validation — 2026-10-27

This check reuses the previously captured read-only Testnet pilot. It makes no
new chain request, transaction or external receiver call. Source evidence is
still endpoint-supplied and unverified. It does not validate production hosting.

The original operator-asserted baseline is ledger 5413741. The database retains
seventeen consecutive ledgers through 5413758, with no derived finding in this
small interval. `follow-archive` exported a gzip NDJSON evidence file;
`verify-follow-archive` independently replayed it without config, database or
source access. Both commands returned exit 0 with identical JSON receipts.

- Compressed size: 890,752 bytes.
- Expanded size: 5,741,908 bytes.
- Archive SHA256: `260a7aa2a0b8521300004f1ee2376f0f984bf546e4d30696e4fa827bd3f1bee4`.
- Content SHA256: `1ea9ef6df67130988c143378acc4bba4c8fde777661b17957a678669faa66d13`.
- Source database SHA256 is unchanged before/after export and verification;
  its exact value and the receipt are in the matching JSON evidence file.

Raw files remain ignored under `follow-runs/archive-validation-1`. This is one
small historical sample, not a compression/throughput or resource-use benchmark.
The original capture's provenance and qualification are recorded in
[the follow pilot report](2026-10-27-follow-pilot.md).

Deterministic synthetic tests separately retain forty ledgers with A→B and B→A,
reconstruct two findings with original event identity, and produce identical
repeated archive bytes/receipts. Fault tests reject missing, duplicate, reordered
and malformed evidence, altered applied events, changed baseline/header, bad
head or digest, duplicate/unknown JSON members, missing line/trailer and damaged
gzip. Mutated records remain rejected after recomputing the content checksum.
Tests also cover independent compressed/expanded limits, cancellation,
publication failure, racing/existing/symlink destinations, temporary cleanup,
read-only state opening and refusal to create missing state or write through
the read-only handle. The CLI test exercises export, independent verification,
argument errors and unchanged source bytes. A ten-second, two-worker verifier
fuzz exercise passed; this is a bounded local check, not an audit.

All receipts retain baseline/source UNVERIFIED, metadata commitment NOT_VERIFIED,
membership/history UNKNOWN and final UNDETERMINED. They establish local replay
consistency only. No pruning, archive chaining, archive restore, baseline
replacement, delivery acknowledgement or production retention was performed.
