# Contributing to FleetWatch

FleetWatch is an experimental security monitor. A wrong "all clear" is worse
than no answer, so every change needs tests, and every claim about chain
behavior needs recorded evidence.

## Before you start

1. Read [README.md](README.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
   and [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md).
2. Pick an open issue, or open one first. Issues carry `complexity:S|M|L` and
   `type:*` labels. Comment on the issue before starting so work is not duplicated.
3. Security problems go through [SECURITY.md](SECURITY.md), never a public issue.

## Development setup

- Go: the version in `.go-version` (currently 1.27.3). The module language
  floor is Go 1.25. CI runs with `GOTOOLCHAIN=local` and `GOFLAGS=-mod=readonly`.
- Optional, only for the Testnet fixture contracts: Rust 1.99.1 with the
  `wasm32v1-none` target and Stellar CLI 28.1.0. See
  [docs/TESTNET_DEMO.md](docs/TESTNET_DEMO.md).
- Optional, only for the docs site: Python 3 and `pip install --require-hashes -r docs/requirements.txt`.

No secrets, wallets or network access are needed for the default test suite.

## Checks to run before a pull request

These match the `test` job in [.github/workflows/ci.yml](.github/workflows/ci.yml):

```sh
go mod verify
go mod tidy -diff
test -z "$(gofmt -l cmd internal scripts)"
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./...
go build -trimpath -o /tmp/fleetwatch ./cmd/fleetwatch
```

The `vulnerabilities` job runs `./scripts/check-vulnerabilities.sh`. It needs
network access to `vuln.go.dev`. Docs changes should also pass
`mkdocs build --strict`.

## Rules for changes

- Keep source adapters (RPC, files) separate from pure comparison and
  reconstruction code.
- Preserve CAP-85 tag bytes exactly. Do not decode, trim or case-fold them.
- Never report an unsigned policy match as a signed approval.
- Partial or missing observations must produce a gap or `PARTIAL` result, not a
  clean verdict. Current-state RPC reads are not history.
- New input formats need bounds (size, depth, count) and a JSON Schema in `schema/`.
- Do not commit private keys, tokens, webhook URLs or real signing material.
  Test vectors must be synthetic and labeled as such.
- Fixtures that are not real chain data must say so. Do not present synthetic
  data as Testnet or Mainnet observations.
- No Mainnet writes. Testnet writes only for controlled fixture deployments.

## Commits and pull requests

- Branch from `main`. Use [Conventional Commits](https://www.conventionalcommits.org/)
  (`feat(follow): ...`, `fix(cli): ...`, `docs(site): ...`).
- Keep commits atomic and stage specific paths.
- Fill in the pull request template: what changed, how you tested it, evidence
  for any chain claim, and the remaining limitations.
- Update `docs/build/STATUS.md` when a change completes or advances a milestone.
- All CI jobs must pass. Maintainers may ask for extra failure-case tests.

## License

By contributing you agree that your contributions are licensed under the
Apache License 2.0, as described in [LICENSE](LICENSE).
