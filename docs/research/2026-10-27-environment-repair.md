# Environment reproducibility repair — 2026-10-27

The owner reports that GitHub Pages is unavailable on the current plan and asks
for the original native architecture. A browser/site is not a requirement for
FleetWatch's stateful collector. Neither a temporary preview nor an ephemeral
runner is a permanent deployment. Existing browser artifacts are preserved as
optional diagnostics, not substituted for Phase 9.

## What is established about Publish

Earlier attempts reported “publishing failed” and inability to confirm status.
The selected environment was unreachable; even basic shell execution failed.
It is now connected again at runtime specification 39, with current observations
and an enforced four-domain custom policy. No application code change was needed
for that reconnection. Retained diagnostic history is not_found with no records,
and status.failure is null. There is no failed publication operation/cause exposed
by the available tools. Do not claim the bootstrap change below repairs the
platform service or prevents future platform outages.

The six-domain draft is persisted but has not been applied. A saved draft is not
proof of published setup, restored filesystem or a deployed running application.
No supported publication/restart control is exposed to the agent.

## Confirmed reproducibility defects and repairs

- Inspected actual project root `/workspace/fleetwatch`, ordinary Git metadata,
  no symlink escape, origin `github.com/soroban-fleetwatch/fleetwatch`. Before
  repair, HEAD/remote feature branch were
  `ed3c3ff8377dfc0f20a5547a6266d3b7d512e059`; saved selection still chose `main`
  (`0014aaa`), so it did not select the current Phase 9 source. Declare the final
  checked source HEAD with explicit mount path `fleetwatch`. The old empty mount
  value is not itself proven to cause a platform error.
- Workspace discovery found one project; Cargo dependency Git entries and the
  empty protected workspace Git stub are not project checkouts. No user source,
  raw evidence, cache or ignored work was removed.
- The old bootstrap extracts a verified new Go archive over an old compiler
  tree on version mismatch. That can retain obsolete files. New versioned
  `scripts/setup-cloud.sh` validates a fresh staged archive/compiler first, then
  replaces the tree, with rollback on rename failure. It refuses symlinked or
  non-directory installations, retains TLS/checksum verification, uses an
  installation lock and leaves existing data untouched on checksum failure.
- Full bootstrap uses pinned Go, explicit writable caches, readonly modules,
  GOTOOLCHAIN=local, tidy-diff, native tests/build and a synthetic CLI smoke.
  Future setup instructions must describe current native follow/outbox behavior
  and the verified source checkpoint, not the old unmerged main or a Pages gate.

Neither defect is established as the cause of the reported publication failure.
The installed Go already matched 1.27.3; the old mismatch branch was not active.

## Executed validation

- Old saved bootstrap ran successfully against the current checkout; all default
  Go package tests, module verification and CLI build passed without source edits.
- Revised bootstrap: module verify, tidy-diff, all default native tests, trimmed
  CLI build, `version` and synthetic `validate` passed. Shell syntax passed.
- Fresh isolated upgrade trial downloaded official Go 1.27.3, verified the pinned
  SHA256 `65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40`,
  installed the fresh tree and removed a deliberately obsolete test file.
- Repeated selection of that exact compiler made no network request and preserved
  its inode. Mocked corrupt archive failed before replacement, preserving the old
  binary/sentinel and leaving no staged directory. These tests did not weaken the
  production checksum or TLS rules. The actual installed compiler was untouched.
- Injected replacement-rename failure and interruption immediately after moving
  the old tree both restored its original binary/sentinel. Unsafe file/symlink
  paths were rejected without downloading or altering their targets. This is
  rollback-tested staged replacement, not a crash-atomic two-directory swap.
- Portable Linux package checks passed for both ELF architectures/checksums,
  native installation/preflight and foreign-architecture rejection. ARM64 was
  inspected, not executed. Workflow lint and shell syntax passed.
- Overlay had approximately 30 GiB free during investigation. A roughly 10.4 GiB
  logical workspace, mostly caches/build outputs, does not establish a snapshot
  quota failure. No quota error was returned; no caches were deleted on that guess.

## Actual Phase 9 boundary

Runtime capabilities, provider identities, secrets and runtime bindings are empty;
the supported provider manifest has zero connections. Repo/package inventory has
no provider project or authorized permanent execution target. A Git connection
cannot allocate those resources. No paid service, account, credential, trial,
publicity change or external receiver was created.

The native collector still needs persistent single-writer local storage and an
unattended process. Hard retention caps also need a reviewed production strategy;
the current collector stops at capacity. Phase 9 therefore remains BLOCKED. The
saved configuration repairs development reproducibility, not these production
requirements. See [zero-cost boundaries](../ZERO_COST_DEPLOYMENT.md).
