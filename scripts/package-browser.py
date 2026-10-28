#!/usr/bin/env python3
"""Build a self-contained browser app. No network or deployment credentials."""
import base64
import gzip
import hashlib
import html
import json
import os
from pathlib import Path
import subprocess
import sys


def run(*args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()


def main():
    root = Path(__file__).resolve().parent.parent
    if len(sys.argv) != 2:
        raise SystemExit('usage: scripts/package-browser.py NEW_OUTPUT_DIRECTORY')
    out = Path(sys.argv[1]).resolve()
    out.mkdir(mode=0o700, parents=True, exist_ok=False)
    expected = 'go' + (root / '.go-version').read_text().strip()
    if run('go', 'env', 'GOVERSION') != expected:
        raise SystemExit('select the compiler pinned in .go-version')
    compiler_root = Path(run('go', 'env', 'GOROOT'))
    commit = run('git', '-C', str(root), 'rev-parse', 'HEAD')
    dirty = bool(run('git', '-C', str(root), 'status', '--porcelain', '--untracked-files=normal'))
    env = dict(os.environ, GOOS='js', GOARCH='wasm', CGO_ENABLED='0')
    wasm = out / 'fleetwatch.wasm'
    subprocess.run(['go', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(wasm), './cmd/fleetwatch-browser'], cwd=root, env=env, check=True)
    runtime = (compiler_root / 'lib/wasm/wasm_exec.js').read_text()
    # Do not permit an accidental closing HTML script tag in the embedded runtime.
    if '</script' in runtime.lower():
        raise SystemExit('runtime contains a closing script tag')
    app = (root / 'web/index.template.html').read_text()
    app = app.replace('@@BUILD@@', commit + (' (source modified)' if dirty else ''))
    app = app.replace('@@GO_RUNTIME@@', runtime)
    app = app.replace('@@WASM_GZIP_BASE64@@', base64.b64encode(gzip.compress(wasm.read_bytes(), mtime=0)).decode('ascii'))
    wasm.unlink()
    (out / 'GO-LICENSE.txt').write_text((compiler_root / 'LICENSE').read_text())
    # Same linked dependency/license inventory used by the portable CLI package.
    inventory = run('go', 'list', '-deps', '-f', '{{if .Module}}{{.Module.Path}} {{.Module.Version}} {{.Module.Dir}}{{end}}', './cmd/fleetwatch-browser', cwd=root, env=env)
    modules = {}
    license_dir = out / 'licenses'
    license_dir.mkdir()
    for line in inventory.splitlines():
        parts = line.split()
        if len(parts) != 3 or parts[0] in modules:
            continue
        name, version, location = parts
        modules[name] = version
        source = Path(location)
        for candidate in ('LICENSE', 'LICENSE.txt', 'LICENSE.md', 'COPYING'):
            if (source / candidate).is_file():
                (license_dir / (name.replace('/', '_') + '.txt')).write_bytes((source / candidate).read_bytes())
                break
        else:
            raise SystemExit('missing license for linked module ' + name)
    (out / 'LICENSE').write_bytes((root / 'LICENSE').read_bytes())
    notices = []
    for path in [out/'LICENSE', out/'GO-LICENSE.txt', *sorted(license_dir.iterdir())]:
        notices.append('<h3>' + html.escape(path.name) + '</h3><pre>' + html.escape(path.read_text()) + '</pre>')
    app = app.replace('</main>', '<details><summary>Open-source licenses</summary>' + ''.join(notices) + '</details></main>')
    (out / 'FleetWatch.html').write_text(app)
    metadata = dict(schema_version=1, source_commit=commit, source_dirty=dirty, compiler=expected, target='js/wasm', runtime='foreground_browser', history_completeness='UNKNOWN', source_authentication='UNVERIFIED', chain_verdict='UNDETERMINED', production_daemon=False, linked_modules=modules)
    (out / 'BUILD.json').write_text(json.dumps(metadata, indent=2) + '\n')
    lines = []
    for path in sorted(out.rglob('*')):
        if path.is_file():
            lines.append(hashlib.sha256(path.read_bytes()).hexdigest() + '  ' + str(path.relative_to(out)))
    (out / 'SHA256SUMS').write_text('\n'.join(lines) + '\n')
    print(json.dumps(dict(output=str(out), html_bytes=(out/'FleetWatch.html').stat().st_size, **metadata)))


if __name__ == '__main__':
    main()
