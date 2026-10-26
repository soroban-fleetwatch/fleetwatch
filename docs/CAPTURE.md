# Experimental read-only capture

Build with Go 1.25+ and the pinned Stellar SDK v0.8.0. Supply a local watchlist
with one canonical Stellar contract address per line (maximum 50, no duplicates).
Synthetic fixture IDs are intentionally rejected.

```sh
/workspace/.tools/bin/fleetwatch capture \
  --rpc-url https://soroban-testnet.stellar.org \
  --network-passphrase 'Test SDF Network ; September 2015' \
  --contracts-file /path/to/watchlist.txt > /workspace/capture.json
```

Replace the watchlist path with an actual file. Inspect the exit status; a source
failure before evidence collection can leave an empty redirected file. This
command has not yet been validated against live Testnet: a public `getNetwork`
request was denied by the network proxy (HTTP 403). The required domain allowance
has been saved in the environment draft, but has not been verified at runtime.

The collector verifies the exact network passphrase and protocol >=28, samples getLatestLedger, then reads
contract instance keys, and resolves external references from persistent owner/tag
entries. SDK XDR types decode direct Wasm, Stellar Asset, and external references.
Returned data must match the requested key. Opaque tag bytes, including NUL and
invalid UTF-8, round-trip as hex. Evidence retains key/data XDR, SHA-256 of decoded
XDR, response ledger, and last-modified ledger. HTTPS certificate validation stays
enabled. URL credentials/query/fragment and redirects are rejected. Watchlist,
response, XDR, memory, recursion and request duration have explicit bounds.

Output is a diagnostic schema-v2 envelope, deliberately incompatible with v1
`compare`. Do not extract its nested snapshot to bypass assurance limits. A
COMPLETE single-response observation is an endpoint claim for the explicit
watchlist, labeled SINGLE_RESPONSE_CLAIM. Source authentication remains UNVERIFIED
and history completeness UNKNOWN. Missing/malformed entries yield PARTIAL.
External-reference resolution requires multiple calls: equal latestLedger fields
alone do not prove atomicity, so these captures stay PARTIAL with UNKNOWN ledger
consistency (INCONSISTENT if heights differ) and exit 4.

Exit 0 means the requested single-response observation was collected; 3 is invalid
configuration/watchlist; 4 is unavailable/wrong-network/partial source; 5 is output
failure. No transactions, private keys, or background service are used. Additional
divergence checks, verified atomic capture, protocol-30 compatibility,
real Testnet upgrade fixtures, history, alerts and signed approvals remain future
work. Mock TLS/XDR tests are synthetic evidence only.

## Freshness and bounded retries

Capture samples `getLatestLedger` before reading state, stores the claimed head,
and requires its close time to be no more than 60 seconds old or 10 seconds in
the future. Network and head protocol versions must agree. Entry responses below
the sampled head are partial. These checks compare endpoint assertions to the
local clock; they do not independently authenticate a ledger or prove liveness.
The HTTP client retries 429/502/503/504 at most twice, with 100/200 ms backoff.
Retry-After is honored only within a two-second budget; a longer or invalid
interval terminates the request. Waiting respects context cancellation. Other
HTTP errors, malformed RPC envelopes, and transport failures are not retried.

Latest live check: Testnet responded HTTP 403 with `error code: 1010` after the
domain was added to the draft. The failure now appears to come from the endpoint
edge; current data collection remains unverified. No duplicate secret or broader
network access is required based on this response alone.
