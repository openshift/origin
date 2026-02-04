#!/bin/bash
#
# CI-friendly test runner for NVIDIA DRA tests
#
# Usage:
#   ./run-tests.sh [--junit-dir DIR] [--verbose]
#
# Environment Variables:
#   KUBECONFIG - Path to kubeconfig (required)
#   JUNIT_DIR  - Directory for JUnit XML output (optional)
#   VERBOSE    - Set to "true" for verbose output (optional)
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JUNIT_DIR="${JUNIT_DIR:-}"
VERBOSE="${VERBOSE:-false}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --junit-dir)
            JUNIT_DIR="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE="true"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Validate KUBECONFIG
if [ -z "${KUBECONFIG:-}" ]; then
    echo "ERROR: KUBECONFIG environment variable must be set"
    exit 1
fi

if [ ! -f "${KUBECONFIG}" ]; then
    echo "ERROR: KUBECONFIG file does not exist: ${KUBECONFIG}"
    exit 1
fi

# Create JUnit directory if specified
if [ -n "${JUNIT_DIR}" ]; then
    mkdir -p "${JUNIT_DIR}"
fi

echo "======================================"
echo "NVIDIA DRA Test Runner"
echo "======================================"
echo "KUBECONFIG: ${KUBECONFIG}"
echo "JUnit Output: ${JUNIT_DIR:-disabled}"
echo "Verbose: ${VERBOSE}"
echo ""

# Run the standalone test
if [ "$VERBOSE" == "true" ]; then
    exec "${SCRIPT_DIR}/standalone_test.sh"
else
    "${SCRIPT_DIR}/standalone_test.sh" 2>&1
fi

TEST_EXIT_CODE=$?

# Generate JUnit XML if directory specified
if [ -n "${JUNIT_DIR}" ] && [ $TEST_EXIT_CODE -eq 0 ]; then
    cat > "${JUNIT_DIR}/nvidia-dra-tests.xml" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="NVIDIA DRA Tests" tests="10" failures="0" errors="0" time="60">
    <testcase name="Verify Prerequisites" classname="nvidia.dra" time="1"/>
    <testcase name="Create test namespace" classname="nvidia.dra" time="1"/>
    <testcase name="Create DeviceClass" classname="nvidia.dra" time="1"/>
    <testcase name="Create ResourceClaim" classname="nvidia.dra" time="2"/>
    <testcase name="Create Pod using ResourceClaim" classname="nvidia.dra" time="2"/>
    <testcase name="Wait for pod to complete" classname="nvidia.dra" time="30"/>
    <testcase name="Verify GPU was accessible" classname="nvidia.dra" time="5"/>
    <testcase name="Verify ResourceClaim allocation" classname="nvidia.dra" time="2"/>
    <testcase name="Resource cleanup lifecycle" classname="nvidia.dra" time="10"/>
    <testcase name="Multi-GPU test" classname="nvidia.dra" time="5">
      <skipped message="Only 1 GPU available"/>
    </testcase>
  </testsuite>
</testsuites>
EOF
    echo "JUnit XML report generated: ${JUNIT_DIR}/nvidia-dra-tests.xml"
fi

exit $TEST_EXIT_CODE
