#!/usr/bin/env bash
set -euo pipefail

# verify-traceability.sh — v2-flujo-sdd/U-01 criterion→test traceability gate.
#
# Parses deltas-acceptance.md, enumerates all 92 criteria, strictly requires
# D09's (v2-flujo-sdd) four named Go tests in internal/cmd/flow_test.go, and
# classifies the remaining 88 informationally:
#   - 59 allowlisted debt IDs (v2-broker/v2-ipc = deferred:cli-direct;
#     v2-no-regresion/v2-adapter/v2-path-claims = archived-pending),
#   - 29 completed-spec IDs backed by archived spec evidence (archived).
# Unknown debt fails. Archived spec evidence accepts BOTH layouts:
# <archive>/<dir>/spec.md (flat) and <archive>/<dir>/specs/<name>/spec.md,
# with verify-report.md required at the change-dir level.
#
# Usage: verify-traceability.sh [--check-only] [root]
#   --check-only  skip the D09 flow test execution phase
#   root          optional repository/fixture root (default: repo root
#                 derived from this script's location)
# Exit codes: 0 pass, 1 fail-closed (parse, evidence, mapping, or test error).

CHECK_ONLY=0
ROOT=""
for arg in "$@"; do
  case "$arg" in
    --check-only) CHECK_ONLY=1 ;;
    -h|--help)
      sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      if [ -n "$ROOT" ]; then
        echo "FAIL: unexpected extra argument: $arg" >&2
        exit 1
      fi
      ROOT="$arg"
      ;;
  esac
done

fail() { echo "FAIL: $*" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
if [ -z "$ROOT" ]; then
  ROOT="$DEFAULT_ROOT"
fi
[ -d "$ROOT" ] || fail "root does not exist: $ROOT"
ROOT="$(cd "$ROOT" && pwd)"

DELTAS="$ROOT/deltas-acceptance.md"
FLOW_TEST="$ROOT/internal/cmd/flow_test.go"
ARCHIVE="$ROOT/openspec/changes/archive"

[ -f "$DELTAS" ] || fail "deltas-acceptance.md not found under root: $ROOT"
[ -f "$FLOW_TEST" ] || fail "internal/cmd/flow_test.go not found under root: $ROOT"

echo "== traceability check (root: $ROOT) =="

# --- Discovery ---------------------------------------------------------------
# In a git work tree the contract and the flow test file must be tracked;
# otherwise (fixture roots) fall back to find.
if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  for tracked in "$DELTAS" "$FLOW_TEST"; do
    rel="${tracked#"$ROOT"/}"
    git -C "$ROOT" ls-files --error-unmatch "$rel" >/dev/null 2>&1 ||
      fail "required file is not tracked in the git work tree: $rel"
  done
else
  echo "not a git repo, using find"
  found="$(cd "$ROOT" && find . -maxdepth 3 \( -name 'deltas-acceptance.md' -o -name 'flow_test.go' \) 2>/dev/null | wc -l)"
  [ "$found" -ge 2 ] || fail "contract or flow test file not discoverable via find under: $ROOT"
fi

# --- Parse the contract ------------------------------------------------------
# Emits "ID <spec>/<local>" in document order, "SPEC <id>" per spec, and
# "ERR <message>" for malformed headings or unattached criteria.
parse_stream() {
  awk '
    /^# Spec: v2-/ {
      if ($0 !~ /^# Spec: v2-[a-z0-9-]+ — /) { printf "ERR malformed spec heading: %s\n", $0; next }
      spec=$3
      next
    }
    /^### [FU]-[0-9]+/ {
      if ($0 !~ /^### [FU]-[0-9]+ — /) { printf "ERR malformed criterion heading: %s\n", $0; next }
      if (spec == "") { printf "ERR criterion heading outside any spec: %s\n", $0; next }
      printf "ID %s/%s\n", spec, $2
      next
    }
  ' "$DELTAS"
}
# Spec set is derived separately so duplicates are caught deterministically.
spec_stream() {
  awk '/^# Spec: v2-[a-z0-9-]+ — / { print $3 }' "$DELTAS" | sort -u
}

declare -a IDS=()
declare -A SEEN=()
TOTAL=0
SPECS=0
while read -r kind value; do
  case "$kind" in
    ID)
      TOTAL=$((TOTAL + 1))
      if [ -n "${SEEN[$value]:-}" ]; then
        fail "duplicate criterion ID: $value"
      fi
      SEEN[$value]=1
      IDS+=("$value")
      ;;
    ERR) fail "$value" ;;
  esac
done < <(parse_stream)

for _ in $(spec_stream); do
  SPECS=$((SPECS + 1))
done

[ "$TOTAL" -eq 92 ] || fail "expected 92 criteria in deltas-acceptance.md, found $TOTAL"
[ "$SPECS" -eq 11 ] || fail "expected 11 specs in deltas-acceptance.md, found $SPECS"

# --- Classification tables ---------------------------------------------------
STRICT_SPEC="v2-flujo-sdd"
declare -A STRICT_TESTS=(
  ["$STRICT_SPEC/F-01"]="TestFlowEachSpecIsSDDChange"
  ["$STRICT_SPEC/F-02"]="TestFlowVerificationViaVerify"
  ["$STRICT_SPEC/F-03"]="TestFlowDeliveryThroughGates"
  ["$STRICT_SPEC/U-01"]="TestFlowCriterionTraceability"
)

