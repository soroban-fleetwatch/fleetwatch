# Pinned webhook proxy validation — 2026-10-27

This checkpoint adds an explicit pinned-connect mode to deliver and deliver-loop.
Default behavior still rejects environment-selected proxy routes. The new path
retains local public-address checks, numeric CONNECT authority and original
receiver-hostname TLS. It does not grant remote network access or configure a
receiver. No external receiver/proxy was contacted in this validation.

The fixture suite exercises both HTTP and HTTPS proxies. Each records a numeric
CONNECT target and matching numeric Host, verifies proxy Basic auth, then tunnels
to a fixed loopback TLS receiver. Test-only DNS provides a logical public target;
the proxy never connects to that public address. Receiver Host/SNI, exact-body
HMAC and idempotency headers remain correct, with no Proxy-Authorization at the
receiver. Separate test CA pools validate proxy and receiver independently.
An untrusted receiver certificate is refused through both proxy types; an
untrusted HTTPS proxy is also refused. Test dialers reject non-fixture routes.

Negative cases reject private/mixed DNS before opening the proxy connection,
CONNECT 403 without direct/hostname fallback, redirects, excessive CONNECT
headers, cancelled handshake, unsupported proxy URL forms and unknown modes.
Constructor-only subprocesses verify default rejection and environment proxy/
NO_PROXY selection without DNS, dialing or sends. CLI preflight accepts explicit
pinned mode with an isolated fake key and creates no state or network request.
These subprocess settings are isolated test inputs, not changes to the running
session's proxy configuration.

The route uses public Go APIs compiled/tested against pinned Go 1.27.3:
http.ProxyFromEnvironment, Request.Write, ReadResponse, TLS HandshakeContext and
context.AfterFunc. CONNECT request target/Host use the same numeric authority;
the receiver hostname is used only inside verified target TLS/HTTP. Relevant
API references are https://pkg.go.dev/net/http and https://pkg.go.dev/crypto/tls;
those references are not evidence of any remote proxy's current capabilities.

Full local default tests: 251 top-level tests, six fuzz seed targets and four
intentional skips PASS. Affected alert/CLI race suites, formatting/vet/module
verification/tidy-diff, pinned module/source-test/binary vulnerability gates,
workflow/Bash and package/install checks PASS. Optional delivery unit syntax
passes with proxy mode defaulting to reject; installer starts/enables nothing.
Remote CI and actual target-host proxy/receiver/service operation are unverified.

The managed environment's real sidecar may deny numeric CONNECT or require a
hostname authority. Such denial is preserved, never worked around. No egress
rule, TCP grant, proxy selector, CA trust, secret or credential binding was changed.
The mode does not establish production routing or independent chain evidence.
Real proxy permission/receiver validation, retention rollover/archive chaining,
source-gap notifications, representative sizing and permanent-host validation
remain open. All existing UNVERIFIED/NOT_VERIFIED/UNKNOWN/UNDETERMINED assurance
and at-least-once delivery semantics remain.
