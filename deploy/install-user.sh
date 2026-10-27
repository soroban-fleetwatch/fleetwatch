#!/usr/bin/env bash
# Install into a NEW user-owned directory; never start or enable a service.
set -euo pipefail
umask 077
if [[ $# != 3 ]]; then
  echo 'usage: install-user.sh NEW_ABSOLUTE_INSTALL_ROOT ASSERTED_BASELINE_JSON HTTPS_RPC_URL' >&2
  exit 3
fi
[[ "$1" =~ ^/[a-zA-Z0-9_./-]+$ ]] || { echo 'install root must be absolute without whitespace or systemd metacharacters' >&2; exit 3; }
install_root="$(realpath -m -- "$1")"
[[ "$install_root" =~ ^/[a-zA-Z0-9_./-]+$ ]] || { echo 'resolved install root contains unsupported metacharacters' >&2; exit 3; }
[[ ! -e "$install_root" ]] || { echo 'install root must not exist' >&2; exit 3; }
baseline="$(realpath -e -- "$2")"
[[ -f "$baseline" ]] || { echo 'asserted baseline file required' >&2; exit 3; }
package_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$package_root"
sha256sum --check --strict SHA256SUMS >/dev/null
[[ -d "$(dirname -- "$install_root")" ]] || { echo 'install parent directory must exist' >&2; exit 3; }
staging="$(mktemp -d "$(dirname -- "$install_root")/.fleetwatch-install.XXXXXXXX")"
trap 'if [[ -d "$staging" ]]; then rm -rf -- "$staging"; fi' EXIT
mkdir -p "$staging/bin" "$staging/etc/fleetwatch" "$staging/var/lib/fleetwatch" "$staging/systemd" "$staging/docs" "$staging/schema"
install -m 0755 bin/fleetwatch "$staging/bin/fleetwatch"
install -m 0755 bin/host-check.py "$staging/bin/host-check.py"
install -m 0600 "$baseline" "$staging/etc/fleetwatch/baseline.json"
install -m 0644 LICENSE "$staging/LICENSE"
cp -R licenses "$staging/"
install -m 0644 docs/FOLLOW.md docs/OPERATIONS.md docs/ARCHIVES.md docs/COMPACTION.md docs/FOLLOW_DELIVERY.md docs/DELIVERY_LOOP.md docs/WEBHOOKS.md docs/WEBHOOK_PROXY.md docs/ALERTS.md docs/ALERT_RECOVERY.md docs/BACKUP.md "$staging/docs/"
install -m 0644 schema/follow-alert-v3.schema.json schema/delivery-loop-status-v1.schema.json "$staging/schema/"
python3 - "$staging" "$install_root" "$3" <<'PY'
import json,pathlib,sys
stage=pathlib.Path(sys.argv[1]);root=sys.argv[2]
config={'schema_version':1,'rpc_url':sys.argv[3],'baseline':'baseline.json',
 'state':'../../var/lib/fleetwatch/follow.db','listen':'127.0.0.1:9789',
 'poll_interval_seconds':5,'batch_size':8,'head_delay_ledgers':1,'max_endpoint_lag':4,
 'max_retained_ledgers':100000,'max_retained_payload_bytes':67108864}
(stage/'etc/fleetwatch/follow.json').write_text(json.dumps(config,indent=2)+'\n')
template=pathlib.Path('systemd/fleetwatch.service.in').read_text()
(stage/'systemd/fleetwatch.service').write_text(template.replace('@ROOT@',root))
template=pathlib.Path('systemd/fleetwatch-sync.service.in').read_text()
(stage/'systemd/fleetwatch-sync.service').write_text(template.replace('@ROOT@',root))
template=pathlib.Path('systemd/fleetwatch-delivery.service.in').read_text()
(stage/'systemd/fleetwatch-delivery.service').write_text(template.replace('@ROOT@',root))
PY
"$staging/bin/fleetwatch" follow --check --config "$staging/etc/fleetwatch/follow.json"
mv -T --no-clobber -- "$staging" "$install_root"
[[ ! -d "$staging" ]] || { echo 'install root appeared during installation; existing directory preserved' >&2; exit 5; }
echo "Installed into $install_root. No service was started or enabled."
echo "Review $install_root/systemd/fleetwatch.service and the operations guide before linking it to systemd."
echo 'The optional fleetwatch-sync.service enables local outbox sync only. Choose one follower unit; neither was started.'
echo 'The separate fleetwatch-delivery.service requires explicit receiver/secret configuration. It was not configured or started.'
