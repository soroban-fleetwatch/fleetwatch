#!/usr/bin/env bash
# Testnet-only fixture smoke. This is not the full history/alert acceptance gate.
set -euo pipefail
umask 077
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

case "${1:-check}" in
  check)
    [[ $# -le 1 ]] || { echo 'usage: demo-testnet.sh check' >&2; exit 3; }
    bash scripts/build-testnet-fixtures.sh
    exec go run ./scripts/testnet-preflight
    ;;
  run)
    [[ $# -eq 4 ]] || { echo 'usage: demo-testnet.sh run KEY_ALIAS CONFIG_DIR NEW_OUTPUT_DIR' >&2; exit 3; }
    ;;
  *) echo 'use check (read only) or run KEY_ALIAS CONFIG_DIR NEW_OUTPUT_DIR (Testnet writes)' >&2; exit 3 ;;
esac

# An alias is accepted, never a secret key or seed phrase in the command line.
alias_name="$2"
[[ "$alias_name" =~ ^[a-z][a-z0-9_-]{0,47}$ ]] || { echo 'invalid test-only key alias' >&2; exit 3; }
config_dir="$(realpath -e -- "$3")"
[[ -d "$config_dir" ]] || { echo 'config directory is required' >&2; exit 3; }
case "$config_dir/" in "$repo_root/"*) echo 'keep the Testnet signing config outside the repository' >&2; exit 3 ;; esac
[[ ! -e "$4" ]] || { echo 'output directory must not exist' >&2; exit 3; }
mkdir -m 700 -- "$4"
output_dir="$(realpath -e -- "$4")"
fixtures="$repo_root/contracts/testfixtures/target/fixtures"
for wasm in owner member-v1 member-v2; do
  [[ -s "$fixtures/$wasm.wasm" ]] || { echo 'build and test fixtures first' >&2; exit 3; }
done

go run ./scripts/testnet-preflight > "$output_dir/preflight.json"
go build -trimpath -o "$output_dir/fleetwatch" ./cmd/fleetwatch
rpc='https://soroban-testnet.stellar.org'
passphrase='Test SDF Network ; September 2015'
# The CLI's jsonrpsee client does not honor the managed HTTP proxy. A loopback
# adapter forwards only allowed RPC methods to the fixed HTTPS Testnet endpoint,
# preserving the managed outbound proxy and OS CA verification.
go build -trimpath -o "$output_dir/testnet-rpc-bridge" ./scripts/testnet-rpc-bridge
"$output_dir/testnet-rpc-bridge" > "$output_dir/bridge-url.txt" 2> "$output_dir/bridge.log" &
bridge_pid=$!
trap 'kill "$bridge_pid" 2>/dev/null || true; wait "$bridge_pid" 2>/dev/null || true' EXIT
for ((ready_attempt=0; ready_attempt<50; ready_attempt++)); do
  [[ -s "$output_dir/bridge-url.txt" ]] && break
  kill -0 "$bridge_pid" 2>/dev/null || { echo 'Testnet bridge failed to start' >&2; exit 4; }
  sleep 0.1
done
bridge_url="$(cat "$output_dir/bridge-url.txt")"
[[ "$bridge_url" =~ ^http://127\.0\.0\.1:[0-9]+$ ]] || { echo 'Testnet bridge did not become ready' >&2; exit 4; }
common=(--config-dir "$config_dir" --rpc-url "$bridge_url" --network-passphrase "$passphrase" --source-account "$alias_name" --inclusion-fee 100)
admin="$(stellar keys address "$alias_name" --config-dir "$config_dir")"
[[ "$admin" =~ ^G[A-Z2-7]{55}$ ]] || { echo 'invalid test-only account address' >&2; exit 3; }

# Keep public CLI transaction receipts, never request or print signing material.
stellar contract upload "${common[@]}" --optimize=false --wasm "$fixtures/member-v1.wasm" > "$output_dir/hash-a.txt" 2> "$output_dir/upload-a.log"
stellar contract upload "${common[@]}" --optimize=false --wasm "$fixtures/member-v2.wasm" > "$output_dir/hash-b.txt" 2> "$output_dir/upload-b.log"
hash_a="$(cat "$output_dir/hash-a.txt")"
hash_b="$(cat "$output_dir/hash-b.txt")"
[[ "$hash_a" =~ ^[0-9a-f]{64}$ && "$hash_b" =~ ^[0-9a-f]{64}$ && "$hash_a" != "$hash_b" ]] || { echo 'invalid uploaded hashes' >&2; exit 4; }
[[ "$hash_a" == "$(sha256sum "$fixtures/member-v1.wasm" | cut -d' ' -f1)" && "$hash_b" == "$(sha256sum "$fixtures/member-v2.wasm" | cut -d' ' -f1)" ]] || { echo 'uploaded hashes differ from tested Wasm' >&2; exit 4; }
stellar contract deploy "${common[@]}" --optimize=false --wasm "$fixtures/owner.wasm" -- --admin "$admin" > "$output_dir/owner.txt" 2> "$output_dir/deploy-owner.log"
owner="$(cat "$output_dir/owner.txt")"
[[ "$owner" =~ ^C[A-Z2-7]{55}$ ]] || { echo 'invalid owner contract' >&2; exit 4; }
invoke_owner() { stellar contract invoke "${common[@]}" --id "$owner" --send=yes -- "$@"; }
invoke_owner publish --tag fleetwatch-demo --wasm_hash "$hash_a" > "$output_dir/publish-a.json" 2> "$output_dir/publish-a.log"
for member in 1 2; do
  salt="$(python3 -c 'import sys; print((bytes([int(sys.argv[1])])*32).hex())' "$member")"
  invoke_owner deploy --tag fleetwatch-demo --salt "$salt" > "$output_dir/member-$member.json" 2> "$output_dir/member-$member.log"
done
python3 - "$output_dir" <<'PY'
import json,pathlib,re,sys
p=pathlib.Path(sys.argv[1])
ids=[json.loads((p/f'member-{i}.json').read_text()) for i in (1,2)]
assert all(isinstance(x,str) and re.fullmatch(r'C[A-Z2-7]{55}',x) for x in ids)
assert ids[0]!=ids[1]
(p/'contracts.txt').write_text('\n'.join(ids)+'\n')
PY
capture() {
  local result=0
  "$output_dir/fleetwatch" capture --rpc-url "$rpc" --network-passphrase "$passphrase" --contracts-file "$output_dir/contracts.txt" > "$output_dir/$1.json" || result=$?
  [[ "$result" == 0 || "$result" == 4 ]] || return "$result"
}
check_capture() {
python3 - "$output_dir" "$owner" "$1" "$2" <<'PY'
import json,pathlib,sys
p=pathlib.Path(sys.argv[1]);owner=sys.argv[2];tag=b'fleetwatch-demo'.hex()
ids=set((p/'contracts.txt').read_text().splitlines())
name,expected=sys.argv[3:5]
c=json.loads((p/f'{name}.json').read_text())
# Explicitly retain the one expected diagnostic. Any other gap stops the smoke.
assert c['errors']==['multi-call ledger consistency is not proven; capture is diagnostic'],c['errors']
s=c['snapshot'];assert {x['id'] for x in s['contracts']}==ids
assert all(x['executable']=={'kind':'external_ref','owner':owner,'tag_hex':tag} for x in s['contracts'])
assert s['references']==[{'owner':owner,'tag_hex':tag,'wasm_hash':expected}]
(p/f'{name}-snapshot.json').write_text(json.dumps(s,indent=2)+'\n')
PY
}
capture before
check_capture before "$hash_a"
invoke_owner publish --tag fleetwatch-demo --wasm_hash "$hash_b" > "$output_dir/publish-b.json" 2> "$output_dir/publish-b.log"
capture after
check_capture after "$hash_b"
result=0
"$output_dir/fleetwatch" compare --before "$output_dir/before-snapshot.json" --after "$output_dir/after-snapshot.json" --format json > "$output_dir/diagnostic-comparison.json" || result=$?
[[ "$result" == 2 ]] || { echo 'expected a diagnostic reference-change finding' >&2; exit 4; }
python3 - "$output_dir" "$owner" "$hash_a" "$hash_b" <<'PY'
import json,pathlib,sys
p=pathlib.Path(sys.argv[1]);r=json.loads((p/'diagnostic-comparison.json').read_text())
assert r['coverage']=='UNKNOWN' and r['evaluation_status']=='UNVERIFIED_SNAPSHOTS'
assert len(r['findings'])==1
f=r['findings'][0]
assert f['code']=='UNAPPROVED_REFERENCE_CHANGE' and f['authorization']=='UNVERIFIED'
assert f['owner']==sys.argv[2] and f['tag_hex']==b'fleetwatch-demo'.hex()
assert f['old_hash']==sys.argv[3] and f['new_hash']==sys.argv[4]
assert set(f['affected_contracts'])==set((p/'contracts.txt').read_text().splitlines())
PY
echo "Testnet snapshot smoke recorded in $output_dir; captures remain PARTIAL and authorization UNVERIFIED."
echo 'History reconstruction, restart, alert delivery and independent transaction verification still require validation.'
