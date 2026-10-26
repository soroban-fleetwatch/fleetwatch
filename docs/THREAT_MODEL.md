# Threat model — preproduction prototype

## Assets and trust boundaries

FleetWatch tracks observed executable associations for an explicit watched fleet
and retains evidence, findings, delivery state and supplied approval receipts.
Inputs include operator snapshots/baselines/configuration/trust files, HTTPS RPC
claims and ledger-close metadata, a local bbolt database and a configured webhook
receiver. Operator-controlled file paths and RPC destinations are trusted local
configuration, not an unauthenticated hosted API.

An attacker may forge source data, omit an intermediate change or fleet member,
modify metadata outside a valid embedded header, supply ambiguous JSON/XDR,
exhaust bounded resources, alter approval material, redirect outbound requests,
or induce duplicate delivery through lost acknowledgments. A party able to
rewrite the entire local database can also replace its evidence and consistency
hashes. Local hashes/receipts do not provide independent provenance.

## Checks present

- Shared operator JSON decoding bounds bytes/nesting/object members and rejects
  duplicate/case-aliased/unknown fields. [RPC JSON guards](RPC_JSON.md) reject
  structural ambiguity while permitting nonconflicting provider extensions.
  Failed RPC methods expose zero values and fixed errors.
- Typed Stellar XDR decoding validates executable variants, persistent reference
  keys, address/hash forms and opaque tags. Tags retain exact bytes as lowercase
  hex. Capture rejects inconsistent claimed ledger heights and unresolved inputs;
  missing observations are not presumed on-chain deletions.
- Bounded sequential metadata collection checks exact range/order, header hashes
  and previous links. Watched reconstruction applies asserted baseline state with
  operation/preimage checks and rejects unsupported or ambiguous mutations.
  Raw retained evidence is rederived on checkpoint inspection and cold resume.
- Checkpoints/findings/outboxes commit atomically. Resume checks the expected
  cursor after fetching; cancellation, conflict and commit-failure tests exercise
  rollback. A finite [worker](WATCH.md) has explicit attempt/runtime/retention
  limits and context-aware waits. Its counters are local observations, not proof
  of current source health.
- Standalone Ed25519 approval verification checks exact statements, validity,
  signer trust/thresholds and revocation. Optional snapshot incident binding
  consumes nonces within the retained local DB history. Unsigned policy matches
  remain unverified; signatures do not authenticate the underlying chain input.
- Explicit [webhook delivery](WEBHOOKS.md) signs exact request bytes with HMAC,
  binds a sink, leases attempts with fencing tokens, bounds retries and requires
  receiver-side deduplication. Its strict public-destination HTTPS transport fails
  closed for unsupported proxy delivery. RPC transports retain inherited proxy/CA
  routing and disable redirects/URL credentials.
- [Manual recovery](ALERT_RECOVERY.md) preserves payload/ID/sink and atomically
  records bounded local receipts. Reconstruction health checks their consistency.
  [Backups](BACKUP.md) preserve all namespaces in one transaction snapshot.

## Remaining limits

1. Baseline/source authentication remains UNVERIFIED; metadata commitment is
   NOT_VERIFIED. Valid embedded headers and local metadata hashes do not prove
   that all metadata belongs to an independently authenticated ledger. There is
   no consensus/SCP or full transaction/state commitment validation.
2. Fleet membership and chain history completeness remain UNKNOWN. Snapshot
   polling can miss A→B→A. Projection covers only the explicit asserted watchlist
   and supported operation-boundary cases, not every lifecycle or network member.
3. Durable reconstruction remains limited to 16 ledgers beyond the original
   baseline, 25 MiB/stream and 128 combined scopes. Outboxes and recovery audits
   have separate bounds. There is no production retention/migration, continuous
   unbounded monitor, Postgres service or independently measured live lag.
4. Delivery is at least once. A receiver may accept a request whose acknowledgment
   is lost. Restoring an older DB can forget later delivery, recovery and nonce
   consumption; distributed coordination and rollback reconciliation remain open.
5. Operator trust distribution, authenticated chain-policy integration and actor
   attribution are unfinished. Severity from unverified input is not proof of a
   malicious or unauthorized on-chain operation. Final chain verdicts remain
   UNDETERMINED, including when local health reports OK_OFFLINE.
6. Parser/resource/fuzz/race and local TLS/crash tests are internal evidence.
   They are not an independent audit. Dependency vulnerability scanning,
   representative fleet/load measurements, production availability/backup
   validation and controlled live CAP-85 upgrade proof remain unfinished.

Use the prototype for explicit diagnostic evidence and reproducible tests.
Source/coverage labels and retained-prefix limits must accompany findings used
for investigation. It is not a production governance or custody safety verdict.
