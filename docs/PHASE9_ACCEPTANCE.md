# Phase 9 acceptance evidence

The Dripwave hosting phase ends when the deployed topology actually works
end-to-end on its permanent host. A build, local TLS test or static systemd
verification cannot satisfy that gate. The user's deployment constraint is
**zero cost**: do not purchase resources, accept billable trials or silently
upgrade an account. See [ZERO_COST_DEPLOYMENT.md](ZERO_COST_DEPLOYMENT.md).

The selected architecture is one persistent Linux host, one follower/native
database, a separate local delivery database, optional dispatcher, loopback-only
status/metrics and an explicitly authorized HTTPS receiver. There is no required
frontend or shared database server. The two DB files are not one transaction;
startup queue sync repairs retained findings, while receiver deduplication
handles uncertain delivery acknowledgement.

Record dated evidence for every row on the **chosen permanent host**, with
private credentials, webhook URLs and raw sensitive logs excluded from the repo.

| Gate | Required evidence | Current result |
| --- | --- | --- |
| Zero-cost host | Owner identifies an existing Linux host or confirms a currently eligible always-free allocation with spending disabled; no paid purchase/trial | BLOCKED: no connected permanent host/account |
| Artifact and configuration | Exact source SHA, archive/per-file checksum verification, correct architecture, strict config/original asserted baseline checks | Portable artifacts/checks available; target host UNVERIFIED |
| Disk and retention | Local persistent filesystem, measured space/RAM, reviewed evidence-preserving retention/capacity policy and alarms | Local pilot data only; production policy UNVERIFIED |
| Service operation | One chosen follower unit starts; loopback status, readiness and metrics advance with their assurance fields | Local pilot only; permanent host UNVERIFIED |
| Queue and receiver | Queue sync after controlled finding; explicitly authorized receiver verifies exact HMAC/idempotency; ACK persists on restart | Local TLS fixtures PASS; actual receiver UNVERIFIED |
| Routing and TLS | Required proxy/CA selectors present; allowed receiver path works without disabling policy; if pinned CONNECT is used, actual proxy permits it | Local HTTP/HTTPS proxy fixtures PASS; real path UNVERIFIED |
| Lifecycle | Graceful stop/resume, forced interruption, user logout and real VM reboot retain cursor/findings and restart the reviewed unit | Local interruption tests PASS; host lifecycle UNVERIFIED |
| Backup/recovery | Paired stopped backups, trusted checksums, separate restored copy, exact evidence/ACK comparison and subsequent resume | Local copies PASS; separate host backup location UNVERIFIED |
| Observability and capacity | Alerts for not-ready, increasing lag/gaps, dead letters, retention use and disk pressure; representative workload measurement | Commands and bounded statuses available; target acceptance UNVERIFIED |

The collector below reads local installed preflight/configuration metadata only.
`--running` also GETs fixed loopback readiness/status routes; it never contacts
the configured RPC or receiver and never starts, stops or enables a service:

```sh
/absolute/install/root/bin/phase9-preflight.sh /absolute/install/root > NEW_PHASE9_PREFLIGHT.json
/absolute/install/root/bin/phase9-preflight.sh /absolute/install/root --running > NEW_RUNNING_PREFLIGHT.json
```

It always returns phase9_complete=false and production_validated=false. Exit 0
means evidence was collected, not that Phase 9 passed; exit 1 means observed
requirements failed, exit 3 means invalid invocation. Root/host-check paths must
come from the trusted installation. Keep receipts private and use a new output
path. Human/operator evidence for the table remains necessary.

## Measured small-sample sizing, 2026-10-27

Five fresh CLI processes per operation ran against an isolated copy of the
existing seventeen-ledger captured Testnet pilot using clean candidate ebca09c.
Linux fork/wait4 measured child wall/CPU time and peak resident memory; filesystem
cache was warm. No RPC/receiver call or original DB mutation occurred.

| Operation | Median wall time | Maximum child peak RSS |
| --- | ---: | ---: |
| follow-inspect with complete startup replay | 0.490640 s | 25,024 KiB |
| verify-follow-archive with full replay | 0.349744 s | 23,340 KiB |

The sample retains 5,741,419 logical payload bytes for seventeen ledgers; original
physical DB is 16,777,216 bytes. Observed close spacing is five seconds. Dividing
payload by this tiny sample gives about 337,731 bytes per ledger. At that sample
rate, 64 MiB represents roughly **16.6 minutes**, and 256 MiB roughly **66.2
minutes**. These illustrative ratios include baseline/projection overhead and
do not predict actual capacity-stop times, future traffic or fleet size. They
show why the old pilot budget is not a production retention plan. A larger disk
alone does not raise hard logical limits, and compaction does not remove logical
evidence. Benchmarking this sample does not establish a production SLA or load
capacity. Repeat on the chosen host with the final storage design and a
representative history before marking sizing accepted.

The detailed samples are retained in [the sizing record](research/2026-10-27-phase9-sizing.json).
Local tool/runtime changes may improve these numbers; record final candidate
and exact workload rather than presenting these as a provider guarantee. Archive
and source byte hashes were unchanged; detailed temporary measurements were
reported to the implementation agent for the build record.

## Completion decision

Do not check a row merely because its command exists or a document describes it.
Mark Phase 9 complete only after the permanent-host observations, authorized
receiver receipt and reviewed retention/cost evidence are recorded. Continue
with Dripwave repository hygiene/documentation only after this real hosting gap
is closed, or explicitly state that Phase 9 is blocked. All baseline/source
UNVERIFIED, commitments NOT_VERIFIED, coverage UNKNOWN and final UNDETERMINED
assurance remains; hosting acceptance cannot upgrade chain evidence.
