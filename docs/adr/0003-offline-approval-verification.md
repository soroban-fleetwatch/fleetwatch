# ADR 0003: separate cryptographic approval checks from chain verdicts

Date: 2026-10-26. Status: experimental offline implementation.

Add a standalone, bounded Ed25519 verifier for exact network/owner/tag/from/to
statements. Scope operator trust roots explicitly, require fingerprinted distinct
signers and configurable thresholds, and evaluate activation/revocation against
the supplied ledger. Use a versioned fixed-field JSON representation with a
signature domain prefix and a public compatibility vector. This initial custom
format is not DSSE or a generic JSON canonicalization implementation; no external
interoperability claim is made.

Keep unsigned policy matching unchanged. The current observation sources cannot
authenticate chain history, so final_verdict remains UNDETERMINED even when the
approval's signature/trust checks succeed. Partial input cannot yield VERIFIED
approval evaluation. There is no signing operation or production key in the repo.
Nonce consumption, durable event binding, signed trust distribution and verified
chain-evidence integration are deferred. Tests cover the cryptographic boundary,
not on-chain authorization, source consensus, or an independent security audit.
