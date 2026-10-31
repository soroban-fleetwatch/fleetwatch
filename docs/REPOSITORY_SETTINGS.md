# Repository settings checklist

These settings live on GitHub, not in the repository files. Only a repository
administrator can apply them. None has been applied by an agent. Each row says
what to set and why.

## Branch protection for `main`

Settings → Branches → Add rule (or Rulesets), branch name pattern `main`:

| Setting | Value |
| --- | --- |
| Require a pull request before merging | On |
| Required approvals | 1 (raise when there are more maintainers) |
| Dismiss stale approvals when new commits are pushed | On |
| Require status checks to pass | On |
| Required checks | `test`, `vulnerabilities`, `testnet-fixtures`, `docs` |
| Require branches to be up to date before merging | On |
| Require conversation resolution | On |
| Allow force pushes / deletions | Off |

The check names are the job IDs in [.github/workflows/ci.yml](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/.github/workflows/ci.yml).
A check only appears in the picker after it has run once on the repository.
The `docs` job is new; merge it to `main` before marking it required.

## Security

- Settings → Code security → **Private vulnerability reporting: Enable.**
  [SECURITY.md](https://github.com/soroban-fleetwatch/fleetwatch/blob/main/SECURITY.md) points reporters there.
- Dependabot alerts: On.
- Secret scanning: On where the plan allows it.

## GitHub Pages (docs site)

Enabled with Source: **GitHub Actions**. The site is at
<https://soroban-fleetwatch.github.io/fleetwatch/> and redeploys on every push
to `main`, or when the "Docs site" workflow is run by hand.

## Topics

Settings → General → Topics (About panel):

`stellar`, `soroban`, `cap-85`, `smart-contract-security`, `monitoring`,
`blockchain-security`, `golang`, `upgrade-safety`

## Visibility

The repository is **public** (GitHub API, 2026-10-31). Before the switch, a
pattern scan of all branches' history found no private keys, tokens, Stellar
secret seeds or webhook URLs. Topics are set.

## Release tag

The `v0.1.0-rc.1` pre-release was published by the owner on 2026-10-31:
<https://github.com/soroban-fleetwatch/fleetwatch/releases/tag/v0.1.0-rc.1>.
Notes: [release/v0.1.0-rc.1.md](release/v0.1.0-rc.1.md). For later releases,
create the tag from Releases → Draft a new release on `main`.

FleetWatch has no product contract. The release notes should list the
**Testnet fixture** contracts from
[the validation record](research/2026-10-27-testnet-validation.md) as test
fixtures only, not as deployed product addresses.

## Labels and issues

Issues #3–#22 were created on 2026-10-28. GitHub created their labels in its
default grey. `scripts/create-issues.sh --apply` (with an authenticated `gh`)
recolors the labels and skips the existing issues.
