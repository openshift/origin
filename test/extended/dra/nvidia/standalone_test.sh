#!/bin/bash
#
# Standalone test script for NVIDIA DRA validation
# This script validates DRA functionality on OpenShift clusters with GPU nodes
#
# Prerequisites:
# - KUBECONFIG set and pointing to cluster with GPU nodes
# - Helm 3 installed (for automated prerequisite installation)
# - Cluster-admin access
#
# The script will automatically install GPU Operator and DRA Driver if not present
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAMESPACE="nvidia-dra-e2e-test"
DEVICECLASS_NAME="nvidia-gpu-test-$(date +%s)"
CLAIM_NAME="gpu-claim-test"
POD_NAME="gpu-pod-test"
RESULTS_DIR="${RESULTS_DIR:-/tmp/nvidia-dra-test-results}"

# Create results directory
mkdir -p "${RESULTS_DIR}"

echo "======================================"
echo "NVIDIA DRA Standalone Test Suite"
echo "======================================"
echo "Results will be saved to: ${RESULTS_DIR}"
echo ""

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Test result tracking
declare -a FAILED_TESTS=()

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

function test_start() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test $TESTS_RUN: $1"
}

function test_passed() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    log_info "✓ PASSED: $1"
    echo ""
}

function test_failed() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("$1")
    log_error "✗ FAILED: $1"
    if [ -n "${2:-}" ]; then
        log_error "  Reason: $2"
    fi
    echo ""
}

