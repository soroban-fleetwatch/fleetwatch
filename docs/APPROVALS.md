# Offline signed approval verification

`verify-approval` verifies a custom version-1 Ed25519 statement against supplied
transition fields and explicit operator-provided public trust roots. It is
separate from the unsigned `compare --policy` format. No comparator severity,
queued alert authorization, or chain provenance changes as a result.

```sh
/workspace/.tools/bin/fleetwatch verify-approval \
  --approval testdata/signed-approval.json \
  --trust testdata/approval-trust.json \
  --observation testdata/approval-observation.json
```

This public-only example is synthetic: its network is not an actual Stellar
network, its contract address is a checksum-valid test value with no claimed
chain deployment, and its signatures prove no real upgrade. A temporary test key
created its signature; the private key was not retained. The matching test trust
file is not an operational trust root. Tests also generate ephemeral keys in
memory. No signing command, production key creation or external signing service
is implemented.

The expected result has policy_match, signature_verified, signer_trusted and
approval_valid_for_ledger set to YES, verification_status VERIFIED, and final_verdict
UNDETERMINED. VERIFIED refers only to the approval for the supplied fields. The
currently supported observation sources are synthetic, user_snapshot and
rpc_current; none establishes independent chain authenticity. The final chain
verdict therefore remains UNDETERMINED for all current sources, including inputs
whose completeness field says COMPLETE. Partial/unknown input completeness also
sets verification_status UNDETERMINED, retaining diagnostic signature results.

Exit 0: approval verified for supplied fields; 2: rejected approval/tuple/trust;
3: malformed or unsupported input; 4: incomplete observation; 5: output failure.
An exit-0 verification is not an all-clear for an actual upgrade.

## Statement and trust scope

The signature binds version, SHA-256 of exact network passphrase UTF-8 bytes,
checksum-validated owner contract address, raw tag as lowercase hex, old/new
32-byte hashes, ticket, reviewed Wasm artifact digest, inclusive ledger window,
and a 32-byte nonce represented as lowercase hex. The reviewed digest must equal
the target Wasm hash. This binds the claimed artifact; it does not prove an audit
or that anyone inspected those bytes. From and to must differ. Tags preserve
opaque bytes (including NUL/invalid UTF-8) through hex; they are never decoded as
text for identity. Numeric/time semantics are ledger based, with no wall-clock
expiry inference.

A trust configuration applies to one exact network hash, owner, and allowlisted
set of tags. It contains 1–32 Ed25519 public keys and a threshold from 1 through
the key count. A key ID is lowercase SHA-256 of the raw 32-byte public key. This
prevents one key being counted under several aliases. Duplicate keys or signer
IDs are rejected. Thresholds count distinct valid, active trusted keys. Public
keys and signatures require canonical padded standard base64.

Key activation and expiry are inclusive ledger bounds. revoked_at_ledger is
inclusive: the key is not trusted at or after that ledger; 0/omission means no
revocation. Rotation uses separate key records/windows. Revocation and trust roots
are operator assertions: verify public-key fingerprints out of band and protect
the trust file from unauthorized changes. A file supplied alongside an envelope
cannot bootstrap its own authority merely because verification succeeds.

## Exact signing representation

This is a custom domain-separated envelope, **not DSSE, in-toto, or RFC 8785**.
Ed25519 uses the standard Go crypto implementation. The bytes to sign are the
literal UTF-8 prefix `fleetwatch.approval.v1\n` followed immediately by compact
Go `encoding/json` serialization of the Statement struct, with no trailing
newline. Field order is:

```text
schema_version, network_passphrase_hash, owner_contract, tag_hex, from_hash,
to_hash, ticket, reviewed_artifact_digest, valid_from_ledger,
valid_until_ledger, nonce
```

All fields are present. Numbers are decimal uint32 values, strings use Go's
standard JSON escaping (including HTML characters and U+2028/U+2029). There are no
maps or floating-point numbers. Inputs must be valid UTF-8; hex is canonical
lowercase. Unknown fields, duplicate members (including case aliases), excessive
nesting, and input above 25 MiB are rejected by the shared strict decoder.
`testdata/approval-signing-vector.json` includes exact base64 signing bytes,
public key and signature. Implementations in other languages should verify this
vector byte-for-byte before generating envelopes. Statement digests in results
are SHA-256 of the complete domain-prefixed signing bytes.

The nonce is signed and mandatory, but this standalone verifier does not consume
nonces or bind them to durable incident IDs. Re-verification is deliberately
repeatable. Optional same-database nonce consumption and event association are now available
through bind-approval; see APPROVAL_BINDING.md. Global replay prevention, trustworthy
ledger evidence, policy integration and trust-store distribution require future
work before signed approvals can downgrade a finding. The format and protocol are experimental
and have not received an independent security audit.

JSON Schemas are in schema/signed-approval-v1.schema.json,
approval-trust-v1.schema.json and approval-observation-v1.schema.json. Runtime
validation additionally checks StrKey checksums, key fingerprints, scope, ledger
ordering, exact reviewed digest, unique signers and signature correctness.
