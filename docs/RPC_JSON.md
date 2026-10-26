# RPC response JSON guards

All four HTTPS RPC methods (`getNetwork`, `getLedgerEntries`, `getLatestLedger`,
`getLedgers`) now validate the whole response document before interpreting typed
fields. This protects current capture, history collection, ingestion and watch.
It does not authenticate source claims or Stellar consensus.

Go's standard `encoding/json` decoder accepts duplicate object keys, uses
case-insensitive struct-field matching and can partially fill a target before a
type error. Its Unmarshal documentation describes those behaviors:
https://go.dev/src/encoding/json/decode.go . FleetWatch adds these boundaries:

- At most 25 MiB of response body, read with a limit-plus-one guard.
- Valid UTF-8 bytes, one JSON document, bounded nesting (64 levels), and no null root.
- No duplicate members, including pairs differing only by case, anywhere in nested result objects
  and ignored provider extensions. Escaped keys are compared after JSON decoding.
- At most 10,000 members in each object, bounding the duplicate-key set. This
  shared bound also applies to operator input decoded by `jsoninput`.
- Canonical envelope names `jsonrpc`, `id`, `result`, `error`. Case aliases of
  reserved names are refused, even without a canonical counterpart.
- Exact version `2.0` and matching numeric, nonnull request ID; an object result
  is required. Nonnull RPC errors are refused, including an error alongside a
  result. An optional `error:null` remains accepted for provider compatibility.
- A failed public RPC method returns its zero value and a fixed error, never a
  partially decoded record or raw upstream error details.

Providers may add nonconflicting envelope/result fields such as `friendbotUrl`.
Result-field interpretation retains the existing typed decoder; this is not a
closed provider schema. Unknown extension content still passes the structural
checks before it is ignored. Operator-controlled snapshots, baselines, approval
files and watch configurations continue to reject unknown fields. The new
extensible reader is used only for provider responses.

Malformed responses do not advance a cursor past rejected evidence. If earlier
ledgers were accepted, ingestion can retain that prefix and a diagnostic. RPC
JSON failure reaches the collector as a fixed NETWORK_QUERY_FAILED or
LEDGER_QUERY_FAILED code. The bounded worker may retry those query failures;
it does not treat the rejected response as verified or complete. Source field
and XDR validation still occur after JSON decoding.

The guards reject invalid UTF-8 bytes. JSON escape decoding otherwise follows
Go's string rules; no new Unicode normalization or raw tag conversion is
introduced. Executable tag identity continues to use validated lowercase hex and
original XDR bytes.

Reproduce the parser checks after activating the development Go environment:

```sh
go test ./internal/source ./internal/jsoninput
go test ./internal/source -run '^$' -fuzz '^FuzzRPCJSONResponse$' -fuzztime=30s -parallel=2
GOMAXPROCS=2 go test ./internal/source -run '^$' \
  -bench '^BenchmarkRPCJSONMetadataSizedResponse$' -benchtime=3x -benchmem
```

The benchmark is an RPC network result with a synthetic 2 MiB string extension;
it measures response parsing, not XDR decoding, full ingestion or service capacity.
On this cloud instance (Go 1.27.3, linux/amd64, AMD EPYC 9V74, GOMAXPROCS=2), three
iterations measured 46,329,675 ns/op, 45.27 MB/s, 25,697,389 B/op and 131 allocs/op.
Other tests were running on the instance. B/op is total allocation per operation,
not peak resident memory; this small sample establishes no throughput SLA or
worst-case resource bound. The 30-second fuzz exercise completed 171,088
executions without failure; this is not an exhaustive security review.

These checks reduce parsing ambiguity and constrain input work. Authenticated
baseline/chain/metadata commitments, production retention, dependency scanning,
representative load tests and an independent security audit remain unfinished.
