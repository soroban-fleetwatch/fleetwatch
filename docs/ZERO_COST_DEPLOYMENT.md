# Zero-cost deployment decision

The user requires no deployment charge. The first candidate is an
**existing owner-controlled, already-running Linux machine with persistent local
disk and network access**, with owner-confirmed spare capacity and no additional
hosting, hardware, power or network charge for FleetWatch. It must still have
enough resources, uptime and an allowed receiver route. An ephemeral development
workspace is not a permanent production host merely because it costs nothing.

No such host is connected in this session. The current managed environment has
no configured cloud identity, provider capability, SSH host or authorized
external receiver. Its PID 1 is tail, with no working permanent user-systemd
deployment established. Do not treat it as the host, start an unattended service
there or claim reboot/persistence acceptance.

An always-free cloud VM could be an alternative **only after current account
eligibility, capacity and billing rules are confirmed in the provider account**.
Oracle OCI Always Free is a possible candidate, not a verified allocation or
current eligibility promise. Read-only fetches of the official Oracle free-tier
and Always Free documentation URLs returned HTTP 403 through this session's
normal proxy on 2026-10-27. No network control was bypassed. Free capacity may
depend on architecture/region/account conditions; do not substitute a paid
instance or expiring paid trial if free resources are unavailable. Current
provider plan names, quotas and cost safeguards have not been verified here.

The official pages to review in an authorized browser/account are:

- https://www.oracle.com/cloud/free/
- https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier.htm

If a VM is actually allocated, verify its architecture before selecting the
matching bundle. A Linux ARM64 allocation requires an ARM64 binary; an amd64
archive cannot be assumed to work. Use the candidate's BUILD.json and checksum,
and the current packaging guide. Do not guess an unavailable cloud shape or use
an API call copied from an unverified provider example.

## Information still needed

Provide one owner-controlled existing Linux host or an actually allocated,
confirmed-free cloud VM, together with its normal authorized access mechanism.
Put credentials/SSH private keys in the supported environment connection/secret
settings, not chat or the repository. A host address alone does not supply login
authority. Also identify an authorized HTTPS notification receiver and provision
its HMAC key through the host's supported secret mechanism. No public callback
port is needed for FleetWatch itself: its status listener stays loopback-only.

Before deployment, verify in the provider account that the chosen allocation and
disk/network usage will not incur charges, and that paid fallback/upgrade is
disabled. Budgets/alerts alone are not proof of a hard spending cap. No purchase,
new account, billing instrument, paid resource or provider permission is created
by any FleetWatch script in this package.

## Once a real zero-cost host is available

1. Confirm Linux architecture, persistent local storage, owner access and actual
   cost policy. Keep source and delivery DBs on that host, not an ephemeral or
   network-mounted filesystem. Review sizing and retention before unattended use.
2. Transfer the final clean candidate, archive checksum and original asserted
   baseline through the normal authorized connection. Verify hashes before
   extraction/install. Follow [OPERATIONS.md](OPERATIONS.md); installer refuses
   overwrite and starts/enables nothing.
3. Run installed host-check and [Phase 9 evidence collection](PHASE9_ACCEPTANCE.md).
   Review the default follower versus automatic-sync follower unit; choose one.
   Confirm actual unit hardening/cgroups, boot/login behavior and loopback health.
4. Configure the separate dispatcher only for the authorized receiver, with a
   private environment/secret setup. Review [DELIVERY_LOOP.md](DELIVERY_LOOP.md)
   and [WEBHOOK_PROXY.md](WEBHOOK_PROXY.md), preserving required proxy/CA policy.
   Local preflight does not prove the real route is permitted.
5. Exercise controlled finding → queue → verified receiver ACK → restart/resume,
   source outage/not-ready, backup/separate restore, logout and reboot. Record
   actual evidence in [PHASE9_ACCEPTANCE.md](PHASE9_ACCEPTANCE.md).

This is a concrete zero-cost handoff, not a completed deployment. Until host,
access, receiver and cost conditions are supplied/verified, the Dripwave Phase 9
production hosting gate remains blocked. No approval, uptime SLA, production
chain authentication or guaranteed free allocation is claimed.