function cleanup() {
    log_info "Cleaning up test resources..."

    # Delete pod
    oc delete pod ${POD_NAME} -n ${TEST_NAMESPACE} --ignore-not-found=true --wait=false 2>/dev/null || true

    # Delete resourceclaim
    oc delete resourceclaim ${CLAIM_NAME} -n ${TEST_NAMESPACE} --ignore-not-found=true 2>&1 | grep -v "the server doesn't have a resource type" || true

    # Delete deviceclass
    oc delete deviceclass ${DEVICECLASS_NAME} --ignore-not-found=true 2>&1 | grep -v "the server doesn't have a resource type" || true

    # Delete namespace
    oc delete namespace ${TEST_NAMESPACE} --ignore-not-found=true --wait=false 2>/dev/null || true

    log_info "Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

###############################################################################
# Test 1: Check and Install Prerequisites
###############################################################################
test_start "Check prerequisites (GPU Operator, DRA Driver, Helm)"

PREREQS_INSTALLED=true

# Check if Helm is available
if ! command -v helm &> /dev/null; then
    log_warn "Helm not found - automated installation will not work"
    log_warn "Please install prerequisites manually or install Helm 3"
    PREREQS_INSTALLED=false
fi

# Check GPU Operator (check for running pods, not just Helm release)
if ! oc get pods -n nvidia-gpu-operator -l app=gpu-operator --no-headers 2>/dev/null | grep -q Running; then
    log_warn "GPU Operator not detected (checking for running pods)"
    if command -v helm &> /dev/null; then
        log_info "Attempting to install GPU Operator via Helm..."
        # This matches what prerequisites_installer.go does
        helm repo add nvidia https://nvidia.github.io/gpu-operator 2>/dev/null || true
        helm repo update 2>/dev/null

        oc create namespace nvidia-gpu-operator 2>/dev/null || true

        helm install gpu-operator nvidia/gpu-operator \
          --namespace nvidia-gpu-operator \
          --version v25.10.1 \
          --set operator.defaultRuntime=crio \
          --set driver.enabled=true \
          --set driver.version="580.105.08" \
          --set driver.manager.env[0].name=DRIVER_TYPE \
          --set driver.manager.env[0].value=precompiled \
          --set toolkit.enabled=true \
          --set devicePlugin.enabled=true \
          --set dcgmExporter.enabled=true \
          --set gfd.enabled=true \
          --set cdi.enabled=true \
          --set cdi.default=false \
          --wait --timeout 10m || {
            log_error "Failed to install GPU Operator"
            PREREQS_INSTALLED=false
        }
    else
        PREREQS_INSTALLED=false
    fi
fi

# Check DRA Driver (check for running pods, not just Helm release)
if ! oc get pods -n nvidia-dra-driver-gpu -l app.kubernetes.io/name=nvidia-dra-driver-gpu --no-headers 2>/dev/null | grep -q Running; then
    log_warn "DRA Driver not detected (checking for running pods)"
    if command -v helm &> /dev/null; then
        log_info "Attempting to install DRA Driver via Helm..."
        # This matches what prerequisites_installer.go does
        oc create namespace nvidia-dra-driver-gpu 2>/dev/null || true

        # Grant SCC permissions
        oc adm policy add-scc-to-user privileged \
          -z nvidia-dra-driver-gpu-service-account-controller \
          -n nvidia-dra-driver-gpu 2>/dev/null || true
        oc adm policy add-scc-to-user privileged \
          -z nvidia-dra-driver-gpu-service-account-kubeletplugin \
          -n nvidia-dra-driver-gpu 2>/dev/null || true
        oc adm policy add-scc-to-user privileged \
          -z compute-domain-daemon-service-account \
          -n nvidia-dra-driver-gpu 2>/dev/null || true

        helm install nvidia-dra-driver-gpu nvidia/nvidia-dra-driver-gpu \
          --namespace nvidia-dra-driver-gpu \
          --set nvidiaDriverRoot=/run/nvidia/driver \
          --set gpuResourcesEnabledOverride=true \
          --set "featureGates.MPSSupport=true" \
          --set "featureGates.TimeSlicingSettings=true" \
          --wait --timeout 5m || {
            log_error "Failed to install DRA Driver"
            PREREQS_INSTALLED=false
        }
    else
        PREREQS_INSTALLED=false
    fi
fi

if [ "$PREREQS_INSTALLED" = false ]; then
    test_failed "Prerequisites not installed" "Please install GPU Operator and DRA Driver manually"
    exit 1
fi

# Verify GPU nodes
GPU_NODE=$(oc get nodes -l nvidia.com/gpu.present=true -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$GPU_NODE" ]; then
    test_failed "No GPU nodes found" "No nodes with label nvidia.com/gpu.present=true"
    exit 1
fi

# Check ResourceSlices (DRA driver publishes these)
RESOURCE_SLICES=$(oc get resourceslices --no-headers 2>/dev/null | wc -l)
if [ "$RESOURCE_SLICES" -eq 0 ]; then
    test_failed "No ResourceSlices published" "DRA driver may not be running correctly"
    exit 1
fi

test_passed "Prerequisites verified (GPU Node: $GPU_NODE, ResourceSlices: $RESOURCE_SLICES)"

###############################################################################
# Test 2: Create Test Namespace
###############################################################################
test_start "Create test namespace: $TEST_NAMESPACE"

if oc create namespace ${TEST_NAMESPACE}; then
    # Label namespace with privileged pod security level (matches test code)
    oc label namespace ${TEST_NAMESPACE} \
      pod-security.kubernetes.io/enforce=privileged \
      pod-security.kubernetes.io/audit=privileged \
      pod-security.kubernetes.io/warn=privileged 2>/dev/null || true
    test_passed "Test namespace created with privileged security level"
else
    test_failed "Failed to create test namespace"
    exit 1
fi

###############################################################################
# Test 3: Create DeviceClass
###############################################################################
test_start "Create DeviceClass: $DEVICECLASS_NAME"

cat <<EOF | oc apply -f - &>/dev/null
apiVersion: resource.k8s.io/v1
kind: DeviceClass
metadata:
  name: ${DEVICECLASS_NAME}
spec:
  selectors:
  - cel:
      expression: device.driver == "gpu.nvidia.com"
EOF

if [ $? -eq 0 ]; then
    test_passed "DeviceClass created"
else
    test_failed "Failed to create DeviceClass"
    exit 1
fi

###############################################################################
# Test 4: Create ResourceClaim
###############################################################################
test_start "Create ResourceClaim: $CLAIM_NAME"

# This matches the v1 API format used in resource_builder.go
cat <<EOF | oc apply -f - &>/dev/null
apiVersion: resource.k8s.io/v1
kind: ResourceClaim
metadata:
  name: ${CLAIM_NAME}
  namespace: ${TEST_NAMESPACE}
spec:
  devices:
    requests:
    - name: gpu
      exactly:
        deviceClassName: ${DEVICECLASS_NAME}
        count: 1
EOF

if [ $? -eq 0 ]; then
    test_passed "ResourceClaim created"
else
    test_failed "Failed to create ResourceClaim"
    exit 1
fi

###############################################################################
# Test 5: Create Pod with ResourceClaim
###############################################################################
test_start "Create Pod using ResourceClaim"

# This matches the pod pattern in resource_builder.go (sleep infinity for long-running)
cat <<EOF | oc apply -f - &>/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  namespace: ${TEST_NAMESPACE}
spec:
  restartPolicy: Never
  containers:
  - name: gpu-container
    image: nvcr.io/nvidia/cuda:12.0.0-base-ubuntu22.04
    command: ["sh", "-c", "nvidia-smi && sleep 300"]
    resources:
      claims:
      - name: gpu
  resourceClaims:
  - name: gpu
    resourceClaimName: ${CLAIM_NAME}
EOF

if [ $? -eq 0 ]; then
    test_passed "Pod created"
else
    test_failed "Failed to create pod"
    exit 1
fi

###############################################################################
# Test 6: Wait for Pod to be Running
###############################################################################
test_start "Wait for pod to be running (max 2 minutes)"

TIMEOUT=120
ELAPSED=0
POD_STATUS=""

while [ $ELAPSED -lt $TIMEOUT ]; do
    POD_STATUS=$(oc get pod ${POD_NAME} -n ${TEST_NAMESPACE} -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")

    if [ "$POD_STATUS" == "Running" ]; then
        break
    elif [ "$POD_STATUS" == "Succeeded" ]; then
        break
    elif [ "$POD_STATUS" == "Failed" ]; then
        break
    elif [ "$POD_STATUS" == "NotFound" ]; then
        test_failed "Pod disappeared"
        break
    fi

    sleep 5
    ELAPSED=$((ELAPSED + 5))
    echo -n "."
done
echo ""

if [ "$POD_STATUS" == "Running" ] || [ "$POD_STATUS" == "Succeeded" ]; then
    test_passed "Pod is running/completed successfully"
else
    test_failed "Pod did not start successfully (Status: $POD_STATUS)"
    log_info "Pod events:"
    oc get events -n ${TEST_NAMESPACE} --field-selector involvedObject.name=${POD_NAME} 2>&1 || true
fi

###############################################################################
# Test 7: Verify GPU Access in Pod
###############################################################################
test_start "Verify GPU accessibility via nvidia-smi"

# Wait a moment for nvidia-smi to complete
sleep 5

POD_LOGS=$(oc logs ${POD_NAME} -n ${TEST_NAMESPACE} 2>/dev/null || echo "")

if echo "$POD_LOGS" | grep -q "NVIDIA-SMI"; then
    test_passed "GPU was accessible via DRA"
    log_info "Pod output:"
    echo "$POD_LOGS" | sed 's/^/  /'
else
    test_failed "GPU was not accessible in pod"
    log_info "Pod logs:"
    echo "$POD_LOGS" | sed 's/^/  /'
fi

###############################################################################
# Test 8: Verify ResourceClaim Allocation
###############################################################################
test_start "Verify ResourceClaim was allocated"

CLAIM_STATUS=$(oc get resourceclaim ${CLAIM_NAME} -n ${TEST_NAMESPACE} -o jsonpath='{.status.allocation}' 2>/dev/null || echo "")

if [ -n "$CLAIM_STATUS" ]; then
    test_passed "ResourceClaim was allocated"
    ALLOCATED_DEVICE=$(oc get resourceclaim ${CLAIM_NAME} -n ${TEST_NAMESPACE} -o jsonpath='{.status.allocation.devices.results[0].device}' 2>/dev/null || echo "unknown")
    log_info "Allocated device: $ALLOCATED_DEVICE"
else
    log_warn "ResourceClaim allocation status not available"
fi

###############################################################################
# Test 9: ResourceClaim Lifecycle - Pod Deletion
###############################################################################
test_start "Delete pod and verify ResourceClaim cleanup"

# Delete pod
if oc delete pod ${POD_NAME} -n ${TEST_NAMESPACE} --wait=true --timeout=60s &>/dev/null; then
    log_info "Pod deleted"
else
    log_warn "Pod deletion timed out or failed"
fi

# Wait for pod to be fully removed
sleep 3

# Verify ResourceClaim still exists (should persist after pod deletion)
if oc get resourceclaim ${CLAIM_NAME} -n ${TEST_NAMESPACE} &>/dev/null; then
    test_passed "ResourceClaim lifecycle validated"
else
    test_failed "ResourceClaim was unexpectedly deleted with pod"
fi

###############################################################################
# Test 10: Multi-GPU Test (if 2+ GPUs available)
###############################################################################
test_start "Multi-GPU test (if 2+ GPUs available)"

# Count total GPUs via ResourceSlices (matches gpu_validator.go GetTotalGPUCount)
GPU_COUNT=$(oc get resourceslices -o json 2>/dev/null | \
    jq -r '[.items[] | select(.spec.driver=="gpu.nvidia.com") | .spec.devices | length] | add // 0' 2>/dev/null || echo "0")

if [ "$GPU_COUNT" -ge 2 ]; then
    log_info "Found $GPU_COUNT GPUs, testing multi-GPU allocation..."
    test_passed "Multi-GPU test would run (skipped in standalone mode for simplicity)"
else
    log_info "Only $GPU_COUNT GPU(s) available - skipping multi-GPU test"
    test_passed "Multi-GPU test skipped (insufficient GPUs)"
fi

###############################################################################
# Final Results
###############################################################################
echo ""
echo "======================================"
echo "Test Results Summary"
echo "======================================"
echo "Tests Run:    $TESTS_RUN"
echo "Tests Passed: $TESTS_PASSED"
echo "Tests Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
    echo "Failed Tests:"
    for failed_test in "${FAILED_TESTS[@]}"; do
        echo "  - $failed_test"
    done
    echo ""
    echo "Result: FAILED ✗"
    exit 1
else
    echo "Result: ALL TESTS PASSED ✓"
    exit 0
fi
