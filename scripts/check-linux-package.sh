#!/usr/bin/env bash
# Build/install smoke and failure checks, entirely offline after Go deps exist.
set -euo pipefail
umask 077
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
scratch="$(mktemp -d /tmp/fleetwatch-package-check.XXXXXXXX)"
trap 'rm -rf -- "$scratch"' EXIT
[[ "$(uname -s)" == Linux ]] || { echo 'package checks require Linux' >&2; exit 3; }
case "$(uname -m)" in
  x86_64|amd64) native_arch=amd64; cross_arch=arm64 ;;
  aarch64|arm64) native_arch=arm64; cross_arch=amd64 ;;
  *) echo 'package checks require Linux amd64 or arm64' >&2; exit 3 ;;
esac
if [[ "$native_arch" == amd64 ]]; then
  ./scripts/package-linux.sh "$scratch/artifacts"
else
  ./scripts/package-linux.sh "$scratch/artifacts" "$native_arch"
fi
(
  cd "$scratch/artifacts"
  sha256sum --check --strict ARCHIVE.sha256
  mkdir "$scratch/extracted"
  tar -xzf "fleetwatch-linux-$native_arch.tar.gz" --no-same-owner -C "$scratch/extracted"
)
bundle="$scratch/extracted/fleetwatch-linux-$native_arch"
baseline="$repo_root/testdata/reconstruction-baseline.json"
"$bundle/bin/install-user.sh" "$scratch/installed" "$baseline" https://rpc.example.invalid
export XDG_RUNTIME_DIR="$scratch/runtime"
mkdir -m 700 "$XDG_RUNTIME_DIR"
"$scratch/installed/bin/host-check.py" "$scratch/installed" > "$scratch/host.json"
systemd-analyze --user verify "$scratch/installed/systemd/fleetwatch-sync.service"
systemd-analyze --user verify "$scratch/installed/systemd/fleetwatch-delivery.service"
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
# Inspect the other target architecture without running its binary on this host.
./scripts/package-linux.sh "$scratch/cross-artifacts" "$cross_arch"
(
  cd "$scratch/cross-artifacts"
  sha256sum --check --strict ARCHIVE.sha256
  mkdir "$scratch/cross-extracted"
  tar -xzf "fleetwatch-linux-$cross_arch.tar.gz" --no-same-owner -C "$scratch/cross-extracted"
)
cross_bundle="$scratch/cross-extracted/fleetwatch-linux-$cross_arch"
(
  cd "$cross_bundle"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
python3 - "$bundle" "$native_arch" "$cross_bundle" "$cross_arch" <<'PY'
import json,pathlib,struct,subprocess,sys
if not __debug__:raise SystemExit('package checks require Python assertions enabled')
for root_arg,arch in [(sys.argv[1],sys.argv[2]),(sys.argv[3],sys.argv[4])]:
 root=pathlib.Path(root_arg);meta=json.loads((root/'BUILD.json').read_text())
 assert meta['os']=='linux' and meta['arch']==arch and meta['public_release'] is False
 binary=root/'bin/fleetwatch'
 with binary.open('rb') as stream:
  header=stream.read(64)
  assert header[:4]==b'\x7fELF' and header[4:6]==b'\x02\x01'
  assert struct.unpack_from('<H',header,18)[0]=={'amd64':62,'arm64':183}[arch]
  offset=struct.unpack_from('<Q',header,32)[0]
  entry_size,entries=struct.unpack_from('<HH',header,54)
  assert entry_size>=4 and entry_size*entries<=1<<20
  stream.seek(offset);programs=stream.read(entry_size*entries)
  assert len(programs)==entry_size*entries
  assert all(struct.unpack_from('<I',programs,i*entry_size)[0]!=3 for i in range(entries))
 # go version -m reads build metadata and does not execute the target binary.
 info=subprocess.check_output(['go','version','-m',str(binary)],text=True)
 assert 'GOOS=linux' in info and 'GOARCH='+arch in info and 'CGO_ENABLED=0' in info
print('Both Linux ELF architectures, static linkage and target build metadata verified without executing the foreign binary.')
PY
expect_failure "$cross_bundle/bin/install-user.sh" "$scratch/wrong-arch-install" "$baseline" https://rpc.example.invalid
python3 - "$scratch/expected-failure.log" <<'PY'
import pathlib,sys
if 'package architecture does not match host' not in pathlib.Path(sys.argv[1]).read_text():
 raise SystemExit('foreign package was not rejected by the installer architecture guard')
PY
test ! -e "$scratch/wrong-arch-install"
echo 'Linux package checks PASS: native install/preflight/failures, both archives/ELF targets, and foreign-architecture install rejection. Foreign runtime remains unverified.'
