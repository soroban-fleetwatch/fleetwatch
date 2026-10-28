# Contributing

The full guide is
[CONTRIBUTING.md](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/CONTRIBUTING.md)
in the repository. In short:

1. Pick or open an issue. Planned work is listed in
   [`docs/build/issues.json`](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/docs/build/issues.json)
   with complexity and type labels.
2. Set up Go from `.go-version`. No keys or network access are needed for tests.
3. Before a pull request, run:

    ```sh
    go vet ./...
    go test -count=1 ./...
    go test -race -count=1 ./...
    test -z "$(gofmt -l cmd internal scripts)"
    ```

4. For docs changes, build the site:

    ```sh
    python3 -m venv .venv
    .venv/bin/pip install --require-hashes -r docs/requirements.txt
    .venv/bin/mkdocs build --strict
    ```

5. Use Conventional Commit messages and fill in the pull request template.

Rules that matter most: keep tag bytes exact, never report a policy match as a
signed approval, never turn a partial observation into a clean result, and never
commit keys or present synthetic data as chain data.
