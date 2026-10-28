# Demo recording script

A 4–6 minute screen recording for reviewers. Record a real run; do not edit
output. Use a fresh Testnet identity and fixture fleet, because Testnet resets
remove old state. Never show a secret key, webhook URL or HMAC key on screen.

## 0. Setup (before recording)

- Clean checkout at the commit being submitted; Go from `.go-version`.
- Rust 1.99.1 with `wasm32v1-none` and Stellar CLI 28.1.0 for the fixtures.
- Network access to the Testnet RPC and friendbot.

## 1. The problem (30 s, slide or README)

Show the CAP-85 diagram from [How CAP-85 fleets work](../concepts/cap85.md):
one owner entry, many members, one write upgrades all of them.

## 2. Build and test (45 s)

```sh
go test ./...
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch help
```

## 3. Offline detection (45 s)

```sh
./fleetwatch compare --before testdata/before.json --after testdata/after.json
./fleetwatch compare --before testdata/before.json --after testdata/after.json \
  --policy testdata/approved-policy.json
```

Point out: the first exits 2 with both members listed; the second matches the
unsigned policy, and authorization still says `UNVERIFIED`.

## 4. Live Testnet fleet (2–3 min)

Follow [TESTNET_DEMO.md](../TESTNET_DEMO.md) with a new identity:

1. `./scripts/demo-testnet.sh check` (offline fixture build and tests).
2. `./scripts/demo-testnet.sh run KEY_ALIAS CONFIG_DIR NEW_OUTPUT_DIR` with a
   freshly funded Testnet key alias whose config directory is outside the
   repository. It deploys the owner and two members and publishes Wasm A.
3. Start ingestion or `follow` against Testnet.
4. Publish Wasm B. Show the finding: owner, tag hex, old/new hash, both members.
5. Repoint one member to direct Wasm, publish again. Show only the other member affected.
6. Open the transaction in a Testnet explorer next to the finding.

## 5. Restart and gap handling (45 s)

Stop the process mid-run with Ctrl-C, restart, and show it resumes from the
stored cursor with no duplicate findings (`follow-inspect` or `checkpoint`).

## 6. Limits (20 s)

Show `/status` or a JSON report with `source_authentication: UNVERIFIED`,
`metadata_commitment: NOT_VERIFIED` and `final_verdict: UNDETERMINED`, and say
what they mean.
