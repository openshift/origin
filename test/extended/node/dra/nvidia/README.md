# NVIDIA DRA Extended Tests for OpenShift

This directory contains extended tests for NVIDIA Dynamic Resource Allocation (DRA) functionality on OpenShift clusters with GPU nodes.

## Overview

These tests validate:
- NVIDIA DRA driver installation and lifecycle
- Single GPU allocation via ResourceClaims
- Multi-GPU workload allocation
- Pod lifecycle and resource cleanup
- GPU device accessibility in pods

## Prerequisites

1. **OpenShift 4.21+** with GPU-enabled worker nodes
2. **Node Feature Discovery Operator** pre-installed
3. **NVIDIA GPU Operator** pre-installed with **CDI enabled** (`cdi.enabled=true`)
4. **Helm 3** installed and available in PATH
5. **Cluster-admin** access

For installation instructions, see the [NVIDIA GPU Operator on OpenShift Installation Guide](https://docs.nvidia.com/datacenter/cloud-native/openshift/latest/install-gpu-ocp.html).

**Note**: The test framework automatically installs the DRA driver if not already present.

## Quick Start

### Running Tests

```bash
# 1. Build test binary
# Note: Your origin checkout should match the cluster version to avoid version mismatch errors
make WHAT=cmd/openshift-tests

# 2. Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# 3. Run all NVIDIA DRA tests
./openshift-tests run --dry-run all 2>&1 | \
  grep "\[Feature:NVIDIA-DRA\]" | \
  ./openshift-tests run -f -

# OR run a specific test
./openshift-tests run-test \
  '[sig-scheduling][Feature:NVIDIA-DRA][Serial] Basic GPU Allocation should allocate single GPU to pod via DRA'

# OR list all available tests without running them
./openshift-tests run --dry-run all 2>&1 | grep "\[Feature:NVIDIA-DRA\]"
```

**What the tests do automatically:**
- Verify GPU Operator is installed (test fails if not present)
- Install DRA Driver if not already present (version 25.12.0 by default)
- Configure necessary SCC permissions and node labels
- Wait for all components to be ready before running tests

To use a different DRA driver version: `export NVIDIA_DRA_DRIVER_VERSION=25.8.1`

### Running Individual Tests

```bash
# Set your kubeconfig first
export KUBECONFIG=/path/to/kubeconfig

# Discover and run all NVIDIA DRA tests sequentially
./openshift-tests run --dry-run all 2>&1 | grep "\[Feature:NVIDIA-DRA\]" | while read -r test; do
  echo "Running: $test"
  ./openshift-tests run-test "$test"
done
```

## Test Scenarios

### 1. Single GPU Allocation ✅
- Creates DeviceClass with CEL selector
- Creates ResourceClaim requesting exactly 1 GPU
- Schedules pod with ResourceClaim
- Validates GPU accessibility via nvidia-smi
- Validates CDI device injection

**Expected**: PASSED

### 2. Resource Cleanup ✅
- Creates pod with GPU ResourceClaim
- Deletes pod
- Verifies ResourceClaim persists after pod deletion
- Validates resource lifecycle management

**Expected**: PASSED

### 3. Multi-GPU Workloads ⚠️
- Creates ResourceClaim requesting exactly 2 GPUs
- Schedules pod requiring multiple GPUs
- Validates all GPUs are accessible

**Expected**: SKIPPED if cluster has fewer than 2 GPUs (expected behavior)

### 4. Claim Sharing 🔄
- Creates a single ResourceClaim
- Creates two pods referencing the same ResourceClaim
- Tests whether NVIDIA DRA driver supports claim sharing
- Validates behavior when multiple pods attempt to use the same claim

**Expected**: Behavior depends on driver support for claim sharing. Test verifies that:
- If sharing is NOT supported: Second pod remains Pending, first pod continues to work
- If sharing IS supported: Both pods run and have GPU access

### 5. ResourceClaimTemplate 📋
- Creates a ResourceClaimTemplate
- Creates pod with ResourceClaimTemplate reference
- Validates that ResourceClaim is automatically created from template
- Verifies GPU access in pod
- Validates automatic cleanup of template-generated claim when pod is deleted

**Expected**: PASSED

## Manual DRA Driver Installation

The tests automatically install the DRA driver if needed. This section is for manually installing or debugging the DRA driver outside of the test framework.

### Step 1: Add NVIDIA Helm Repository

```bash
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia --force-update
helm repo update
```

### Step 2: Label GPU Nodes

```bash
# Label all GPU nodes for DRA kubelet plugin scheduling
for node in $(oc get nodes -l nvidia.com/gpu.present=true -o name); do
  oc label $node nvidia.com/dra-kubelet-plugin=true --overwrite
done

# Verify labels
oc get nodes -l nvidia.com/dra-kubelet-plugin=true
```

This label ensures the DRA kubelet plugin only runs on GPU nodes and works around NVIDIA Driver Manager eviction issues.

### Step 3: Install DRA Driver

```bash
# Create namespace
oc create namespace nvidia-dra-driver

# Grant SCC permissions
oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-controller \
  -n nvidia-dra-driver
oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-kubeletplugin \
  -n nvidia-dra-driver
oc adm policy add-scc-to-user privileged \
  -z compute-domain-daemon-service-account \
  -n nvidia-dra-driver

# Install via Helm (pinned to version used by tests)
# Version can be overridden via NVIDIA_DRA_DRIVER_VERSION environment variable
NVIDIA_DRA_DRIVER_VERSION=${NVIDIA_DRA_DRIVER_VERSION:-25.12.0}
helm install nvidia-dra-driver nvidia/nvidia-dra-driver-gpu \
  --namespace nvidia-dra-driver \
  --version ${NVIDIA_DRA_DRIVER_VERSION} \
  --set nvidiaDriverRoot=/run/nvidia/driver \
  --set gpuResourcesEnabledOverride=true \
  --set image.pullPolicy=IfNotPresent \
  --set-string kubeletPlugin.nodeSelector.nvidia\.com/dra-kubelet-plugin=true \
  --set controller.tolerations[0].key=node-role.kubernetes.io/master \
  --set controller.tolerations[0].operator=Exists \
  --set controller.tolerations[0].effect=NoSchedule \
  --set controller.tolerations[1].key=node-role.kubernetes.io/control-plane \
  --set controller.tolerations[1].operator=Exists \
  --set controller.tolerations[1].effect=NoSchedule \
  --wait --timeout 5m
```

### Step 4: Verify Installation

```bash
# Check DRA driver pods
oc get pods -n nvidia-dra-driver
# Expected: All pods should be Running

# Verify ResourceSlices are published
oc get resourceslices
# Should show at least 2 slices per GPU node
```

### Uninstalling DRA Driver

```bash
# Uninstall DRA Driver
helm uninstall nvidia-dra-driver -n nvidia-dra-driver --wait --timeout 5m
oc delete namespace nvidia-dra-driver

# Remove SCC permissions
oc delete clusterrolebinding \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-controller \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-kubeletplugin \
  nvidia-dra-privileged-compute-domain-daemon-service-account
```

**Note**: GPU Operator is NOT removed as it's cluster infrastructure. ResourceSlices are automatically cleaned up.

## Troubleshooting

### GPU Operator not found

**Cause**: GPU Operator not installed on cluster.

**Solution**: Install GPU Operator following the [official guide](https://docs.nvidia.com/datacenter/cloud-native/openshift/latest/install-gpu-ocp.html).

### Version mismatch error

**Cause**: Local origin checkout doesn't match cluster's release commit.

**Solution**: Ensure your origin checkout matches the cluster version, or use `./openshift-tests run-test` command which bypasses version checks.

### DRA driver kubelet plugin stuck at Init:0/1

**Cause**: Wrong `nvidiaDriverRoot` setting.

**Solution**: Ensure `nvidiaDriverRoot=/run/nvidia/driver` (not `/`). This is automatically configured by tests.

### ResourceSlices not appearing

**Cause**: DRA driver not fully initialized or missing SCC permissions.

**Solution**:
```bash
# Check DRA driver logs
oc logs -n nvidia-dra-driver -l app.kubernetes.io/name=nvidia-dra-driver-gpu --all-containers

# Verify SCC permissions
oc describe scc privileged | grep nvidia-dra-driver-gpu

# Restart DRA driver if needed
oc delete pod -n nvidia-dra-driver -l app.kubernetes.io/name=nvidia-dra-driver-gpu
```

## References

- **NVIDIA GPU Operator**: https://github.com/NVIDIA/gpu-operator
- **NVIDIA DRA Driver**: https://github.com/NVIDIA/k8s-dra-driver-gpu
- **Kubernetes DRA Documentation**: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
- **OpenShift Extended Tests**: https://github.com/openshift/origin/tree/master/test/extended

---

**Tested On**: OpenShift 4.21.0, Kubernetes 1.34.2, Tesla T4
