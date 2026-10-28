## What changed

<!-- One or two paragraphs. Link the issue: Closes #N -->

## How it was tested

<!-- Exact commands and results. CI must pass. -->

- [ ] `go test -count=1 ./...`
- [ ] `go test -race -count=1 ./...`
- [ ] `go vet ./...` and `gofmt -l cmd internal scripts` is empty
- [ ] New failure cases are tested, not only the success path

## Evidence for chain claims

<!-- Testnet tx hashes/ledgers, or "none: offline only". Do not present synthetic data as chain observations. -->

## Assurance and limitations

<!-- Does any UNVERIFIED / NOT_VERIFIED / UNKNOWN / UNDETERMINED label change? What remains unproven? -->

- [ ] No secrets, private keys, tokens or webhook URLs committed
- [ ] `docs/build/STATUS.md` updated if this advances a milestone
