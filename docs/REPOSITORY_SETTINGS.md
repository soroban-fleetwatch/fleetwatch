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

After the repository is public: Settings → Pages → Build and deployment →
Source: **GitHub Actions**. Then run the "Docs site" workflow from the Actions
tab, or push to `main`. The workflow skips deployment while the repository is
private.

## Topics

Settings → General → Topics (About panel):

`stellar`, `soroban`, `cap-85`, `smart-contract-security`, `monitoring`,
`blockchain-security`, `golang`, `upgrade-safety`

## Visibility

The repository is currently **private**: the GitHub API reported
`"visibility": "private"`, no topics and `has_pages: false` on 2026-10-28.
Open-source programs such as Drips Wave review public repositories, and the
README badges and contributor image do not render for logged-out visitors of a
private repository. Making it public is an owner decision. Before doing so,
confirm that no file contains secrets (`git log -p` review or a secret scan),
because history becomes public too.

## Release tag

`v0.1.0-rc.1` was tagged on 2026-10-28 with notes in
[release/v0.1.0-rc.1.md](release/v0.1.0-rc.1.md). To show it on the Releases
page: Releases → Draft a new release → choose the tag → paste the notes →
tick "Set as a pre-release" → Publish.

FleetWatch has no product contract. The release notes should list the
**Testnet fixture** contracts from
[the validation record](research/2026-10-27-testnet-validation.md) as test
fixtures only, not as deployed product addresses.

## Labels and issues

Issues #3–#22 were created on 2026-10-28. GitHub created their labels in its
default grey. `scripts/create-issues.sh --apply` (with an authenticated `gh`)
recolors the labels and skips the existing issues.
