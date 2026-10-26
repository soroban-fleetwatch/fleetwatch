# Experimental signed HTTPS delivery

The `deliver` command sends a bounded batch from an existing offline outbox. It is
separate from `replay` and `alerts`; neither sends automatically. No external
receiver has been contacted or configured during development. Tests use local TLS
receivers and temporary test keys only.

To use delivery, configure an actual receiver and securely provide a raw HMAC key
through your runtime secret manager. The default variable name is
`FLEETWATCH_WEBHOOK_KEY`; `--hmac-key-env NAME` changes the lookup name. The value
must contain 32–4096 bytes. Do not put keys in command arguments, Git, URLs, or
chat. Proxy-substituted API-token placeholders are not usable local HMAC keys.
No credential requirement is saved by default because a receiver is optional.

Example (replace the host/URL with your receiver; this sends alerts):

```sh
/workspace/.tools/bin/fleetwatch deliver \
  --state /workspace/replay.db --network TESTNET-SYNTHETIC --source synthetic \
  --webhook-url https://hooks.example.com/fleetwatch \
  --allow-host hooks.example.com --limit 10
```

The hostname allowlist is exact, lowercase, and has no wildcards. Only HTTPS on
port 443 is accepted. IP literals, credentials, query strings (including empty `?`)
and fragments are rejected. URLs are normalized before deriving the persistent
sink ID; the signing key does not enter sink identity, so key rotation alone does
not change routing. Rebinding queued records to a different sink is not supported.

The direct transport checks all DNS answers and rejects private, loopback,
link-local, multicast, shared, documentation and selected special-purpose ranges.
It dials the checked numeric address, retaining the original hostname for TLS
verification; every new connection resolves and validates again. Mixed public and
private answers are rejected as a whole. Redirects are disabled. These are local
transport defenses, not permission to connect to arbitrary public hosts.

**Proxy routing is unsupported.** The public constructor fails before delivery
when Go's environment proxy configuration would route the receiver through a
proxy. It does not silently bypass that proxy or disable TLS verification. This
cloud environment may require proxy routing, in which case this sender cannot
perform live delivery here. A future proxy-aware implementation needs equivalent
verified destination controls; do not unset a required proxy to evade policy.
Private-network receivers are intentionally unsupported by this sender.

Requests are POST JSON with an envelope `{schema_version:1,id,payload}`. The body
contains immutable alert payload only; delivery status, lease tokens and keys are
excluded. Headers:

- `Idempotency-Key`: stable alert ID.
- `X-FleetWatch-Timestamp`: decimal Unix seconds.
- `X-FleetWatch-Signature`: `v1=` plus lowercase hex HMAC-SHA256.

The exact signed byte sequence is:

```text
v1\n<timestamp>\n<alert_id>\n<exact HTTP body bytes>
```

A receiver must use the original body bytes, compare the MAC in constant time,
validate timestamp freshness using its own clock (for example, a documented
five-minute tolerance), and durably deduplicate the alert ID. Never parse and
re-serialize JSON before MAC verification. Timestamp validation limits replay
windows; idempotency also handles retries signed with a fresh timestamp. TLS and
this MAC authenticate transport/sender possession of a shared key; they do not
verify the supplied snapshots, chain history or upgrade authorization. Payloads
retain UNVERIFIED_INPUT/UNVERIFIED assurance labels.

Limits: 1 MiB request body, 16 KiB response headers, 64 KiB response body, 5-second
connect/TLS limits, 10-second response-header limit, 15-second send timeout, and a
two-minute CLI batch deadline. A batch holds the state database's exclusive lock.
There are no immediate sender retries: the durable outbox owns retries/backoff.
2xx means accepted; 408/429/5xx and transport/read failures are transient. Other
non-2xx statuses, redirects, unsafe destinations and certificate validation errors
are permanent failures and dead-letter the alert. Receiver bodies, URLs and raw
transport errors are not copied into delivery errors or durable failure records.

The JSON command result reports attempted/delivered/failed counts. Exit 0 means
this batch had no failures (possibly no ready records); 3 invalid configuration or
unavailable key, including unsupported proxy routing; 4 delivery failure/interrupted
batch; 5 state/output failure. Some sends can succeed before a batch fails; inspect
the durable outbox before retrying. Delivery remains at least once. A receiver may
accept a request before its acknowledgement is committed; stable IDs are required
to avoid duplicate side effects. Manual [dead-letter recovery](ALERT_RECOVERY.md) preserves the bound sink, payload
and stable ID. Continuous watching remains future work.

Reconstruction delivery uses `--source rpc_ledger_meta --stream ID`. Its inner
payload uses version 2 with mandatory unverified labels; the outer v1 envelope
and MAC are unchanged. See [RECONSTRUCTION_ALERTS.md](RECONSTRUCTION_ALERTS.md)
before updating a receiver. Historical diagnostics can arrive after repair.
