# Follow archive recovery validation — 2026-10-27

This is an offline recovery check using the evidence archive captured in the
[archive validation report](2026-10-27-follow-archive-validation.md). No new RPC
request, signing key, transaction, receiver call or hosted service was involved.

`restore-follow-archive` used the previously recorded compressed-file SHA256
`260a7aa2a0b8521300004f1ee2376f0f984bf546e4d30696e4fa827bd3f1bee4`
and a new database destination. It returned exit 0. Its receipt's original
baseline was written to a separate file and used by a separate follow config.
`follow-inspect` then reopened and fully validated that recovered database.

The recovered head and finding payloads match the original inspection exactly:
original assertion ledger 5413741, seventeen retained ledgers through 5413758,
zero findings in this sample. The recovered bbolt file is 16,777,216 bytes, with
its exact SHA256 in the matching JSON report. The original database's SHA256
remains unchanged. Raw files stay ignored under follow-runs/restore-validation-1.

As expected, the recovered operational cycle is STARTING, with no prior polls,
gap episodes, success time or endpoint high-water claim. This is an explicit
loss of operational history from the evidence-only archive, not evidence that
the original process had no gaps. No follower was started and no readiness claim
is made. Preserve operational backups and logs separately.

Synthetic tests separately recover both an assertion-only archive and a
24-ledger archive, compare full projections and original-ID finding payloads,
then continue the original and recovered databases through forty ledgers with
identical results and two upgrade/rollback findings. Tests reject wrong hashes,
damaged/truncated input (even when its supplied file checksum matches), missing
digest arguments, existing/symlink/racing destinations and interrupted
publication. Ordinary failures leave no published state or staging residue.
The recovered file has mode 0600; fresh health remains not-ready.

Local full tests, affected follow/CLI race tests, vet, module verification,
tidy-diff, vulnerability checks, workflow lint and package checks passed. Remote
CI remains unverified. This is not a crash/boot durability certification or
production retention benchmark. Restore preserves all retained rows and does
not free capacity, replace the baseline, prune history or acknowledge findings.
All original UNVERIFIED/NOT_VERIFIED/UNKNOWN/UNDETERMINED assurance remains.
