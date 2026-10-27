# Automatic follow outbox sync validation — 2026-10-27

Opt-in follower flags now run local queue sync at startup and between nonfatal
polling cycles. The default follower and default service unit keep queue sync
disabled. No sender, receiver, HMAC key or external notification is enabled by
this feature.

The test injects a sync failure after a native evidence commit, checks
OUTBOX_SYNC_FAILED/not-ready while retaining findings, then closes/reopens native
state and repairs the outbox at startup before a healthy cycle is emitted.
The worker subsequently stops gracefully. Independent tests cover temporary
dispatcher lock contention/retry, cancellation, startup sync failure, unchanged
sync without outbox byte changes, offline flag validation and path collisions.

The separately invoked captured-data test reused baseline 5407791 and twelve
previously captured Testnet ledgers through 5407803. It recovered all three
findings across the simulated sync-failure window. **The RPC adapter/head in
this test is synthetic**: this is offline replay of real metadata, not a new
live RPC/freshness or hosted readiness measurement. No new chain request,
transaction or external receiver call occurred. Test report explicitly says
real_source_freshness_verified=false and final verdict UNDETERMINED.

Invocation: provide FLEETWATCH_FOLLOW_AUTOSYNC_BASELINE and
FLEETWATCH_FOLLOW_AUTOSYNC_CAPTURE as absolute paths to the existing transition
baseline/history files in testnet-runs/live-validation-2, set the exclusive new
FLEETWATCH_FOLLOW_AUTOSYNC_REPORT path, and run `go test -race -count=1
./internal/followdelivery -run
'^TestAutomaticSyncBeforeReadinessAndCrashWindowRecovery$'`. Default tests use
deterministic synthetic fixtures. Raw report stays ignored under
follow-runs/autosync-validation-1; the matching JSON file preserves the result.

Local default suite: 242 top-level tests, six fuzz seed targets and four
intentional skips PASS. Affected followdelivery/follow/store/alert/CLI race
suites, vet, formatting, module verification/tidy-diff, module/source-test/binary
vulnerability gates, workflow/Bash/schema checks and package/install smoke PASS.
Both default and optional sync unit syntax are statically verified. Remote CI,
actual target-host unit startup, logout/boot behavior and production sizing are
unverified; no service was started or enabled by installation.

Native and outbox commits are separate transactions. Tests simulate an
interruption window and confirm persisted-prefix repair; this is not a host
crash/durability certification. Queue readiness does not mean receiver delivery.
Two-minute sync timeout or lineage/storage failure stops safely with retained
native evidence. External dispatch scheduling, gap-notification policy,
production proxy-aware receiver transport, archive chaining/retention rollover
and permanent-host validation remain open. Source/baseline UNVERIFIED,
commitments NOT_VERIFIED, coverage UNKNOWN and verdict UNDETERMINED remain.
