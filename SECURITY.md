# Security policy

## Status

FleetWatch is an experimental prototype. It has had **no external security
audit**. Internal fuzzing, race tests and dependency scans are not an audit.
Do not rely on it as the only protection for contracts that hold real value.

## Supported versions

There is no tagged release yet. Only the latest commit on `main` receives fixes.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository:
**Security → Report a vulnerability** at
<https://github.com/soroban-fleetwatch/fleetwatch/security/advisories/new>.

That link works only after a repository administrator enables private
vulnerability reporting (Settings → Code security). Until it is enabled, do not
publish exploit details in a public issue. Open a public issue that says only
that you have a security report and need a private channel, with no technical
details.

Include, where you can:

- affected command, file or package, and commit SHA;
- steps or input to reproduce;
- impact: for example a missed change, a false "complete" result, a forged
  approval accepted, a crash on crafted input, or secret leakage.

## Scope

In scope:

- parsing of RPC JSON, XDR, snapshots, configuration and approval files;
- failure to report a reference change, repoint or observation gap;
- a partial observation reported as complete;
- unsigned policy matches treated as signed approvals; signature bypass;
- webhook delivery issues: SSRF, HMAC bypass, secret leakage in logs or output;
- corruption or silent loss of stored evidence, cursors or outbox records.

Out of scope:

- vulnerabilities in Stellar Core, Stellar RPC or contracts that FleetWatch observes;
- results that the documentation already labels `UNVERIFIED`, `UNKNOWN` or
  `UNDETERMINED` (for example, source authentication is a known open gap);
- denial of service that needs control of the configured RPC endpoint, unless
  it bypasses a documented bound.

## Response

There is no bug bounty and no guaranteed response time. Maintainers aim to
acknowledge reports within 7 days, as a goal and not a promise. Fixes are
released as normal commits with a changelog entry once a fix is available.
