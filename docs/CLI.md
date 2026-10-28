# CLI reference

FleetWatch is one binary, `fleetwatch`, with one subcommand per task.

```sh
fleetwatch help              # list commands and synopses (exit 0)
fleetwatch COMMAND -h        # flags for one command (exit 0)
fleetwatch version
```

Flags accept one or two leading dashes (`-state` and `--state` are the same).
File paths in configuration files resolve relative to that configuration file.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success, including help. A successful query can still contain findings. |
| 2 | A finding met the `--fail-on` threshold, or delivery needs attention (`doctor`). |
| 3 | Invalid input, configuration or usage. |
| 4 | Source unavailable, or the observation is incomplete or `PARTIAL`. |
| 5 | State database or output failure. |
| 6 | `follow` reached a configured retention limit and stopped safely. |

`go run` prints `exit status N` and then exits with status 1 itself. Build the
binary when a script needs the exact code.

## What each command touches

| Command | Network | Writes |
| --- | --- | --- |
| `validate`, `compare`, `changes`, `verify-approval`, `verify-follow-archive` | none | nothing |
| `replay` | none | `--state` database, when given |
| `reconstruct` | none | `--state` database, when given |
| `report`, `checkpoint`, `doctor`, `alerts`, `recoveries` | none | nothing (read-only open) |
| `capture`, `history` | Stellar RPC (read-only) | nothing; output to stdout |
| `ingest`, `watch` | Stellar RPC (read-only) | state database |
| `follow` | Stellar RPC (read-only), loopback listener | follow state database |
| `follow-inspect`, `follow-sync` | none | opens follow state; `follow-sync` writes the delivery database |
| `follow-archive`, `follow-compact`, `restore-follow-archive`, `backup` | none | a new output file only; never overwrites |
| `requeue`, `bind-approval` | none | state database |
| `deliver`, `deliver-loop` | HTTPS webhook receiver | state database (delivery status) |

No command signs or submits a Stellar transaction. No command needs a private key.
Webhook commands read the HMAC key from an environment variable you name.

## Output and assurance labels