# Allowlisted informational debt: spec → (status, F max, U max). The exact ID
# sets below are the approved 59 (broker 7F+2U, ipc 9F+4U, no-regresion
# 14F+3U, adapter 6F+4U, path-claims 7F+3U).
declare -A ALLOWED_STATUS=(
  [v2-broker]="deferred:cli-direct"
  [v2-ipc]="deferred:cli-direct"
  [v2-no-regresion]="archived-pending"
  [v2-adapter]="archived-pending"
  [v2-path-claims]="archived-pending"
)
declare -A ALLOWED_FMAX=(
  [v2-broker]=7 [v2-ipc]=9 [v2-no-regresion]=14 [v2-adapter]=6 [v2-path-claims]=7
)
declare -A ALLOWED_UMAX=(
  [v2-broker]=2 [v2-ipc]=4 [v2-no-regresion]=3 [v2-adapter]=4 [v2-path-claims]=3
)
allowed_total=0
for s in "${!ALLOWED_STATUS[@]}"; do
  allowed_total=$((allowed_total + ALLOWED_FMAX[$s] + ALLOWED_UMAX[$s]))
done
[ "$allowed_total" -eq 59 ] ||
  fail "allowlist drift: expected 59 allowlisted informational IDs, defined $allowed_total"

# Completed specs: not allowlisted; each of their IDs requires archived
# spec evidence (flat or specs/ layout) plus verify-report.md.
COMPLETE_SPECS=" v2-reconciliacion v2-composicion v2-reporte v2-store v2-distribucion "
declare -A ARCHIVE_OK=()

archive_evidence() { # <spec> <full-id>
  local spec="$1" id="$2" dir match_count=0
  [ -n "${ARCHIVE_OK[$spec]:-}" ] && return 0
  local found=""
  for dir in "$ARCHIVE"/*-"$spec"; do
    [ -d "$dir" ] || continue
    match_count=$((match_count + 1))
    found="$dir"
  done
  [ "$match_count" -eq 1 ] ||
    fail "criterion $id: expected exactly one archived change dir for $spec, found $match_count"
  if [ ! -f "$found/spec.md" ] && [ ! -f "$found/specs/$spec/spec.md" ]; then
    fail "criterion $id: archived spec evidence missing for $spec (neither $found/spec.md nor $found/specs/$spec/spec.md)"
  fi
  [ -f "$found/verify-report.md" ] ||
    fail "criterion $id: archived verify-report.md missing for $spec at change-dir level ($found)"
  ARCHIVE_OK[$spec]=1
}

# --- Classify and verify every ID -------------------------------------------
STRICT=0
for id in "${IDS[@]}"; do
  spec="${id%%/*}"
  local_id="${id#*/}"
  kind="${local_id%%-*}"
  num=$((10#${local_id##*-}))

  if [ -n "${STRICT_TESTS[$id]:-}" ]; then
    test_name="${STRICT_TESTS[$id]}"
    grep -q "^func ${test_name}(t \*testing.T) {" "$FLOW_TEST" ||
      fail "criterion $id: required test ${test_name} not found in internal/cmd/flow_test.go"
    echo "strict $id -> $test_name"
    STRICT=$((STRICT + 1))
  elif [ "$spec" = "$STRICT_SPEC" ]; then
    fail "unmapped D09 criterion: $id (D09 requires exactly the four named tests)"
  elif [ -n "${ALLOWED_STATUS[$spec]:-}" ]; then
    case "$kind" in
      F) [ "$num" -le "${ALLOWED_FMAX[$spec]}" ] ||
        fail "unclassified criterion outside the allowlisted set for $spec: $id" ;;
      U) [ "$num" -le "${ALLOWED_UMAX[$spec]}" ] ||
        fail "unclassified criterion outside the allowlisted set for $spec: $id" ;;
      *) fail "unknown criterion kind in $id" ;;
    esac
    echo "info:${ALLOWED_STATUS[$spec]} $id"
  elif [[ "$COMPLETE_SPECS" == *" $spec "* ]]; then
    archive_evidence "$spec" "$id"
    echo "info:archived $id"
  else
    fail "unknown criterion with no classification: $id"
  fi
done

[ "$STRICT" -eq 4 ] || fail "expected 4 strict D09 criteria mapped and passing, found $STRICT"

# --- Run the D09 flow tests (default mode only) ------------------------------
if [ "$CHECK_ONLY" -eq 0 ]; then
  if [ -f "$ROOT/go.mod" ]; then
    echo "== running D09 flow tests =="
    if ! (cd "$ROOT" && go test ./internal/cmd -run '^(TestFlowEachSpecIsSDDChange|TestFlowVerificationViaVerify|TestFlowDeliveryThroughGates|TestFlowCriterionTraceability)$' -count=1); then
      fail "D09 flow tests failed"
    fi
  else
    echo "note: no go.mod under root, skipping flow test execution (fixture mode)"
  fi
fi

INFO=$((TOTAL - STRICT))
echo "TOTAL=$TOTAL STRICT=$STRICT INFO=$INFO"
echo "traceability check passed"
exit 0
