# Browser delivery validation — 2026-10-27

The user rejected supplying a Linux host or provider account and requires a
zero-cost deployment. Added a self-contained read-only foreground browser client,
not a GitHub Actions hosted daemon. Source uses the existing typed Go/XDR capture
adapter with a WASM Fetch transport: CORS, credentials omitted, redirect error.
Native transport/proxy behavior is unchanged.

## Actual checks

- Go 1.27.3 builds the self-contained JS/WASM package, about 4.8 MB HTML. The file
  embeds the runtime, gzip-compressed WASM and full linked-module license notices;
  no CDN scripts, account, key or backend are used. BUILD.json and SHA256SUMS record
  source identity and bytes. CI now cross-builds this target.
- `go test -timeout=60s -count=1 ./...` PASS. Source race test, native/WASM vet,
  module verify/tidy-diff, actionlint, JS syntax and Python syntax checks PASS.
- `node scripts/check-browser.cjs dist/browser-validation-2/FleetWatch.html` PASS:
  Chromium 154.0.8037.57, Playwright 1.62.1. Ten deterministic scenarios cover
  typed external references, assurance preservation, credentials omitted,
  redirect rejection before follow, raw JSON download, input rejection without
  requests, missing entry, malformed XDR, wrong network and stale values.
- Live browser UI/Go capture at **2026-10-27T19:14:03Z**, Testnet protocol 29,
  head/response ledger **5420960**, two contracts and one shared reference.
  Tag hex `666c65657477617463682d64656d6f`, owner
  `CCSC3UOXCCNB3VV7MM2P63IPNVKHQQ3TUMBCIW6OW42F75FBGMI4Y3VA`, observed hash
  `10c093251bf8e70fba073bdfea8868d6c5229397049a8be2aa880cb82c26eb28`.
  Three raw evidence records retained in [the capture](2026-10-27-browser-capture.json).
  Capture-file SHA256: `397af21ca5e262600544a9d728c332387c15584c330281b622f6830c1c4ef94e`.
- Actual HTTPS CORS preflight with Origin `null` returned 204, allow-origin `*`,
  POST and content-type allowed. Actual getNetwork POST with Origin `null`
  returned 200 and allow-origin `null`. POST using the intended Pages origin also
  returned 200 with that allow-origin. These are endpoint assertions, not a
  successful visit to a published Pages site.

## Test environment qualification

The managed Chromium profile blocks file URLs by administrator policy. Tests
therefore served the unchanged HTML on an allowed loopback HTTP test server;
no browser policy was disabled. Its legacy NSS store also lacks the managed
environment proxy CA. A direct browser RPC attempt failed certificate validation.
TLS verification was **not disabled**. For the live integration check only, a
Playwright route relayed identical RPC POSTs through an APIRequestContext with
the configured environment CA and normal proxy, with ignoreHTTPSErrors=false.
The real browser executed the UI, WASM and typed decoder against those actual
responses. This is a TLS-verified harness-relayed live check, not proof of direct
browser TLS in this managed profile or of a deployed public origin.

Current-state capture stays PARTIAL because multi-call atomicity is unproven.
Authentication UNVERIFIED, history UNKNOWN, verdict UNDETERMINED. No transaction,
key, external receiver, provider resource or account was created. Observations
last only for the open visible session and cannot prove transient history.

Git read/push succeeds. Normal api.github.com access fails proxy CONNECT 403;
unauthenticated repo/Actions pages return 404. Repository visibility, remote CI,
Pages activation and a public URL are not verified. Initial configure-pages
enablement requires a suitable admin token, unlike ordinary page deployments.
No credentials were extracted and no network denial bypassed.

This is a distributable no-host option with tested client behavior. It does not
complete the Dripwave production hosting gate or the native daemon deployment.
