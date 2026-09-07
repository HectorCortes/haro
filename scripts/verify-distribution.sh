#!/usr/bin/env bash
set -euo pipefail

# verify-distribution.sh — D08 F-02 hook-free distribution gate
# Fails if an npm package manifest/shrinkwrap or an install lifecycle script
# (preinstall.sh, install.sh, postinstall.sh) exists in the tree. Exact-name
# classification only: documentation-like files such as requirements.txt,
# CMakeLists.txt, executable *.md/*.mdx and README.sh are allowed.

# Optional root argument defaults to the repository root (parent of scripts/).
ROOT="${1:-}"
if [ -z "$ROOT" ]; then
  ROOT="$(cd "$(dirname "$0")/.." && pwd)"
fi

echo "== distribution check (root: $ROOT) =="

FORBIDDEN='package\.json|npm-shrinkwrap\.json|preinstall\.sh|install\.sh|postinstall\.sh'

# Use git ls-files to inspect tracked files; fall back to find when the root
# is not a git work tree (e.g. fixture roots in tests).
if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  HITS=$(git -C "$ROOT" ls-files | grep -E "(^|/)(${FORBIDDEN})$" || true)
else
  echo "not a git repo, using find"
  HITS=$(cd "$ROOT" && find . -type f \( \
      -name 'package.json' -o \
      -name 'npm-shrinkwrap.json' -o \
      -name 'preinstall.sh' -o \
      -name 'install.sh' -o \
      -name 'postinstall.sh' \) | sed 's|^\./||' || true)
fi

if [ -n "$HITS" ]; then
  echo "FAIL: distribution violation — npm package artifacts or executable install hooks present:"
  echo "$HITS"
  exit 1
fi

echo "distribution check passed — no npm manifests or install hooks"
exit 0
