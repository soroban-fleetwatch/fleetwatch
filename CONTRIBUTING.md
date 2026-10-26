# Contributing

This is an early prototype. We welcome bug reports, fixture improvements and reviewed implementation PRs.

1. Read the README, `docs/SNAPSHOT_FORMAT.md` and `docs/THREAT_MODEL.md`.
2. Create a scoped issue with a problem description, acceptance criteria and test plan.
3. Branch from `main`, use focused commits, and add unit tests for all comparator changes.
4. Run `gofmt -w ./cmd ./internal`, `go test ./...`, and `go vet ./...`.
5. Do not introduce real credentials, network identifiers represented as synthetic examples, or claims that a fixture proves live-chain behavior.
6. Open a PR explaining risks, evidence sources, the feature and limitations.

Security vulnerabilities should follow SECURITY.md, not public issue reports.
