#!/usr/bin/env bash
set -euo pipefail

COVERAGE_FILE="${1:-coverage.out}"
REQUIRED_THRESHOLD="${2:-60.0}"

if [ ! -f "$COVERAGE_FILE" ]; then
  echo "::error::Coverage profile '$COVERAGE_FILE' does not exist!"
  exit 1
fi

echo "============================================================"
echo "    MS1 (jira-issues-go) Statement Coverage Verification    "
echo "============================================================"

TOTAL_LINE=$(go tool cover -func="$COVERAGE_FILE" | grep -E '^total:\s+\(statements\)' || true)

if [ -z "$TOTAL_LINE" ]; then
  echo "::error::Failed to extract total statement coverage line from '$COVERAGE_FILE'!"
  exit 1
fi

COVERAGE=$(echo "$TOTAL_LINE" | awk '{print $NF}' | tr -d '%')

echo "Current Statement Coverage : ${COVERAGE}%"
echo "Required Minimum Threshold : ${REQUIRED_THRESHOLD}%"
echo "------------------------------------------------------------"

PASS=$(awk -v cov="$COVERAGE" -v req="$REQUIRED_THRESHOLD" 'BEGIN { if (cov >= req) print 1; else print 0 }')

if [ "$PASS" -eq 1 ]; then
  echo "[SUCCESS] Coverage threshold (${REQUIRED_THRESHOLD}%) satisfied!"
  exit 0
else
  echo "[FAILURE] Statement coverage (${COVERAGE}%) is below required threshold (${REQUIRED_THRESHOLD}%)!"
  exit 1
fi
