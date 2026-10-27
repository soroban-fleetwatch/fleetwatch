#!/usr/bin/env bash
set -euo pipefail
umask 077
mkdir -p /workspace/.tools /workspace/.cache/go-build /workspace/.cache/go-mod /workspace/.cache/go-path
python3 - /workspace/.tools <<'PY'
import fcntl
import hashlib
import os
import pathlib
import platform
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

base = pathlib.Path(sys.argv[1])
base.mkdir(parents=True, exist_ok=True)
expected = 'go version go1.27.3 linux/amd64'
digest = '65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40'
if sys.platform != 'linux' or platform.machine().lower() not in ('x86_64', 'amd64'):
    raise SystemExit('This cloud setup is pinned to Linux amd64; do not install an incompatible compiler')
go = base / 'go'
env = dict(os.environ, GOTOOLCHAIN='local')

def version(binary):
    try:
        return subprocess.check_output([str(binary), 'version'], text=True, env=env).strip()
    except (OSError, subprocess.CalledProcessError):
        return ''

with (base / '.fleetwatch-go-install.lock').open('a') as lock:
    fcntl.flock(lock, fcntl.LOCK_EX)
    if go.is_symlink():
        raise SystemExit('Refusing to replace a symlinked Go installation')
    if go.exists() and not go.is_dir():
        raise SystemExit('Refusing to replace a non-directory Go installation')
    if version(go / 'bin/go') != expected:
        with tempfile.TemporaryDirectory(prefix='.fleetwatch-go-stage-', dir=base) as work:
            stage = pathlib.Path(work)
            archive = stage / 'go.tar.gz'
            with urllib.request.urlopen('https://go.dev/dl/go1.27.3.linux-amd64.tar.gz', timeout=120) as response, archive.open('wb') as output:
                while chunk := response.read(1024 * 1024):
                    output.write(chunk)
            with archive.open('rb') as input_file:
                actual = hashlib.file_digest(input_file, 'sha256').hexdigest()
            if actual != digest:
                raise SystemExit('Go archive checksum mismatch; existing installation preserved')
            with tarfile.open(archive) as package:
                for member in package.getmembers():
                    parts = pathlib.PurePosixPath(member.name).parts
                    if not parts or parts[0] != 'go' or '..' in parts:
                        raise SystemExit('Go archive has an unexpected path')
                package.extractall(stage, filter='data')
            staged = stage / 'go'
            if version(staged / 'bin/go') != expected:
                raise SystemExit('Staged compiler mismatch; existing installation preserved')
            previous = stage / 'previous-go'
            had_previous = go.exists()
            try:
                if had_previous:
                    go.rename(previous)
                staged.rename(go)
            except BaseException:
                if previous.exists() and not go.exists():
                    previous.rename(go)
                raise
    if version(go / 'bin/go') != expected:
        raise SystemExit('Selected compiler does not match the pinned version')
print(expected)
PY
export PATH=/workspace/.tools/go/bin:$PATH
export GOCACHE=/workspace/.cache/go-build GOMODCACHE=/workspace/.cache/go-mod GOPATH=/workspace/.cache/go-path
export GOFLAGS=-mod=readonly GOTOOLCHAIN=local GOTELEMETRY=off
cd /workspace/fleetwatch
test "$(git rev-parse --show-toplevel)" = /workspace/fleetwatch
test "$(go env GOMOD)" = /workspace/fleetwatch/go.mod
test "$(go env GOVERSION)" = "go$(cat .go-version)"
go mod download || GOPROXY=direct go mod download
go mod verify
go mod tidy -diff
go test -timeout=60s -count=1 ./...
go build -trimpath -o /workspace/.tools/bin/fleetwatch ./cmd/fleetwatch
/workspace/.tools/bin/fleetwatch version
/workspace/.tools/bin/fleetwatch validate --snapshot testdata/before.json
