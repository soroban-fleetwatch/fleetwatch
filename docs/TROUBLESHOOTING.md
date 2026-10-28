# Troubleshooting

Start with the exit code, then the `code` or `reason` field in the JSON output.
Error codes are fixed strings; FleetWatch does not echo raw endpoint responses,
URLs or keys into errors.

## By exit code

| Exit | Typical cause | What to do |
| --- | --- | --- |
| 2 | A finding met `--fail-on`, or `doctor` found dead letters / exhausted attempts | Read the findings. For delivery, see [Alert recovery](ALERT_RECOVERY.md). |
| 3 | Bad flag, missing required flag, invalid JSON, unknown field, path collision | Run `fleetwatch COMMAND -h`. Validate files against `schema/`. |
| 4 | Source unavailable, missing ledger, `PARTIAL` capture, absent scope | Check the endpoint and range. Committed prefixes are kept; rerun to resume. |
| 5 | Database locked, unreadable or corrupt; output could not be written | Stop other FleetWatch processes on the file. Check disk and permissions. Keep the file for analysis. |
| 6 | `follow` retention limit reached | Archive, verify and restore or compact. See [Evidence archives](ARCHIVES.md). |

## Common codes

| Code | Meaning | What to do |
| --- | --- | --- |
| `MISSING_OR_UNEXPECTED_LEDGER` | The endpoint did not return the next ledger in sequence | Usually the ledger is outside the endpoint's retention window or not closed yet. Use an endpoint that retains the range. |
| `RETENTION_RANGE_UNAVAILABLE` | Requested ledgers are older than the endpoint keeps | Same as above. Public RPC keeps a limited window. |
| `BASELINE_HEADER_LINK_MISMATCH` | The first fetched ledger does not link to the baseline's `header_hash` | The baseline ledger or header hash is wrong, or the endpoint is on a different chain. |
| `BASELINE_RANGE_OR_NETWORK_MISMATCH` | Baseline network or ledger does not match the request | Check `network` and `ledger` in the baseline. |
| `NETWORK_MISMATCH` / `OTHER_NETWORK` | The endpoint reports a different network passphrase | Point the config at the right endpoint. Networks are never mixed. |
| `UNSUPPORTED_NETWORK_PROTOCOL` | Endpoint protocol is outside the supported range (history: 28–29) | Upgrade FleetWatch when support for the new protocol lands. |
| `HEADER_CHAIN_DIVERGENCE` | A header's previous-hash link does not match | Treat as a source problem. Do not override. |
| `PREIMAGE_MISMATCH` | A change's "before" value differs from FleetWatch's state | Baseline is wrong or the source is inconsistent. Processing stops. |
| `UNORDERED_WATCHED_EVICTION` | A watched entry was evicted where order cannot be proven | Processing stops rather than guess. Archive/restore lifecycle support is planned. |
| `EVIDENCE_CONFLICT` | New evidence for an already stored ledger differs | The stored prefix is kept. Investigate the source. |
| `CAPACITY_REACHED` | A retention or output budget was reached | See exit 6 above. |
| `DELIVERY_DEAD_LETTERS` | Alerts exhausted their attempts | Fix the receiver, then `requeue`. |
| `CANCELLED` / `DEADLINE_EXCEEDED` | SIGTERM/SIGINT or a time budget | Rerun; committed work is kept. |

## Proxies and TLS

RPC requests use the system's proxy settings and CA trust. Webhook delivery
does not use a proxy unless you pass `--proxy-mode pinned-connect`; otherwise a
configured proxy makes delivery fail closed. See [Webhook proxy](WEBHOOK_PROXY.md).

## Getting help

Open an issue with the command, exit code and sanitized output. Do not paste
webhook URLs, keys or private endpoints. Security problems go to
[SECURITY.md](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/SECURITY.md).
