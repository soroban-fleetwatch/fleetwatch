# Guide: operator

**Goal:** keep the follower running, know when it is behind or stuck, and
recover without losing evidence.

## Install

The portable Linux package contains the binary, an installer, systemd unit
templates and a read-only preflight script. Build it from a clean checkout:

```sh
./scripts/package-linux.sh /tmp/fleetwatch-pkg amd64
```

Installation, unit setup and the preflight are in [Operations](../OPERATIONS.md).
Status and metrics listen on loopback only.

## Health signals

| Route | Use |
| --- | --- |
| `/livez` | Process is up |
| `/readyz` | Fails when the follower is not ready |
| `/status` | JSON status with assurance labels |
| `/metrics` | Text metrics |

Metrics to alert on:

| Metric | Watch for |
| --- | --- |
| `fleetwatch_follow_ready` | 0 for more than a few poll intervals |
| `fleetwatch_follow_endpoint_ledger_lag` | Rising steadily |
| `fleetwatch_follow_gap_episodes_total` | Any increase |
| `fleetwatch_follow_retained_ledgers`, `fleetwatch_follow_retained_payload_bytes` | Approaching configured limits |
| `fleetwatch_follow_pending_findings` | Retained findings; nearing the 10,000 limit. Not an undelivered count: use `alerts` on the delivery DB for that. |

## Stopping and restarting

SIGTERM and SIGINT cancel in-flight requests. Committed batches are kept. On
start the follower replays its retained evidence from the original baseline and
then continues from the stored cursor. A second process on the same file waits
for the lock and then fails.

## Exit codes that need you

| Code | Action |
| --- | --- |
| 4 | Source unavailable or incomplete. Check the endpoint; restart resumes. |
| 5 | State or output failure. Check disk and permissions. Do not delete the DB. |
| 6 | Retention limit reached. Archive, verify, then restore or compact into a new state. |

## Backups

Stop the follower, then copy state with the commands in [Backup](../BACKUP.md)
and [Evidence archives](../ARCHIVES.md). Both write new files and never
overwrite. Restoring an older backup can forget later deliveries and nonce use;
reconcile with your receiver.

## Delivery problems

`doctor` exits 2 when alerts are dead-lettered or exhausted. Inspect with
`alerts` and `recoveries`, fix the receiver, then requeue with `requeue`, which
records an audit receipt. See [Alert recovery](../ALERT_RECOVERY.md).
