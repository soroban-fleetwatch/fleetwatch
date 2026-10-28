# Guide: security responder

**Goal:** after a suspicious upgrade, find what code was running at a ledger,
which contracts it reached, and what evidence supports that.

## What ran at ledger N?

For snapshot replay state, `report` answers this for one owner/tag at an exact
retained ledger. It is read-only.

```sh
./fleetwatch replay --journal testdata/replay-transient.json --state replay.db
./fleetwatch report --state replay.db --network TESTNET-SYNTHETIC \
  --ledger 101 --owner OWNER_DEMO --tag-hex 72656c656173652d737461626c65
```

The output shows the hash and members at ledger 101 and at the previous
retained ledger, with snapshot digests:

```text
"current":  { "ledger": 101, "wasm_hash": "bbbb…", "members": ["CONTRACT_DEMO_A", "CONTRACT_DEMO_B"] }
"previous": { "ledger": 100, "wasm_hash": "aaaa…", "members": ["CONTRACT_DEMO_A", "CONTRACT_DEMO_B"] }
"membership_coverage": "UNKNOWN", "final_verdict": "UNDETERMINED"
```

`report` never interpolates. A ledger that was not retained, or a gap, returns
exit 4 instead of a guess.

## Which contracts were affected?

Every reference-change finding lists the watched members that used the
reference at that operation. Members that switched away before the change are
not included; members that switched to a direct Wasm hash get their own
repoint finding. Contracts outside the watchlist are unknown, not unaffected.

For follow state, list findings with `follow-inspect`. For reconstruction
streams, use `checkpoint --state FILE --network TEXT --stream ID`.

## Is the record intact?

```sh
./fleetwatch doctor --state replay.db --network TESTNET-SYNTHETIC
```

`OK_OFFLINE` means the local database is consistent. It is not a chain-safety
verdict. Exit 4 means a scope is missing or incomplete; exit 2 means delivery
needs attention.

For follow state, export a compressed archive and verify it on another machine:

```sh
./fleetwatch follow-archive --config follow.json --output evidence.ndjson.gz
./fleetwatch verify-follow-archive --input evidence.ndjson.gz
```

Verification replays the archived raw metadata from the original baseline. See
[Evidence archives](../ARCHIVES.md).

## Checking the chain yourself

Findings include the ledger number. To find the transaction, run `changes`
on a captured range: its records keep each change's transaction hash as an
unverified metadata claim. Then look the transaction up in a block explorer or
with your own RPC `getTransaction` call. FleetWatch does not authenticate the
endpoint, so a second, independent look is part of the response.

## What not to conclude

A code change is not proof of an attack. FleetWatch does not attribute a
change to a person and does not take any action on-chain.
