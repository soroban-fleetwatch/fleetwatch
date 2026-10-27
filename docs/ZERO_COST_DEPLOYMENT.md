# Intended deployment and zero-cost constraint

FleetWatch is a native off-chain collector. Its intended Phase 9 topology is:
read-only Stellar RPC → one follower → persistent local evidence/projection DB →
CLI and loopback operational status. Optional notification delivery adds a local
outbox and authorized receiver. No frontend or GitHub Pages dependency is needed.
See [OPERATIONS.md](OPERATIONS.md) and [acceptance](PHASE9_ACCEPTANCE.md).

The owner reports that Pages is unavailable on the current GitHub plan. Do not
recommend an upgrade, change repository visibility, or treat a pushed gh-pages
branch as an active site. The optional browser viewer is foreground current-state
capture; it does not replace the native follower, retained history or outbox.

A development environment publication is also separate from deploying a service.
Publication saves development setup/filesystem state; it does not make this
workspace a permanent server or preserve running processes. The generic failed
publication/status message has no retained causal diagnostic. See the verified
[environment repair](research/2026-10-27-environment-repair.md).

## Current deployment boundary

Current runtime observations are connected/enforced, but there are no deployment
capabilities, outbound provider identities, secret/runtime bindings or provider
connections in the supported identity manifest. Neither the repo nor the uploaded
package contains a provider project or authorized remote target. Git read/push
works; that authority does not allocate permanent compute or persistent storage.

A zero-cost managed target can avoid asking the owner to connect a Linux machine,
but it still requires an authorized provider account/resource and verified cost
conditions. No such target is currently connected. Scripts cannot manufacture
that authority. Do not substitute an expiring anonymous preview, ephemeral CI
runner or temporary tunnel for permanent deployment.

No account, billing instrument, trial, paid resource, repository visibility change
or external receiver was created. Existing native evidence and browser files are
preserved. A host being called free does not prove ongoing capacity or zero
charges for this workload. Current 64–256 MiB retention limits also stop the
collector at capacity; production retention remains a separate acceptance item.

## Live official checks on 2026-10-27

- [Fly trial](https://github.com/superfly/docs/blob/main/about/free-trial.mdx):
  two total VM hours or seven days, with trial machines stopped after five minutes.
  [Ongoing resources](https://github.com/superfly/docs/blob/main/about/pricing.mdx)
  are billed. This is not a permanent zero-cost deployment.
- [Railway pricing](https://github.com/railwayapp/docs/blob/main/content/docs/pricing.md)
  describes a Free plan with $1 monthly resource credit; the
  [plan limits](https://github.com/railwayapp/docs/blob/main/content/docs/pricing/plans.md)
  include 0.5 GB RAM and volume. Account eligibility, actual workload fit and
  zero-overage behavior are unverified here. No deployment is claimed.
- [Railway anonymous trials](https://github.com/railwayapp/docs/blob/main/content/docs/pricing/free-trial.md)
  are temporary; unclaimed boxes and files are deleted. Permanent ownership needs
  signup. That does not satisfy this service's persistence requirement.
- Official Oracle and Render pages were denied by the managed proxy. Their current
  account eligibility and usable free allocations were not verified.

There is no claim that free hosting is impossible in general. The precise blocker
is that no authorized, usable permanent zero-cost target is available through the
session's existing connections. Phase 9 is not complete from a manifest or local
package alone.
