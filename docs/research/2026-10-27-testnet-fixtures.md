# CAP-85 fixture preparation — 2026-10-27

The user authorized the Testnet validation phase after merging PR #1. Native
`git fetch origin main` confirms main at `0014aaa`, merge of that PR; the new
branch is `feat/testnet-cap85-demo`. Its initial tree matches the merged work.

Inspected current official sources:

- CAP-85: https://github.com/stellar/stellar-protocol/blob/master/core/cap-0085.md
- Published SDK 29.0.1: https://crates.io/crates/soroban-sdk/29.0.1
- SDK archive: https://static.crates.io/crates/soroban-sdk/soroban-sdk-29.0.1.crate
  (checksum `6a3b97a76c88f413859bb670903b63af54d7ddb03303a5bd585fee8b9f2a3d8f`).
- Official SDK repository: https://github.com/stellar/rs-soroban-sdk
- Rust manifest: https://static.rust-lang.org/dist/channel-rust-1.99.1.toml
- CLI release: https://github.com/stellar/stellar-cli/releases/tag/v28.1.0

The SDK stable version is 29.0.1 (minimum Rust 1.91); main is already a 30.0.0
release candidate. Inspected the published stable archive rather than building
against main. Verified `Env::executable_refs().set`,
`DeployerWithAddress::deploy_contract(ContractExecutable::ExternalRef(...), ...)`
and `Deployer::update_current_contract(ContractExecutable::Wasm(...))` in that
archive. `Address::executable()` resolves to the current Wasm; it does not itself
prove the underlying instance remains an external reference. The fixture test
checks observable behavior after a shared update and a direct detachment.

Installed Rust 1.99.1 components from official distribution archives, checking
each component's SHA-256 against the official manifest before extraction/install.
Installed the official CLI release archive over verified HTTPS and pinned its
observed SHA-256 in CI. Cargo resolved a lockfile and verified downloaded registry
checksums. No third-party audit or Rust advisory scan is claimed.

Three actual-Wasm host tests pass. Repeated build/check runs reproduce these
artifact hashes in this environment:

| Artifact | SHA-256 |
| --- | --- |
| member-v1.wasm | `8d68f7ede8dd8656c8068736054f8991e2f29a8ff6d1215a5116875f7ea8987d` |
| member-v2.wasm | `10c093251bf8e70fba073bdfea8868d6c5229397049a8be2aa880cb82c26eb28` |
| owner.wasm | `9afd0a5908e150d2060fa0c742e6dceb5dd5662bb07391a8ca166a7bf55cef6e` |

These are local build outputs, not deployed contract IDs or transaction hashes.
No cross-machine reproducibility claim is made.

FleetWatch's Go client reached the public Testnet RPC, confirmed exact passphrase
and protocol 29, and decoded protocol-29 ledger-close metadata version 2. See
`2026-10-27-testnet-preflight.json` for the actual sampled sequence and digests.
This proves structural read compatibility with that endpoint response, not
consensus or metadata-commitment authentication.

Python HTTPS requests to the RPC returned HTTP 403. The prebuilt CLI returned
transport Connect errors. Go's inherited proxy and OS CA configuration succeeded;
no proxy bypass or TLS-verification disablement was used. Source inspection
traced CLI RPC to stellar-rpc-client 28.0.0 and jsonrpsee-http-client 0.26.0:
its transport constructs a direct Hyper connector instead of using the managed
HTTP proxy. Altering reqwest certificate features would not fix this RPC path,
so an exploratory native-root CLI build was stopped and is not used.

Added a developer-only loopback RPC adapter using Go's inherited proxy and
verified upstream TLS, fixed Testnet destination, strict request parsing/method
allowlist and size/time bounds. An actual official-CLI health call through it
returned healthy at ledger 5407609. No signing or sendTransaction was exercised.
The live smoke starts/stops this adapter; read-only FleetWatch capture still
connects through its own existing HTTPS reader. This is one source endpoint,
not independent chain authentication. Invalid/duplicate/unknown requests and
browser-origin requests are rejected in an offline transport test.

The RPC's getNetwork advertises `https://friendbot.stellar.org/`. A Go funding
request for a newly created disposable test identity failed with proxy Forbidden.
No successful funding, contract deployment or upgrade was observed. The unused
identity file was removed; its secret was never printed or tracked. A saved
environment draft adds the funding hostname while preserving existing domains.
Publication and a successful retry are still required.