Machine-readable outputs are JSON with a `schema_version` and a matching JSON
Schema in [`schema/`](https://github.com/soroban-fleetwatch/fleetwatch/tree/main/schema).
Reports carry assurance labels such as `UNVERIFIED`, `NOT_VERIFIED`, `UNKNOWN`
and `UNDETERMINED`. See [Assurance labels](concepts/assurance.md).

## Commands

Each entry below is generated from the binary: the synopsis comes from
`fleetwatch help` and the flag list from `fleetwatch COMMAND -h`. A unit test
fails if this page and the code disagree.

<!-- BEGIN GENERATED COMMAND REFERENCE: do not edit; run FLEETWATCH_UPDATE_DOCS=1 go test ./cmd/fleetwatch -run TestCLIReferenceUpToDate -->

### `validate`

```text
fleetwatch validate --snapshot fleet.json
```

```text
Usage of validate:
  -snapshot string
    	path to fleet snapshot JSON
```

### `compare`

```text
fleetwatch compare --before old.json --after new.json [--policy approvals.json] [--format text|json] [--fail-on critical|high|warning|info|never]
```

```text
Usage of compare:
  -after string
    	later snapshot JSON
  -before string
    	earlier snapshot JSON
  -fail-on string
    	critical, high, warning, info or never (default "high")
  -format string
    	text or json (default "text")
  -policy string
    	optional approval policy JSON; absent means no approved changes
```

### `replay`

```text
fleetwatch replay --journal per-ledger-snapshots.json
```

```text
Usage of replay:
  -journal string
    	bounded offline per-ledger snapshot journal JSON
  -state string
    	optional local bbolt database for resumable offline replay
```

### `report`

```text
fleetwatch report --state replay.db --network TEXT --ledger N --owner ID --tag-hex HEX
```

```text
Usage of report:
  -ledger uint
    	exact retained ledger
  -network string
    	exact journal network
  -owner string
    	exact executable reference owner
  -source string
    	synthetic or user_snapshot (default "synthetic")
  -state string
    	existing local replay database (read only)
  -tag-hex string
    	raw tag bytes in hex
```

### `capture`

```text
fleetwatch capture --rpc-url HTTPS_URL --network-passphrase PASSPHRASE --contracts-file FILE
```

```text
Usage of capture:
  -contracts-file string
    	watchlist: one canonical contract address per line, maximum 50
  -network-passphrase string
    	exact expected network passphrase
  -rpc-url string
    	HTTPS Stellar RPC endpoint
```

### `history`

```text
fleetwatch history --rpc-url HTTPS_URL --network-passphrase TEXT --from-ledger N --to-ledger M
```

```text
Usage of history:
  -from-ledger uint
    	first ledger (inclusive)
  -network-passphrase string
    	exact expected network passphrase
  -rpc-url string
    	HTTPS Stellar RPC endpoint
  -to-ledger uint
    	last ledger (inclusive; at most 16 ledgers)
```

### `changes`

```text
fleetwatch changes --capture history-capture.json
```

```text
Usage of changes:
  -capture string
    	diagnostic history capture JSON
```

### `reconstruct`

```text
fleetwatch reconstruct --baseline watched-baseline.json --capture history-capture.json [--state FILE --stream ID]
```

```text
Usage of reconstruct:
  -baseline string
    	operator-asserted watched baseline JSON
  -capture string
    	raw history capture JSON (not normalized changes)
  -state string
    	optional local database for retained reconstruction evidence
  -stream string
    	operator-assigned stream ID (required with --state)
```

### `checkpoint`

```text
fleetwatch checkpoint --state FILE --network TEXT --stream ID
```

```text
Usage of checkpoint:
  -network string
    	exact persisted network
  -state string
    	existing database (read only)
  -stream string
    	operator-assigned stream ID
```

### `ingest`

```text
fleetwatch ingest --rpc-url HTTPS_URL --baseline FILE --state FILE --stream ID --to-ledger N
```

```text
Usage of ingest:
  -baseline string
    	operator-asserted baseline JSON
  -rpc-url string
    	HTTPS Stellar RPC endpoint
  -state string
    	local database for durable evidence
  -stream string
    	operator-assigned stream ID
  -to-ledger uint
    	inclusive target within 16 ledgers of baseline
```

### `watch`

```text
fleetwatch watch --config FILE
```

```text
Usage of watch:
  -config string
    	bounded watch JSON configuration
```

### `follow`

```text
fleetwatch follow --config FILE [--check] [--delivery-state FILE --delivery-stream ID]
```

```text
Usage of follow:
  -check
    	validate configuration and baseline without opening state or querying RPC
  -config string
    	follow configuration JSON
  -delivery-state string
    	optional separate outbox database for automatic sync (no dispatch)
  -delivery-stream string
    	stable stream required with --delivery-state
```

### `follow-inspect`

```text
fleetwatch follow-inspect --config FILE [--limit N --after-id ID]
```

```text
Usage of follow-inspect:
  -after-id string
    	exclusive stable finding ID cursor
  -config string
    	follow configuration JSON
  -limit int
    	finding page size 1-100 (default 20)
```

### `follow-archive`

```text
fleetwatch follow-archive --config FILE --output NEW_FILE
```

```text
Usage of follow-archive:
  -config string
    	stopped follow configuration
  -output string
    	new evidence archive path
```

### `verify-follow-archive`

```text
fleetwatch verify-follow-archive --input FILE
```

```text
Usage of verify-follow-archive:
  -input string
    	existing gzip evidence archive
```

### `restore-follow-archive`

```text
fleetwatch restore-follow-archive --input FILE --output NEW_DB --sha256 EXPECTED_DIGEST
```

```text
Usage of restore-follow-archive:
  -input string
    	existing gzip evidence archive
  -output string
    	NEW follow database path
  -sha256 string
    	expected compressed archive SHA256 from trusted receipt
```

### `follow-compact`

```text
fleetwatch follow-compact --config FILE --output NEW_DB
```

```text
Usage of follow-compact:
  -config string
    	stopped follow configuration
  -output string
    	NEW compacted database path
```

### `follow-sync`

```text
fleetwatch follow-sync --config FILE --delivery-state FILE --stream ID
```

```text
Usage of follow-sync:
  -config string
    	stopped follow configuration
  -delivery-state string
    	separate durable delivery outbox database
  -stream string
    	stable delivery stream identifier
```

### `alerts`

```text
fleetwatch alerts --state replay.db --network TEXT [--source synthetic|user_snapshot]
```

```text
Usage of alerts:
  -network string
    	exact journal network
  -source string
    	synthetic, user_snapshot, rpc_ledger_meta or follow_ledger_meta (default "synthetic")
  -state string
    	existing local replay database
  -stream string
    	required with ledger-meta sources
```

### `deliver`

```text
fleetwatch deliver --state replay.db --network TEXT --webhook-url HTTPS_URL --allow-host HOST
```

```text
Usage of deliver:
  -allow-host string
    	exact lowercase allowed receiver hostname
  -hmac-key-env string
    	environment variable holding raw HMAC key (32-4096 bytes) (default "FLEETWATCH_WEBHOOK_KEY")
  -limit int
    	maximum alerts this batch, 1-50 (default 10)
  -network string
    	exact journal network
  -proxy-mode string
    	reject proxy routes or explicitly use pinned-connect (default "reject")
  -source string
    	synthetic, user_snapshot, rpc_ledger_meta or follow_ledger_meta (default "synthetic")
  -state string
    	existing replay database
  -stream string
    	required with ledger-meta sources
  -webhook-url string
    	HTTPS receiver URL
```

### `deliver-loop`

```text
fleetwatch deliver-loop --state delivery.db --network TEXT --stream ID --webhook-url HTTPS_URL --allow-host HOST [--batch-size 1 --poll-interval-seconds 5]
```

```text
Usage of deliver-loop:
  -allow-host string
    	exact allowed receiver hostname
  -batch-size int
    	alerts per bounded batch, 1-5 (default 1)
  -check
    	validate local options/key/transport policy without opening state, DNS or delivery
  -hmac-key-env string
    	environment variable holding raw HMAC key (default "FLEETWATCH_WEBHOOK_KEY")
  -network string
    	exact imported network
  -poll-interval-seconds int
    	wait between bounded batches, 1-60 seconds (default 5)
  -proxy-mode string
    	reject proxy routes or explicitly use pinned-connect (default "reject")
  -state string
    	existing imported follow outbox (waits if not created yet)
  -stream string
    	stable follow delivery stream
  -webhook-url string
    	explicit authorized HTTPS receiver URL
```

### `requeue`

```text
fleetwatch requeue --state FILE --network TEXT --alert-id ID --request-id ID --expected-cycle N --reason TEXT
```

```text
Usage of requeue:
  -alert-id string
    	alert ID
  -expected-cycle int
    	current recovery cycle (0 initially) (default -1)
  -network string
    	exact network
  -reason string
    	nonsecret operator rationale
  -request-id string
    	unique operator request ID
  -source string
    	synthetic, user_snapshot, rpc_ledger_meta or follow_ledger_meta (default "synthetic")
  -state string
    	existing local database
  -stream string
    	required for ledger-meta sources
```

### `recoveries`

```text
fleetwatch recoveries --state FILE --network TEXT --alert-id ID
```

```text
Usage of recoveries:
  -alert-id string
    	alert ID
  -network string
    	exact network
  -source string
    	synthetic, user_snapshot, rpc_ledger_meta or follow_ledger_meta (default "synthetic")
  -state string
    	existing local database
  -stream string
    	required for ledger-meta sources
```

### `verify-approval`

```text
fleetwatch verify-approval --approval envelope.json --trust trust.json --observation transition.json
```

```text
Usage of verify-approval:
  -approval string
    	versioned signed envelope JSON
  -observation string
    	supplied transition observation JSON
  -trust string
    	operator-provided public trust configuration JSON
```

### `bind-approval`

```text
fleetwatch bind-approval --state replay.db --network TEXT --event-id ID --approval envelope.json --trust trust.json
```

```text
Usage of bind-approval:
  -approval string
    	signed approval envelope JSON
  -event-id string
    	exact persisted reference-change event ID
  -network string
    	exact persisted journal network
  -source string
    	synthetic or user_snapshot (default "synthetic")
  -state string
    	existing replay database
  -trust string
    	operator-provided public trust configuration JSON
```

### `doctor`

```text
fleetwatch doctor --state replay.db --network TEXT [--source synthetic|user_snapshot|rpc_ledger_meta] [--stream ID for rpc_ledger_meta]
```

```text
Usage of doctor:
  -network string
    	exact journal network
  -source string
    	synthetic, user_snapshot or rpc_ledger_meta (default "synthetic")
  -state string
    	existing offline replay database (read only)
  -stream string
    	required with --source rpc_ledger_meta
```

### `backup`

```text
fleetwatch backup --state replay.db --output NEW_FILE
```

```text
Usage of backup:
  -output string
    	new backup path in an existing directory
  -state string
    	existing replay or follow database (read only)
```

<!-- END GENERATED COMMAND REFERENCE -->
