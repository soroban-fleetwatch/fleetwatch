# Opt-in follow webhook dispatcher

`deliver-loop` repeatedly dispatches an imported follow outbox to one explicitly
configured receiver. It is separate from source collection and local queue sync.
**Starting this command sends queued notifications.** No receiver, key, service
start or external notification is configured automatically by the installer.

After choosing an authorized receiver, provisioning its raw HMAC key through the
host's supported secret mechanism, and reviewing [WEBHOOKS.md](WEBHOOKS.md):

```sh
fleetwatch deliver-loop --state delivery.db \
  --network 'Test SDF Network ; September 2015' --stream pilot \
  --webhook-url https://hooks.example.invalid/fleetwatch \
  --allow-host hooks.example.invalid \
  --hmac-key-env FLEETWATCH_WEBHOOK_KEY \
  --batch-size 1 --poll-interval-seconds 5
```

The `.invalid` hostname is a placeholder. Replace it with your receiver before
starting. This command accepts only the imported follow source; no `--source`
flag is needed. Network/stream must match the existing outbox import. The key is
32–4096 raw bytes from the named environment variable, never a command argument.
Restart after key rotation; the running sender retains its startup key.

Adding `--check` validates local options, key availability and the sender's URL/
allowlist/proxy policy without opening a database, resolving DNS or sending a
request. It does not establish receiver availability or validate queued state.
TLS/public DNS/IP pinning, HTTPS port 443, redirect rejection, HMAC, sink binding
and existing request/response limits remain unchanged. Proxy routes are rejected
by default; explicit [pinned CONNECT mode](WEBHOOK_PROXY.md) supports compatible
HTTP/HTTPS proxies without proxy-side receiver DNS. Real proxy permission and
production receiver routing must still be validated. Do not bypass policy.

## Runtime and coordination

The dispatcher opens **existing** delivery state for one bounded batch and
releases it before status output and interval waits. Missing state is never
created. A zero-length file is reported as OUTBOX_EMPTY and left untouched:
the producer may still be initializing, or operator recovery may be needed.
Nonempty invalid state, bad manifests or inconsistent queue payloads stop the
loop. No database is reset, repaired or silently replaced.

Defaults are one alert per batch and five seconds between batches. Allowed
batches are 1–5 alerts; polling interval is 1–60 seconds. A batch has a 90-second
dispatch deadline plus a bounded five-second post-batch inspection; individual
send and lease limits are unchanged. The existing outbox open can wait one second
for its lock. These bounds do not interrupt a blocked kernel filesystem call or
promise throughput/latency. Keep batches short so the automatic follower sync
can obtain the file within its two-minute deadline. Only one dispatcher should
operate a given queue/sink; an already bound sink cannot be silently changed.

SIGINT/SIGTERM cancel the current request/wait, preserve durable attempt/lease
state and emit STOPPED. An uncertain acknowledgement may be retried using the
same delivery ID. Receivers must durably deduplicate: this is at-least-once
delivery. Transient failures use the existing retry schedule; permanent failures
or exhausted attempts become DEAD. The loop continues reporting dead letters;
it does not automatically requeue them. Use the existing audited `requeue` and
`recoveries` commands after inspection.

NDJSON status follows [delivery-loop-status-v1.schema.json](../schema/delivery-loop-status-v1.schema.json).
It contains process cycle/time, fixed code, current batch counts and queue
counts (pending/inflight/delivered/dead). Queue counts are null when unavailable,
not fabricated zero. Process cycle resets at restart; durable row state remains
in the outbox. Status omits URL, key, IDs, lease tokens and raw transport errors.

| Code | Meaning |
| --- | --- |
| WAITING_FOR_OUTBOX / OUTBOX_EMPTY | Producer file is absent/empty; no state created or initialized |
| OUTBOX_BUSY / WAITING_FOR_IMPORT | File is held elsewhere, or no matching import manifest exists yet |
| DISPATCHED / DELIVERY_RETRY | Batch acknowledged messages, or transport failed and retained retry state |
| WAITING_FOR_RETRY | Pending/inflight rows exist but no attempt was currently eligible |
| DEAD_LETTERS_PRESENT | Dead rows require explicit operator review/recovery |
| ACK_UNCERTAIN / BATCH_TIMEOUT | Acknowledgement lease was stale, or bounded dispatch timed out; durable state governs retries |
| IDLE | No currently pending/inflight/dead rows in the selected imported queue; not a chain all-clear |
| STATE_FAILURE / STOPPED | Fatal state/validation failure, or graceful cancellation |

Exit 0 means graceful stop/check; 3 means invalid local options, unavailable key
or rejected transport policy; 5 means fatal state, storage or output failure.
Output failure after delivery does not undo acknowledgements. Inspect before
restarting; a restarted loop skips acknowledged rows and retains dead letters.

## Optional user service

The installed `fleetwatch-delivery.service` is separate from the two alternative
follower units. It can run alongside the chosen follower, but requires explicit
receiver/secret configuration in `$HOME/.config/fleetwatch/delivery` (mode 0600)
or an equivalent reviewed host secret setup. The required environment names are:

- FLEETWATCH_DELIVERY_NETWORK: exact imported network passphrase.
- FLEETWATCH_DELIVERY_STREAM: stable imported stream, e.g. pilot.
- FLEETWATCH_WEBHOOK_URL and FLEETWATCH_WEBHOOK_HOST: authorized URL and exact hostname.
- FLEETWATCH_WEBHOOK_KEY: provisioned shared HMAC key; no example secret is supplied.

The unit's local --check runs before the loop. No environment file/key is
generated, and no unit is linked, enabled or started by installation. Review
the rendered unit, environment, endpoint policy and `systemd-analyze --user
verify` on the actual host before linking/starting it. Starting the delivery unit
does not automatically start the follower. Static syntax checks in the build
environment do not prove host boot persistence, hardening or receiver operation.

Stop the follower and dispatcher for paired backup/recovery. Source and delivery
databases are separate transactions: preserve both, including later delivery
acknowledgements. Restoring an older outbox can repeat external notifications;
reconcile with receiver records before resuming. Native pending_findings counts
retained findings, not undelivered messages. Source-gap notifications, production
retention/rollover, shared storage and permanent-host validation remain open.
All chain/source/approval assurance remains unchanged and final UNDETERMINED.
