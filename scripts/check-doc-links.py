#!/usr/bin/env python3
"""Fail when a Markdown file links to a repository path that does not exist.

Covers every tracked *.md outside generated/published output. External URLs and
pure in-page anchors are skipped; mkdocs build --strict checks site anchors.
"""
import pathlib
import re
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parent.parent
tracked = subprocess.run(['git', 'ls-files', '*.md'], cwd=root, check=True,
                         capture_output=True, text=True).stdout.split()
link = re.compile(r'\]\(([^)\s]+)\)')
broken = []
for name in tracked:
    if name.startswith('web/'):
        continue
    path = root / name
    for target in link.findall(path.read_text(encoding='utf-8')):
        if target.startswith(('http://', 'https://', 'mailto:', '#')):
            continue
        relative = target.split('#', 1)[0]
        if relative and not (path.parent / relative).exists():
            broken.append(f'{name}: {target}')
for item in broken:
    print('broken link:', item)
print(f'checked {len(tracked)} Markdown files; {len(broken)} broken links')
sys.exit(1 if broken else 0)
