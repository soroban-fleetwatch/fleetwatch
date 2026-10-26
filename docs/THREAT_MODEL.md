# Threat model — preproduction prototype

## Assets and trust boundaries

**Protected interest:** visibility into which Soroban contracts in a watched fleet share an executable reference, and notification when the resolved code hash changes.

**Inputs:** (a) two normalized JSON snapshots and (b) an optional policy file. Both are operator supplied and potentially attacker controlled. The tool currently accepts them as provided and performs no authenticated on-chain retrieval.

**Attacker goals:** perform an undetected upgrade; induce false alerts or suppress true alerts; tamper with an approval file; remove an observation from a snapshot; exploit parsing to crash or consume resources.

## Checks present

- Schema and field validation; duplicate identifiers rejected.
- Byte-exact executable tags stored as hex; no lossy Unicode conversion.
- Exact approval tuple validation; incomplete approvals do not count.
- Missing references and contracts are marked as observation gaps, not on-chain deletions.
- Critical findings default to nonzero CLI exits for CI.

## Critical limitations

1. **Authenticity:** The JSON may be forged. These are comparisons, not authoritative claims about Stellar's consensus state.
2. **Completeness:** Snapshot captures may omit contracts, code owner keys, or intermediate ledger changes. An A→B→A transition between snapshots is invisible.
3. **Time and continuity:** No live subscription, checkpoint validation, or chain reorganization/rollback handling exists yet.
4. **Policy integrity:** A malicious editor can mark an unauthorized change as approved. Future approval sources need signatures and trusted keys.
5. **Authorization:** A change to the tag value is not proof the manager's privileged operation was unauthorized, and no actor attribution is available yet.
6. **False positives:** Snapshot coverage changes can produce misleading counts and gaps. Captures must be comparable.
7. **Security audit:** No external audit, threat-testing campaign, or production reliability benchmark has been conducted.
8. **DoS:** File-size and validation limits reduce but do not eliminate resource-exhaustion attacks. Input count and memory usage are not yet fully bounded.

## Safe use

Use against **synthetic fixtures or deliberately generated, authenticated test snapshots**. Do not market this version as a real-time chain surveillance or custody-protection product, or use it as the sole basis for production contract governance decisions.

## Later hardening acceptance criteria

- Verify ledger sequence and network passphrase from configured trusted source.
- Support exactly once or idempotent processing with replay/backfill on missed ledgers.
- Validate XDR envelope and field types, including opaque executable tag string bytes.
- Cross-check captured resolved Wasm against the owner contract's persistent tag entry at the same ledger.
- Track expected coverage, missing-fleet-member observability, endpoint divergence, and finalized checkpoints.
- Verify release approval with trusted signers, expiry and revocation checks, and immutable audit evidence.
- Add deterministic replay, fuzzing, malicious input corpus, rate-limit safeguards, and alert deduplication.
