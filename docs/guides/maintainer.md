# Guide: protocol maintainer

**Goal:** watch a fleet you run and learn about any change to its shared code.

You need the network passphrase, an HTTPS Stellar RPC endpoint, your owner
contract ID, the tag, and the IDs of the member contracts you want to watch.

## 1. Try it offline first

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch compare --before testdata/before.json --after testdata/after.json
```

This prints an `UNAPPROVED_REFERENCE_CHANGE` finding for the two demo members
and exits 2. Nothing touches the network.

## 2. Write the baseline

The follower starts from a **baseline**: the executable of each watched
contract and the hash behind each reference, at one ledger, plus that ledger's
header hash. You assert it; FleetWatch checks that the first ledger it reads
links back to that header hash.

Use [`testdata/reconstruction-baseline.json`](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/testdata/reconstruction-baseline.json)
as the shape, and validate against
[`schema/watched-baseline-v1.schema.json`](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/schema/watched-baseline-v1.schema.json).
Rules:

- 1 to 50 watched contracts, as checksum-valid `C…` strings.
- Every reference a watched contract uses must be listed with its hash.
- `tag_hex` is the exact tag bytes in hex.
- `header_hash` is the hash of the baseline ledger; the next ledger must link to it.

`capture` reads current entries for a watchlist and is a useful starting
point, but its output is `PARTIAL` because separate reads are not proven to
come from one ledger. Review what you assert.

## 3. Write the follow configuration

```json
{
  "schema_version": 1,
  "rpc_url": "https://soroban-testnet.stellar.org",
  "baseline": "baseline.json",
  "state": "follow.db",
  "listen": "127.0.0.1:9789",
  "poll_interval_seconds": 5,
  "batch_size": 8,
  "head_delay_ledgers": 1,
  "max_endpoint_lag": 4,
  "max_retained_ledgers": 100000,
  "max_retained_payload_bytes": 67108864
}
```

Paths are relative to the configuration file. Every field is required. See
[Follow](../FOLLOW.md) for the allowed ranges.

## 4. Check, then run

```sh
./fleetwatch follow --check --config follow.json   # offline: config and baseline only
./fleetwatch follow --config follow.json
```

In another shell:

```sh
curl --fail http://127.0.0.1:9789/readyz
curl --fail http://127.0.0.1:9789/metrics
```

## 5. Read findings

Stop the follower (or use a copy of the state), then:

```sh
./fleetwatch follow-inspect --config follow.json --limit 20
```

Each finding names the owner, tag hex, old and new hash, ledger, affected
watched members and evidence digests.

## 6. Get alerts

To send findings to your own HTTPS endpoint, run the follower with a separate
delivery database and the dispatcher. See [Follow delivery](../FOLLOW_DELIVERY.md)
and [Delivery loop](../DELIVERY_LOOP.md). Your receiver must verify the HMAC
signature and deduplicate by alert ID.

## Limits to keep in mind

- Contracts you do not list are not watched.
- The endpoint is trusted for data; results stay `UNVERIFIED`.
- When retention limits are reached, `follow` stops with exit 6. Archive and
  restart as described in [Evidence archives](../ARCHIVES.md).
