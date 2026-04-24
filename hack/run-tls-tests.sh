#!/usr/bin/env bash
#
# Runs all TLSObservedConfig tests one by one and reports results.
# Usage: ./hack/run-tls-tests.sh [--skip-disruptive]

SKIP_DISRUPTIVE=false
if [[ "${1:-}" == "--skip-disruptive" ]]; then
    SKIP_DISRUPTIVE=true
fi

OPENSHIFT_TESTS="./openshift-tests"
PASSED=0
FAILED=0
SKIPPED=0
FAILURES=""

TESTLIST=$(mktemp)
TESTOUT=$(mktemp)
trap "rm -f $TESTLIST $TESTOUT" EXIT

$OPENSHIFT_TESTS list tests \
    | grep -o '"name": "[^"]*TLSObservedConfig[^"]*"' \
    | sed 's/"name": "//;s/"$//' > "$TESTLIST"

TOTAL=$(wc -l < "$TESTLIST" | tr -d ' ')

echo "=========================================="
echo " TLSObservedConfig Test Runner"
echo " Found $TOTAL tests"
echo "=========================================="
echo ""

NUM=0
while IFS= read -r TEST; do
    NUM=$((NUM + 1))

    if $SKIP_DISRUPTIVE && echo "$TEST" | grep -q '\[Disruptive\]'; then
        echo "[$NUM/$TOTAL] SKIP (disruptive): $TEST"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    echo "----------------------------------------"
    echo "[$NUM/$TOTAL] Running: $TEST"
    echo "----------------------------------------"

    $OPENSHIFT_TESTS run-test "$TEST" < /dev/null > "$TESTOUT" 2>&1 || true

    if grep -q '\[SKIPPED\]' "$TESTOUT"; then
        echo ">>> SKIPPED"
        SKIPPED=$((SKIPPED + 1))
    elif grep -q '"result": "passed"' "$TESTOUT" || grep -q '1 Passed' "$TESTOUT"; then
        echo ">>> PASSED"
        PASSED=$((PASSED + 1))
    else
        grep -E '(FAIL|occurred)' "$TESTOUT" | tail -3
        echo ">>> FAILED"
        FAILED=$((FAILED + 1))
        FAILURES="$FAILURES
  - $TEST"
    fi
    echo ""
done < "$TESTLIST"

echo "=========================================="
echo " RESULTS"
echo "=========================================="
echo " Total:   $TOTAL"
echo " Passed:  $PASSED"
echo " Failed:  $FAILED"
echo " Skipped: $SKIPPED"
echo "=========================================="

if [[ -n "$FAILURES" ]]; then
    echo ""
    echo "Failed tests:$FAILURES"
    exit 1
fi
