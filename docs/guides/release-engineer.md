# Guide: release engineer

**Goal:** confirm that an observed code change is the one you approved.

FleetWatch offers two separate checks. Do not treat the first as the second.

## Unsigned policy file

A JSON list of approved old→new hash transitions per owner and tag. Useful in CI.

```sh
./fleetwatch compare --before testdata/before.json --after testdata/after.json \
  --policy testdata/approved-policy.json
```

The finding becomes `POLICY_MATCHED_REFERENCE_CHANGE` (Info) and the command
exits 0. Authorization stays `UNVERIFIED`: anyone who can edit the file can
add a match.

## Signed approval

An Ed25519 statement that binds network, owner, tag, the exact old and new
hash, a reviewed artifact digest, a ledger validity range and a nonce. It is
checked against trust roots you control, with a signer threshold, revocation
and rotation.

```sh
./fleetwatch verify-approval \
  --approval testdata/signed-approval.json \
  --trust testdata/approval-trust.json \
  --observation testdata/approval-observation.json
```

Output from the public synthetic vector:

```json
{
  "policy_match": "YES",
  "signature_verified": "YES",
  "signer_trusted": "YES",
  "approval_valid_for_ledger": "YES",
  "verification_status": "VERIFIED",
  "final_verdict": "UNDETERMINED",
  "reason": "APPROVAL_VERIFIED_FOR_SUPPLIED_FIELDS"
}
```

`VERIFIED` means the signers approved those exact fields. The final verdict
stays `UNDETERMINED` because the observation itself is not authenticated.

To record that an approval was used for a specific persisted incident, and to
stop the same nonce being used again in that database:

```sh
./fleetwatch bind-approval --state replay.db --network NETWORK \
  --event-id EVENT_ID --approval envelope.json --trust trust.json
```

Formats and rules: [Signed approvals](../APPROVALS.md),
[Approval binding](../APPROVAL_BINDING.md).

## Suggested release flow

1. Build the new Wasm and record its hash.
2. Signers produce an approval for `owner, tag, old hash → new hash` with a
   short ledger validity window and a fresh nonce.
3. Publish the upgrade.
4. FleetWatch reports the reference change. Verify the approval against that
   finding's fields and bind it.
5. Any change without a matching verified approval needs investigation.

Binding signed approvals directly to `follow` findings is planned work.
