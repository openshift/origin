#!/bin/bash
#
# Cleanup script for NVIDIA GPU stack
# Removes DRA Driver and GPU Operator installed by tests
# This script mirrors the UninstallAll logic from prerequisites_installer.go
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

echo "========================================"
echo "NVIDIA GPU Stack Cleanup"
echo "========================================"
echo ""

# Uninstall DRA Driver first (mirrors prerequisites_installer.go UninstallAll)
log_info "Uninstalling NVIDIA DRA Driver..."
if helm uninstall nvidia-dra-driver-gpu \
  --namespace nvidia-dra-driver-gpu \
  --wait \
  --timeout 5m 2>/dev/null; then
    log_info "DRA Driver Helm release uninstalled"
else
    log_warn "DRA Driver Helm release not found or already uninstalled"
fi

# Clean up SCC permissions (ClusterRoleBindings)
log_info "Cleaning up SCC permissions..."
for crb in \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-controller \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-kubeletplugin \
  nvidia-dra-privileged-compute-domain-daemon-service-account; do
    if oc delete clusterrolebinding "$crb" --ignore-not-found=true 2>/dev/null; then
        log_info "Deleted ClusterRoleBinding: $crb"
    fi
done

# Delete DRA Driver namespace
if oc delete namespace nvidia-dra-driver-gpu --ignore-not-found=true 2>/dev/null; then
    log_info "Deleted namespace: nvidia-dra-driver-gpu"
else
    log_warn "Namespace nvidia-dra-driver-gpu not found"
fi

echo ""

# Uninstall GPU Operator
log_info "Uninstalling GPU Operator..."
if helm uninstall gpu-operator \
  --namespace nvidia-gpu-operator \
  --wait \
  --timeout 5m 2>/dev/null; then
    log_info "GPU Operator Helm release uninstalled"
else
    log_warn "GPU Operator Helm release not found or already uninstalled"
fi

# Delete GPU Operator namespace
if oc delete namespace nvidia-gpu-operator --ignore-not-found=true 2>/dev/null; then
    log_info "Deleted namespace: nvidia-gpu-operator"
else
    log_warn "Namespace nvidia-gpu-operator not found"
fi

echo ""

# Clean up test resources (DeviceClasses and test namespaces)
log_info "Cleaning up test resources..."

# Delete any test DeviceClasses (these are cluster-scoped)
TEST_DEVICECLASSES=$(oc get deviceclass -o name 2>/dev/null | grep -E 'nvidia-gpu-test' || true)
if [ -n "$TEST_DEVICECLASSES" ]; then
    log_info "Deleting test DeviceClasses..."
    echo "$TEST_DEVICECLASSES" | xargs oc delete --ignore-not-found=true 2>/dev/null || true
fi

# Delete any test namespaces
TEST_NAMESPACES=$(oc get namespaces -o name 2>/dev/null | grep -E 'nvidia-dra.*test|e2e.*nvidia' || true)
if [ -n "$TEST_NAMESPACES" ]; then
    log_info "Deleting test namespaces..."
    echo "$TEST_NAMESPACES" | xargs oc delete --wait=false --ignore-not-found=true 2>/dev/null || true
fi

echo ""
echo "========================================"
echo "Cleanup Complete"
echo "========================================"
log_info "GPU node labels managed by NFD will be removed automatically"
log_info "ResourceSlices will be cleaned up by the Kubernetes API server"
