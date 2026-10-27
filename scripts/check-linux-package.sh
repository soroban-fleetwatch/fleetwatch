#!/usr/bin/env bash
# Build/install smoke and failure checks, entirely offline after Go deps exist.
set -euo pipefail
umask 077
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
scratch="$(mktemp -d /tmp/fleetwatch-package-check.XXXXXXXX)"
trap 'rm -rf -- "$scratch"' EXIT
./scripts/package-linux.sh "$scratch/artifacts"
(
  cd "$scratch/artifacts"
  sha256sum --check --strict ARCHIVE.sha256
  mkdir "$scratch/extracted"
  tar -xzf fleetwatch-linux-amd64.tar.gz --no-same-owner -C "$scratch/extracted"
)
bundle="$scratch/extracted/fleetwatch-linux-amd64"
baseline="$repo_root/testdata/reconstruction-baseline.json"
"$bundle/bin/install-user.sh" "$scratch/installed" "$baseline" https://rpc.example.invalid
export XDG_RUNTIME_DIR="$scratch/runtime"
mkdir -m 700 "$XDG_RUNTIME_DIR"
"$scratch/installed/bin/host-check.py" "$scratch/installed" > "$scratch/host.json"
systemd-analyze --user verify "$scratch/installed/systemd/fleetwatch-sync.service"
python3 - "$scratch" <<'PY'
import json,pathlib,sys
if not __debug__:raise SystemExit('package checks require Python assertions enabled')
root=pathlib.Path(sys.argv[1]);report=json.loads((root/'host.json').read_text())
assert report['offline'] and not report['production_validated']
assert report['result']=='WARN'
assert next(c for c in report['checks'] if c['check']=='configuration')['state']=='PASS'
assert next(c for c in report['checks'] if c['check']=='unit_syntax')['state']=='PASS'
assert not (root/'installed/var/lib/fleetwatch/follow.db').exists()
assert not list((root/'installed/var/lib/fleetwatch').iterdir())
PY
expect_failure() {
  if "$@" > "$scratch/expected-failure.log" 2>&1; then
    echo 'expected rejection but command succeeded' >&2
    exit 1
  fi
}
expect_failure "$bundle/bin/install-user.sh" "$scratch/installed" "$baseline" https://rpc.example.invalid
printf '{}\n' > "$scratch/invalid-baseline.json"
expect_failure "$bundle/bin/install-user.sh" "$scratch/invalid-install" "$scratch/invalid-baseline.json" https://rpc.example.invalid
test ! -e "$scratch/invalid-install"
expect_failure "$scratch/installed/bin/host-check.py" "$scratch/missing"
expect_failure "$scratch/installed/bin/host-check.py" "$scratch/installed" --min-free-bytes 9223372036854775807
# Mode failure must be diagnosed without opening/creating state.
chmod 0755 "$scratch/installed/var/lib/fleetwatch"
expect_failure "$scratch/installed/bin/host-check.py" "$scratch/installed"
chmod 0700 "$scratch/installed/var/lib/fleetwatch"
printf '\n# tampered\n' >> "$bundle/bin/host-check.py"
expect_failure "$bundle/bin/install-user.sh" "$scratch/tampered-install" "$baseline" https://rpc.example.invalid
test ! -e "$scratch/tampered-install"
test -z "$(ls -A "$scratch/installed/var/lib/fleetwatch")"
test -z "$(find "$scratch" -maxdepth 1 -name '.fleetwatch-install.*' -print -quit)"
echo 'Linux package checks PASS: archive, extracted install, offline preflight, no overwrite, invalid baseline, capacity/mode rejection and tamper rejection.'
