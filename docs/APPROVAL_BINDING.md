# Durable offline approval association

`bind-approval` records a verified approval against one existing reference-change
event in the local replay database. It derives network, source, owner/tag,
old/new hashes and ledger from that stored event; there is no caller-supplied
observation file to substitute for it. It does not authenticate chain history,
change finding severity, set authorization to APPROVED, or rewrite queued alerts.

An entirely synthetic example (all files are public test fixtures):

```sh
/workspace/.tools/bin/fleetwatch replay --journal testdata/approval-journal.json \
  --state /workspace/approval-demo.db > /workspace/approval-demo-report.json
# expected exit 2: A -> B -> A -> B, three unverified findings

FLEETWATCH_EVENT_ID="$(python3 -c 'import json; print(json.load(open("/workspace/approval-demo-report.json"))["events"][0]["id"])')"
/workspace/.tools/bin/fleetwatch bind-approval \
  --state /workspace/approval-demo.db \
  --network TESTNET-SYNTHETIC-NOT-A-REAL-NETWORK --source synthetic \
  --event-id "$FLEETWATCH_EVENT_ID" \
  --approval testdata/signed-approval.json --trust testdata/approval-trust.json
```

The first binding exits 0 with new_binding true. Repeating the exact command
exits 0 with new_binding false and the original receipt. The final chain verdict
remains UNDETERMINED. Attempting the same approval against the later A->B event
(the third event in the demo report) exits 2: the same nonce cannot be used for a
different incident even though its transition fields and ledger window match.
Do not treat the fixture public key or checksum-valid test address as operational
ownership or a real chain deployment.

The existing standalone `verify-approval` command remains read-only and repeatable.
Binding is an additional explicit state mutation. Current trust configuration and
revocation are rechecked on every bind/rebind. Failed verification and incomplete
input sequences do not consume a nonce. Unknown/unsupported incidents fail closed;
only persisted UNAPPROVED_REFERENCE_CHANGE events with checksum-valid owner
addresses can currently be associated. Gaps in the retained journal prevent
binding until repaired. A binding does not authorize filling or ignoring a gap.

## Atomicity and scope

One synchronous bbolt transaction writes the event receipt and nonce reservation.
The nonce scope is SHA-256 of a versioned tuple of network passphrase hash and
owner; the reservation key is the signed nonce. Tags and source kinds are not
part of that scope, so changing either cannot reuse that nonce within the same
owner/network. Each event can have only one statement/trust-version association.
A new approval with a fresh nonce cannot silently replace an existing receipt.

The receipt retains the public signed envelope, public trust configuration,
statement digest, canonical trust digest, original verification result, event ID,
network/source and local recording time. The local timestamp is not a ledger
attestation. Trust digest hashes `fleetwatch.trust.v1\n` plus compact Go JSON of
the Trust struct, with keys sorted by fingerprint and allowed tags sorted. Field
ordering changes in a file do not change the decoded struct; array reorderings
also retain the same trust identity. Changes to the actual trust policy require
an explicit future migration workflow; they cannot silently rewrite a receipt.

Existing receipts are checked against their retained envelope/trust and incident
before idempotent return. Current revocation can reject a previously recorded
approval; its historical receipt remains stored, not a claim of current trust.
The nested verification object describes the signature check; presence of a
binding receipt describes persistence. Read both and honor the process exit code.

Tests exercise cold reopen, idempotence, later same-tuple reuse, alternate nonce
replacement, source-scope switching, concurrent competing incidents, revocation,
gap repair, stored-verdict tampering, injected write failure and a subprocess crash
after both writes but before commit. Failed transactions retain neither half of
the association. Repeating an exact request is safe after an output failure.
An I/O error during commit may require reopening/retry to establish persistence.

This is replay prevention within one database history, not a global nonce registry.
A fresh database, cloned database, or old backup cannot establish consumption in
another copy. Keep receipts and reservations together during backup/restore or
migration; deleting either side is unsupported. No unbind, reassignment, pruning,
or distributed coordination is implemented. Limits are 10,000 bindings per
network/source, 10,000 nonce reservations per owner/network, and 256 KiB per
receipt. No private key or secret is stored. Trust roots remain explicit operator
assertions that need out-of-band validation and file protection.

Exit 0: receipt recorded or idempotently found; 2: verification rejected or
nonce/event association conflict; 3: malformed arguments/envelope/trust;
4: event unavailable/unsupported or incomplete input sequence; 5: state/output
failure. No severity downgrade or automatic delivery follows successful binding.
Independently authenticated ledger evidence, signed trust distribution, recovery
workflows and production Postgres migration remain unfinished.
