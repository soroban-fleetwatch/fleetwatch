#!/usr/bin/env bash
# Create the planned issues in docs/build/issues.json on GitHub.
# Default is a dry run that prints every label and issue and contacts nothing.
# --apply needs an authenticated `gh` CLI with issue write access. Run it only
# with the repository owner's approval: it creates public labels and issues.
set -euo pipefail

usage() {
  echo 'usage: create-issues.sh [--dry-run | --apply] [--repo OWNER/NAME] [--catalog FILE]' >&2
  exit 3
}

mode=dry-run
repo=soroban-fleetwatch/fleetwatch
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
catalog="$repo_root/docs/build/issues.json"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) mode=dry-run ;;
    --apply) mode=apply ;;
    --repo) [[ $# -ge 2 ]] || usage; repo="$2"; shift ;;
    --catalog) [[ $# -ge 2 ]] || usage; catalog="$2"; shift ;;
    *) usage ;;
  esac
  shift
done
[[ "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo 'invalid --repo' >&2; exit 3; }
[[ -f "$catalog" ]] || { echo "catalog not found: $catalog" >&2; exit 3; }

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT

# Render one body file per issue plus TSV indexes, validating the catalog.
python3 -I - "$catalog" "$work" <<'PY'
import json, pathlib, sys
catalog, out = json.load(open(sys.argv[1], encoding='utf-8')), pathlib.Path(sys.argv[2])
labels, issues = catalog['labels'], catalog['issues']
titles = set()
with open(out / 'labels.tsv', 'w', encoding='utf-8') as f:
    for name, color in sorted(labels.items()):
        assert len(color) == 6 and all(c in '0123456789abcdef' for c in color), name
        f.write(f'{name}\t{color}\n')
with open(out / 'issues.tsv', 'w', encoding='utf-8') as f:
    for n, issue in enumerate(issues, 1):
        title = issue['title']
        assert title not in titles and '\t' not in title and len(title) <= 120, title
        titles.add(title)
        unknown = set(issue['labels']) - set(labels)
        assert not unknown, (title, unknown)
        assert issue['summary'] and issue['acceptance'] and issue['stack'], title
        body = ['## Summary', '', issue['summary'], '', '## Acceptance criteria', '']
        body += [f'- [ ] {item}' for item in issue['acceptance']]
        body += ['', '## Tech stack', '', issue['stack'], '',
                 '_From the planned backlog in docs/build/issues.json. Read CONTRIBUTING.md before starting._', '']
        (out / f'{n}.md').write_text('\n'.join(body), encoding='utf-8')
        f.write(f"{n}\t{title}\t{','.join(issue['labels'])}\n")
PY

if [[ "$mode" == dry-run ]]; then
  echo "DRY RUN for $repo: nothing is sent."
  echo "Labels:"
  while IFS=$'\t' read -r name color; do echo "  $name (#$color)"; done < "$work/labels.tsv"
  while IFS=$'\t' read -r n title labels; do
    echo
    echo "=== Issue $n: $title"
    echo "Labels: $labels"
    cat "$work/$n.md"
  done < "$work/issues.tsv"
  echo
  echo "$(wc -l < "$work/issues.tsv") issues previewed. Re-run with --apply to create them."
  exit 0
fi

command -v gh >/dev/null || { echo 'gh CLI is required for --apply' >&2; exit 3; }
gh auth status >/dev/null 2>&1 || { echo 'gh is not authenticated' >&2; exit 3; }

while IFS=$'\t' read -r name color; do
  gh label create "$name" --repo "$repo" --color "$color" --force >/dev/null
  echo "label ok: $name"
done < "$work/labels.tsv"

# Exact-title duplicate check against every existing issue, open or closed.
gh issue list --repo "$repo" --state all --limit 1000 --json title --jq '.[].title' > "$work/existing.txt"
created=0
skipped=0
while IFS=$'\t' read -r n title labels; do
  if grep -Fxq -- "$title" "$work/existing.txt"; then
    echo "skip (exists): $title"
    skipped=$((skipped + 1))
    continue
  fi
  gh issue create --repo "$repo" --title "$title" --label "$labels" --body-file "$work/$n.md"
  created=$((created + 1))
done < "$work/issues.tsv"
echo "created=$created skipped=$skipped"
