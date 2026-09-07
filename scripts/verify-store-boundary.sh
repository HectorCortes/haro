#!/usr/bin/env bash
set -euo pipefail

# verify-store-boundary.sh — F-01 store import containment gate
# Fails if database/sql or modernc.org/sqlite is imported outside internal/store.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "== store boundary check =="

# Use git grep to respect .gitignore and allow exclusion patterns
if ! command -v git >/dev/null 2>&1 || ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "not a git repo, using grep"
  GREP="grep -R -n"
  HITS=$($GREP -E "database/sql|modernc\.org/sqlite" --include="*.go" . 2>/dev/null | grep -v "internal/store" || true)
else
  HITS=$(git grep -n -E "database/sql|modernc\.org/sqlite" -- '*.go' ':!internal/store/**' 2>/dev/null || true)
fi

if [ -n "$HITS" ]; then
  echo "FAIL: store boundary violation — SQL imports outside internal/store:"
  echo "$HITS"
  exit 1
fi

echo "store boundary check passed — no SQL imports outside internal/store"
exit 0
