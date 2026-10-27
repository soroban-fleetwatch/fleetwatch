# Zero-cost deployment

FleetWatch now has a **self-contained browser edition**. A Linux machine,
hosting account, receiver and key are not required to use it. Download the
published HTML and open it, or serve it on a static site. Instructions and
actual topology are in [BROWSER.md](BROWSER.md).

Go WebAssembly reads the fixed Stellar Testnet RPC directly while the page is
open and visible. It has no paid service, server/database dependency, signup or
background CI compute. Normal Internet/device costs remain those of the person
opening it. It does not provide unattended ledger-by-ledger monitoring, a durable
alarm queue or complete history. Native follow evidence/outbox state is untouched.
Its portable Linux deployment remains in [OPERATIONS.md](OPERATIONS.md), but a
host is no longer the only way to run FleetWatch end-to-end.

GitHub Pages is an optional static URL. Standard Pages is free for public repos;
private-repo availability depends on the existing plan. Do not change visibility
or buy a plan to make deployment green. A gh-pages branch alone does not prove
Pages is enabled. Initial activation requires repository Pages configuration;
the built-in Actions token cannot auto-enable a new site via configure-pages.
Check the actual URL before claiming a live deployment.

Here, normal Git read/push works. GitHub API calls to api.github.com are denied
at managed-proxy CONNECT. Unauthenticated repo web requests return 404; visibility
and Pages settings are unknown. This does not prove the token invalid. No Git
proxy credential is extracted, no policy bypassed and no public URL invented.
The downloadable client avoids that hosting prerequisite.

Actions builds and tests software; it is **not** a perpetual production host.
Official terms restrict serverless application hosting. Schedules are default
branch only, can be delayed/dropped and disable after inactivity. Private usage
can exceed included quotas. No scheduled daemon was created.

Official sources checked live on 2026-10-27:

- [Pages publishing sources](https://github.com/github/docs/blob/main/content/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site.md)
- [configure-pages enablement/token requirements](https://github.com/actions/configure-pages/blob/main/action.yml)
- [Actions product terms](https://github.com/github/docs/blob/main/content/site-policy/github-terms/github-terms-for-additional-products-and-features.md#actions)
- [Actions billing](https://github.com/github/docs/blob/main/content/billing/concepts/product-billing/github-actions.md)

The runnable browser package and a completed public hosting gate are separate
claims. See [Phase 9 acceptance](PHASE9_ACCEPTANCE.md) for precise status.
