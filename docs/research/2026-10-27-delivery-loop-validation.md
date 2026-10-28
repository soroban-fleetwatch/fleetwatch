# Follow dispatcher loop validation — 2026-10-27

The optional deliver-loop repeatedly sends imported follow alerts to an explicit
receiver with the existing signed transport. It is separate from the collector
and automatic queue sync, and is never started by installation. This checkpoint
does not configure or contact an external receiver.

The local verified-TLS loop test used the previously captured Testnet transition
baseline/history under testnet-runs/live-validation-2. A temporary native follow
store applied that evidence; the shared bridge imported its three findings into
a separate outbox. The dispatcher sent three requests to a fixed loopback test
receiver. It verified exact-body HMAC, schema-3 payload identity/idempotency and
UNDETERMINED assurance. Each status callback could reopen the outbox, confirming
the dispatcher released it between batches. Restart attempted no acknowledged
alert again, and cancellation emitted STOPPED. External deliveries and new chain
requests: zero. This is captured-data replay/local transport validation, not a
new live freshness, production receiver or hosted service test.

Invocation set FLEETWATCH_DELIVERY_LOOP_BASELINE and
FLEETWATCH_DELIVERY_LOOP_CAPTURE to absolute paths of the existing transition
baseline/history files, selected an exclusive new FLEETWATCH_DELIVERY_LOOP_REPORT
path, then ran `go test -race -count=1 ./internal/alert -run
'^TestDeliveryLoopLocalTLSSignsAndAcknowledgesAcrossRestart$'`. Default tests
use deterministic synthetic metadata. Raw report is ignored under
follow-runs/dispatch-loop-validation-1; the matching JSON preserves its counts.

Synthetic tests cover transient retry, permanent dead letters retained across
restart, waiting on missing/busy/unimported/empty state, no creation or empty-file
initialization, corrupt-state refusal, in-request cancellation, output failure
after durable acknowledgement, CLI bounds/missing keys and no state creation on
invalid invocations. The dispatcher uses a new writable existing-only store
opener that strips file creation flags, retaining existing scope recognition.

Local full suite: 247 top-level tests, six fuzz seed targets, four intentional
skips PASS. Affected alert/store/followdelivery/follow/CLI race suites, vet,
formatting/module verification/tidy-diff, module/source-test/binary advisory
gates, workflow/Bash/schema and package checks PASS. Static verification includes
the optional delivery user-unit template. It requires an explicitly provisioned
private receiver/key environment file; none was generated and no service was
linked, enabled or started. Remote CI, actual unit/boot/hardening, production
proxy routing, receiver authorization/setup and representative load remain
unverified. Existing sender proxy/public-address/TLS restrictions are unchanged.

Dispatch is at least once; ambiguous acknowledgements require receiver-side
deduplication and durable row state, not assumptions from process logs. No dead
letter is automatically requeued. Status counts describe transport/outbox state,
not chain safety or complete fleet coverage. Retention rollover/archive chaining,
source-gap notifications and permanent-host validation remain open; all existing
source/baseline/commitment/coverage/approval uncertainty remains.
