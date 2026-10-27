# Opt-in pinned CONNECT proxy routing

Webhook delivery defaults to `--proxy-mode reject`: if Go's environment proxy
selection chooses a proxy for the receiver, construction fails before delivery.
An explicit `--proxy-mode pinned-connect` enables an experimental HTTP/1.1
CONNECT route through the selected HTTP or HTTPS proxy.

```sh
# Starting this loop sends queued notifications to your configured receiver.
fleetwatch deliver-loop --state delivery.db \
  --network 'Test SDF Network ; September 2015' --stream pilot \
  --webhook-url https://hooks.example.invalid/fleetwatch \
  --allow-host hooks.example.invalid --proxy-mode pinned-connect
```

The example hostname is a placeholder. Choose an authorized real receiver and
provision its HMAC key through the supported host mechanism before sending.
The one-batch `deliver` command accepts the same proxy-mode option. The loop's
`--check` remains local only: it validates options/key/proxy URL selection, not
DNS, CONNECT permission, receiver availability or actual delivery.

## Destination and TLS checks

The route uses Go's normal `http.ProxyFromEnvironment`, including existing
HTTPS_PROXY/HTTP_PROXY/NO_PROXY selection. It does not change those variables,
discover credentials from files or silently bypass a selected proxy. If the
environment selects no proxy, the normal direct, DNS/IP-pinned route is used.

For a selected proxy, FleetWatch locally resolves the allowed receiver hostname
and rejects empty, excessive, private or mixed public/private answers exactly as
the direct sender does. It chooses a checked numeric public IP and sends both
CONNECT authority and Host as that **numeric IP:443**. It never sends the receiver
hostname for a second proxy-side resolution. The proxy TCP connection goes only
to the operator-selected proxy address. There is no fallback to a direct target
connection or a hostname CONNECT if the proxy denies the pinned route.

Inside the tunnel, the receiver HTTP Host and TLS SNI/certificate verification
still use the original allowed hostname. HTTPS proxies have a separate verified
TLS connection to the proxy hostname, with a minimum of TLS 1.2 for both roles.
Both use OS CA trust; no CLI option disables verification. HTTP proxy transport
is supported as selected by the environment; its trust and network path must be
appropriate for the operator's proxy setup.

Selected proxy URL userinfo, if provided by the supported environment route,
is sent as Basic Proxy-Authorization on CONNECT only. It is never copied to
receiver request headers or logs. HMAC uses the original receiver envelope and
delivery ID, unchanged. SOCKS, proxy URL paths/query/fragment and interactive
authentication schemes are unsupported. URLs and credentials are not emitted
in status or transport error records.

## Policy compatibility and bounds

The proxy must honor numeric CONNECT authorities and its destination policy
must permit the checked IP on port 443. A proxy that only permits named CONNECT
hosts may reject this mode. A numeric policy rejection remains a rejection:
do not change Host/authority to disguise the destination, unset a required proxy,
disable TLS or use an alternate route to evade policy. Use the supported network
configuration workflow if the destination genuinely needs permission.

This cloud session's real sidecar compatibility has **not** been established.
No authorized external receiver has been supplied or contacted. Passing local
proxy tests or --check does not establish remote permission or hosting readiness.
The mode adds a tested protocol path, not a claim that every proxy supports it.

CONNECT headers are capped at 16 KiB and handshake IO at ten seconds (or the
earlier context deadline). Cancellation closes the in-progress proxy socket.
Buffered bytes after the CONNECT header remain available for target TLS. Only
HTTP 200 opens the tunnel; redirects/auth/policy errors are permanent, while
408/429/5xx are retryable under the existing delivery policy. There is no
unbounded header/body drain. Existing outer 15-second send timeout, response
bounds, retry limits, sink binding and at-least-once delivery semantics remain.
Receiver deduplication is still required after ambiguous acknowledgement.

## Optional user service

The prepared delivery unit defaults FLEETWATCH_DELIVERY_PROXY_MODE to reject.
To select the new mode on a reviewed host, set that variable to pinned-connect
in the private delivery environment configuration, keeping the required proxy
and CA selectors supplied through the host's supported mechanism. The service
manager may not inherit interactive shell variables. No environment file,
credential, recipient, network grant or service start is created by installation.

Local fixtures verify HTTP and HTTPS proxy tunnels, numeric authority, receiver
hostname TLS and HMAC, proxy credential separation, denied/malformed/oversized
responses, cancellation, private/mixed DNS and untrusted certificates at both
TLS roles. These are loopback-only fixtures with explicit test CA trust and
fixed test routing. They are not production proxy or chain evidence validation.
Source/baseline UNVERIFIED, commitments NOT_VERIFIED, coverage UNKNOWN and final
UNDETERMINED remain unchanged.
