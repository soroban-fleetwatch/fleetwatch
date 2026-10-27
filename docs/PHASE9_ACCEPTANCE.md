# Phase 9 acceptance evidence

The Dripwave hosting phase ends when the deployed topology actually works
end-to-end on its permanent host. A build, local TLS test or static systemd
verification cannot satisfy that gate. The user's deployment constraint is
**zero cost**: do not purchase resources, accept billable trials or silently
upgrade an account. See [ZERO_COST_DEPLOYMENT.md](ZERO_COST_DEPLOYMENT.md).

## Intended service topology

The native collector is the deployment target. A frontend is not required by the
build plan. GitHub Pages is unavailable on the owner's current plan; it is not a
production dependency. The browser viewer is a separate foreground
current-state diagnostic, not a substitute for durable unattended monitoring.
[Environment repair](research/2026-10-27-environment-repair.md) and production
service acceptance are distinct: saving/publishing a development environment does
not deploy its processes as a permanent hosted service.

The native architecture is one persistent Linux execution target, one follower/native
database and loopback status/metrics. Optional notification delivery adds a
separate local delivery DB, dispatcher and authorized HTTPS receiver. There is no required
frontend or shared database server. The two DB files are not one transaction;
startup queue sync repairs retained findings, while receiver deduplication
handles uncertain delivery acknowledgement.

Record dated evidence for every row on the **chosen permanent host**, with
private credentials, webhook URLs and raw sensitive logs excluded from the repo.

| Gate | Required evidence | Current result |
| --- | --- | --- |
| Zero-cost execution target | Authorized permanent deployment target, persistent local storage and verified no-overage cost conditions; no paid purchase/trial | BLOCKED: no provider identity/resource is connected |
| Artifact and configuration | Exact source SHA, archive/per-file checksum verification, correct architecture, strict config/original asserted baseline checks | Portable artifacts/checks available; target host UNVERIFIED |
| Disk and retention | Local persistent filesystem, measured space/RAM, reviewed evidence-preserving retention/capacity policy and alarms | Local pilot data only; production policy UNVERIFIED |
| Service operation | One chosen follower unit starts; loopback status, readiness and metrics advance with their assurance fields | Local pilot only; permanent host UNVERIFIED |
| Queue and receiver, if delivery enabled | Controlled finding sync; authorized receiver verifies HMAC/idempotency; ACK persists on restart | Local TLS fixtures PASS; production delivery not enabled/validated |
| Routing and TLS | RPC route and CA trust work; when delivery is enabled, the receiver/proxy route also works without bypass | RPC pilot and local proxy fixtures PASS; production target UNVERIFIED |
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

Do not check a row merely because its command exists. For the intended native
service, record actual permanent-target, restart, retention and cost evidence,
plus receiver acceptance if notification delivery is enabled. Public website
hosting is not a gate for the native tool. Browser polling cannot replace native
history/cursor/evidence behavior. The generic environment Publish error still has
no causal diagnostic; no code change can be claimed to prevent a platform outage.

Phase 9 remains BLOCKED until the native topology has actually been deployed and
verified. Baseline/source UNVERIFIED, commitments NOT_VERIFIED, coverage UNKNOWN
and verdict UNDETERMINED remain unchanged by any hosting or setup acceptance.
