#!/usr/bin/env bash
# Local deployment candidate only: no release, tag, image push or remote deploy.
set -euo pipefail
umask 077
if [[ $# != 1 && $# != 2 ]]; then
  echo 'usage: package-linux.sh NEW_OUTPUT_DIR [amd64|arm64]' >&2
  exit 3
fi
target_arch="${2:-amd64}"
[[ "$target_arch" == amd64 || "$target_arch" == arm64 ]] || { echo 'target architecture must be amd64 or arm64' >&2; exit 3; }
[[ ! -e "$1" && ! -L "$1" ]] || { echo 'output directory must not exist' >&2; exit 3; }
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -m 700 -- "$1"
output="$(realpath -e -- "$1")"
cd "$repo_root"
[[ "$(go env GOVERSION)" == "go$(cat .go-version)" ]] || { echo 'activate the pinned Go compiler' >&2; exit 3; }
bundle_name="fleetwatch-linux-$target_arch"
bundle="$output/$bundle_name"
mkdir -p "$bundle/bin" "$bundle/docs" "$bundle/systemd" "$bundle/schema"
mkdir -p "$bundle/docs/research"
CGO_ENABLED=0 GOOS=linux GOARCH="$target_arch" GOFLAGS=-mod=readonly go build -trimpath -o "$bundle/bin/fleetwatch" ./cmd/fleetwatch
install -m 0755 deploy/install-user.sh "$bundle/bin/install-user.sh"
install -m 0755 deploy/host-check.py "$bundle/bin/host-check.py"
install -m 0755 scripts/phase9-preflight.sh "$bundle/bin/phase9-preflight.sh"
install -m 0644 deploy/systemd/fleetwatch.service.in deploy/systemd/fleetwatch-sync.service.in deploy/systemd/fleetwatch-delivery.service.in "$bundle/systemd/"
install -m 0644 docs/FOLLOW.md docs/OPERATIONS.md docs/ARCHIVES.md docs/COMPACTION.md docs/FOLLOW_DELIVERY.md docs/DELIVERY_LOOP.md docs/WEBHOOKS.md docs/WEBHOOK_PROXY.md docs/ALERTS.md docs/ALERT_RECOVERY.md docs/BACKUP.md "$bundle/docs/"
install -m 0644 schema/follow-alert-v3.schema.json schema/delivery-loop-status-v1.schema.json "$bundle/schema/"
install -m 0644 LICENSE "$bundle/LICENSE"
install -m 0644 docs/PHASE9_ACCEPTANCE.md docs/ZERO_COST_DEPLOYMENT.md "$bundle/docs/"
install -m 0644 docs/research/2026-10-27-phase9-sizing.json "$bundle/docs/research/"
python3 - "$bundle" "$target_arch" <<'PY'
import hashlib,json,os,pathlib,shutil,subprocess,sys
root=pathlib.Path(sys.argv[1]);arch=sys.argv[2]
meta={'kind':'LOCAL_EXPERIMENTAL_DEPLOYMENT_CANDIDATE','os':'linux','arch':arch,
 'go':subprocess.check_output(['go','env','GOVERSION'],text=True).strip(),
 'source_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
 'source_dirty':bool(subprocess.check_output(['git','status','--porcelain'],text=True).strip()),
 'public_release':False,'requires_os_ca_certificates':True}
(root/'BUILD.json').write_text(json.dumps(meta,indent=2)+'\n')
licenses=root/'licenses';licenses.mkdir()
goroot=pathlib.Path(subprocess.check_output(['go','env','GOROOT'],text=True).strip())
(licenses/'go').mkdir();shutil.copyfile(goroot/'LICENSE',licenses/'go/LICENSE')
raw=subprocess.check_output(['go','list','-deps','-json','./cmd/fleetwatch'],text=True,
 env=dict(os.environ,CGO_ENABLED='0',GOOS='linux',GOARCH=arch,GOFLAGS='-mod=readonly'))
decoder=json.JSONDecoder();modules={}
while raw.strip():
 raw=raw.lstrip();package,end=decoder.raw_decode(raw);raw=raw[end:]
 module=package.get('Module')
 if module and not module.get('Main'):modules[module['Path']]=module.get('Replace',module)
inventory=[]
for name,module in sorted(modules.items()):
 source=pathlib.Path(module['Dir']);destination=licenses/(name.replace('/','_')+'@'+module.get('Version','local'))
 destination.mkdir();copied=[]
 for filename in ['LICENSE','LICENSE.txt','LICENSE.md','LICENCE','COPYING','COPYRIGHT','NOTICE','NOTICE.txt','NOTICE.md']:
  if (source/filename).is_file():shutil.copyfile(source/filename,destination/filename);copied.append(filename)
 if not any(name.startswith(('LICENSE','LICENCE','COPYING')) for name in copied):
  raise SystemExit(f'runtime module license missing: {name}; inspect before distributing')
 inventory.append({'module':name,'version':module.get('Version'),'files':copied})
(licenses/'INDEX.json').write_text(json.dumps(inventory,indent=2)+'\n')
lines=[]
for path in sorted(root.rglob('*')):
 if path.is_file():lines.append(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.relative_to(root)}')
(root/'SHA256SUMS').write_text('\n'.join(lines)+'\n')
PY
epoch="$(git show -s --format=%ct HEAD)"
tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -czf "$output/$bundle_name.tar.gz" -C "$output" "$bundle_name"
(
  cd "$output"
  sha256sum "$bundle_name.tar.gz" > ARCHIVE.sha256
)
echo "Prepared $output/$bundle_name.tar.gz (local deployment candidate)."
