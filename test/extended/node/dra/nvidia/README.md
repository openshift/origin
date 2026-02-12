# NVIDIA DRA Extended Tests for OpenShift

This directory contains extended tests for NVIDIA Dynamic Resource Allocation (DRA) functionality on OpenShift clusters with GPU nodes.

## Overview

These tests validate:
- NVIDIA DRA driver installation and lifecycle
- Single GPU allocation via ResourceClaims
- Multi-GPU workload allocation
- Pod lifecycle and resource cleanup
- GPU device accessibility in pods

## ⚠️ Important: Version Matching Requirement

**CRITICAL**: The `openshift-tests` binary version MUST match the cluster's release image version. This is a design requirement of the OpenShift test framework.

### Why Version Matching is Required

The OpenShift test framework has a two-layer architecture:
1. **Local binary**: Your built `openshift-tests` binary
2. **Cluster release image**: The version of OpenShift running on your cluster

When tests run, the framework attempts to extract component-specific test binaries from the cluster's release image. If versions don't match, you'll see errors like:

```
error: couldn't retrieve test suites: failed to extract test binaries
note the version of origin needs to match the version of the cluster under test
```

### How to Match Versions

#### Step 1: Find Your Cluster's Release Commit

```bash
# Set your kubeconfig
export KUBECONFIG=/path/to/your/kubeconfig

# Get the cluster version
oc get clusterversion version -o jsonpath='{.status.desired.version}'
# Example output: 4.21.0

# Get the exact origin commit used for this release
oc adm release info $(oc get clusterversion version -o jsonpath='{.status.desired.image}') \
  --commits | grep "^origin"

# Example output:
# origin    https://github.com/openshift/origin    1d23a96bb921ad1ceffaaed8bf295d26626f87d5
```

#### Step 2: Checkout the Matching Commit

```bash
cd /path/to/origin

# Checkout the cluster's commit (use the commit from Step 1)
git checkout 1d23a96bb921ad1ceffaaed8bf295d26626f87d5

# Create a working branch for your NVIDIA DRA tests
git checkout -b nvidia-dra-ocp-4.21.0

# Now add your NVIDIA DRA test code to this branch
# (cherry-pick commits, copy files, or apply patches as needed)
```

#### Step 3: Verify Version Match

After building, verify the versions match:

```bash
# Build the test binary
make WHAT=cmd/openshift-tests

# Check binary version
./openshift-tests version 2>&1 | grep "openshift-tests"
# Example: openshift-tests v4.1.0-10527-g1d23a96

# The commit hash (g1d23a96) should match your cluster's commit
```

### Alternative: Using run-test Command

The `run-test` command bypasses the release image extraction and runs tests directly from your local binary:

```bash
# This works even with version mismatch
./openshift-tests run-test -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]'
```

## Prerequisites

### Automatically Installed by Tests

The tests will **automatically install** the following prerequisites if not already present:
- NVIDIA GPU Operator v25.10.1 (via Helm)
- NVIDIA DRA Driver v25.8.1 (via Helm)
- All required SCC permissions
- Helm repository configuration

The test framework intelligently detects existing installations (whether installed via Helm or OLM) and skips installation if components are already running.

### Required Before Running Tests

1. **OpenShift cluster** with GPU-enabled worker nodes (OCP 4.19+)
   - Tested on OCP 4.21.0 with Kubernetes 1.34.2
2. **Helm 3** installed and available in PATH
3. **GPU hardware** present on worker nodes
   - Tested with NVIDIA Tesla T4 (g4dn.xlarge on AWS)
4. **Cluster-admin** access for test execution
5. **Matching origin checkout** (see Version Matching section above)

## Test Structure

```
test/extended/node/dra/nvidia/
├── nvidia_dra.go             # Main test suite (Ginkgo) - extended test format
├── prerequisites_installer.go # Automated prerequisite installation
├── driver_installer.go        # Legacy DRA driver helpers (compatibility)
├── gpu_validator.go           # GPU validation utilities
├── resource_builder.go        # DRA resource builders (DeviceClass, ResourceClaim, Pod)
├── fixtures/                  # YAML test fixtures
│   ├── deviceclass-nvidia.yaml
│   ├── resourceclaim-single-gpu.yaml
│   ├── resourceclaim-multi-gpu.yaml
│   ├── pod-single-gpu.yaml
│   └── pod-multi-gpu.yaml
├── standalone_test.sh         # Standalone validation script
├── cleanup.sh                 # Cleanup utility
└── README.md                  # This file
```

## Quick Start - Running Tests via openshift-tests

### Option 1: Fully Automated (Recommended)

```bash
# 1. Match your origin checkout to cluster version (see Version Matching section above)
cd /path/to/origin
git checkout <cluster-commit-hash>
git checkout -b nvidia-dra-ocp-<version>

# 2. Ensure NVIDIA DRA test code is present in test/extended/node/dra/nvidia/

# 3. Build test binary
make WHAT=cmd/openshift-tests

# 4. Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# 5. Run all NVIDIA DRA tests
./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]'

./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should handle pod deletion and resource cleanup [Suite:openshift/conformance/parallel]'

./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Multi-GPU Workloads should allocate multiple GPUs to single pod [Suite:openshift/conformance/parallel]'
```

**What happens automatically:**
1. Tests check if GPU Operator is already installed (via pods, not just Helm releases)
2. Tests check if DRA Driver is already installed (via pods, not just Helm releases)
3. If not found, Helm repository is added (`nvidia` repo)
4. GPU Operator v25.10.1 is installed via Helm with OpenShift-specific settings
5. Waits for GPU Operator to be ready (drivers, device plugin, NFD)
6. DRA Driver is installed via Helm with correct `nvidiaDriverRoot` setting
7. SCC permissions are granted to DRA service accounts
8. Waits for DRA Driver to be ready (controller + kubelet plugin)
9. Tests execute against the configured GPU stack

**Re-running tests:** Prerequisites are automatically skipped if already installed (detection works with both Helm and OLM installations).

### Option 2: List Available Tests

```bash
# List all NVIDIA DRA tests
./openshift-tests run --dry-run all 2>&1 | grep "NVIDIA DRA"

# Example output:
# "[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]"
# "[sig-scheduling] NVIDIA DRA Basic GPU Allocation should handle pod deletion and resource cleanup [Suite:openshift/conformance/parallel]"
# "[sig-scheduling] NVIDIA DRA Multi-GPU Workloads should allocate multiple GPUs to single pod [Suite:openshift/conformance/parallel]"
```

### Option 3: Run Standalone Validation (No Framework)

For quick manual validation without the test framework:

```bash
cd test/extended/node/dra/nvidia
export KUBECONFIG=/path/to/kubeconfig
./standalone_test.sh
```

**Features**: The standalone script now includes:
- **Automated prerequisite installation** (GPU Operator and DRA Driver via Helm)
- Detection of existing installations (via running pods, not just Helm releases)
- Complete end-to-end validation (10 test scenarios)
- Detailed test result reporting
- Automatic cleanup on exit

**Note**: Requires Helm 3 for automated installation. If Helm is not available, prerequisites must be pre-installed manually.

## Standalone Test Suite

The `standalone_test.sh` script provides a complete validation suite that mirrors the functionality of the openshift-tests framework tests, but can run independently without requiring the test framework build.

### Features

- **Automated Installation**: Automatically installs GPU Operator and DRA Driver via Helm if not present
- **Smart Detection**: Detects existing installations by checking for running pods (not just Helm releases)
- **Complete Validation**: Runs 10 comprehensive test scenarios
- **Detailed Reporting**: Color-coded output with pass/fail tracking
- **Automatic Cleanup**: Cleans up test resources on exit (via trap)

### Test Coverage

The standalone script runs the following tests:

1. **Prerequisites Check** - Verifies Helm, GPU Operator, DRA Driver, GPU nodes, and ResourceSlices
2. **Namespace Creation** - Creates test namespace with privileged pod security level
3. **DeviceClass Creation** - Creates DeviceClass with CEL selector for `gpu.nvidia.com`
4. **ResourceClaim Creation** - Creates ResourceClaim using v1 API with `exactly` field
5. **Pod Creation** - Creates pod with ResourceClaim reference
6. **Pod Scheduling** - Waits for pod to reach Running/Succeeded state (2 minute timeout)
7. **GPU Access Validation** - Verifies nvidia-smi output shows accessible GPU
8. **ResourceClaim Allocation** - Validates ResourceClaim allocation status
9. **Lifecycle Testing** - Tests pod deletion and ResourceClaim persistence
10. **Multi-GPU Detection** - Checks if cluster has 2+ GPUs for multi-GPU testing

### Running the Standalone Tests

```bash
cd test/extended/node/dra/nvidia
export KUBECONFIG=/path/to/kubeconfig

# Run with default results directory (/tmp/nvidia-dra-test-results)
./standalone_test.sh

# Run with custom results directory
RESULTS_DIR=/my/results/path ./standalone_test.sh
```

### Example Output

```
======================================
NVIDIA DRA Standalone Test Suite
======================================
Results will be saved to: /tmp/nvidia-dra-test-results

[INFO] Test 1: Check prerequisites (GPU Operator, DRA Driver, Helm)
[INFO] ✓ PASSED: Prerequisites verified (GPU Node: ip-10-0-10-28, ResourceSlices: 2)

[INFO] Test 2: Create test namespace: nvidia-dra-e2e-test
[INFO] ✓ PASSED: Test namespace created with privileged security level

[INFO] Test 3: Create DeviceClass: nvidia-gpu-test-1738672800
[INFO] ✓ PASSED: DeviceClass created

...

======================================
Test Results Summary
======================================
Tests Run:    10
Tests Passed: 9
Tests Failed: 0

Result: ALL TESTS PASSED ✓
```

### Prerequisites

The standalone script requires:
- **Helm 3** - For automated installation (if prerequisites not already present)
- **Cluster-admin access** - For SCC permissions and namespace creation
- **GPU-enabled cluster** - OpenShift cluster with GPU worker nodes
- **Internet access** - To pull Helm charts and container images (if installing prerequisites)

If Helm is not available, prerequisites must be pre-installed manually (see Manual Installation Reference section).

## Test Scenarios

### 1. Single GPU Allocation ✅
- Creates DeviceClass with CEL selector
- Creates ResourceClaim requesting exactly 1 GPU
- Schedules pod with ResourceClaim
- Validates GPU accessibility via nvidia-smi
- Validates CDI device injection

**Expected Result**: PASSED

### 2. Resource Cleanup ✅
- Creates pod with GPU ResourceClaim
- Deletes pod
- Verifies ResourceClaim persists after pod deletion
- Validates resource lifecycle management

**Expected Result**: PASSED

### 3. Multi-GPU Workloads ⚠️
- Creates ResourceClaim requesting exactly 2 GPUs
- Schedules pod requiring multiple GPUs
- Validates all GPUs are accessible

**Expected Result**: SKIPPED if cluster has fewer than 2 GPUs on a single node (expected behavior)

## Manual Installation Reference

The following steps document what the automated test code does. Use this as a reference for:
- Understanding the automated installation process
- Manually pre-installing prerequisites (optional)
- Debugging installation issues
- CI job configuration

### Prerequisites for Manual Installation

```bash
# Verify Helm 3 is installed
helm version

# If not installed, install Helm 3
curl -fsSL https://get.helm.sh/helm-v3.20.0-linux-amd64.tar.gz -o /tmp/helm.tar.gz
tar -zxvf /tmp/helm.tar.gz -C /tmp
sudo mv /tmp/linux-amd64/helm /usr/local/bin/helm
rm -rf /tmp/helm.tar.gz /tmp/linux-amd64
```

### Step 1: Add NVIDIA Helm Repository

```bash
# Add NVIDIA Helm repository
helm repo add nvidia https://nvidia.github.io/gpu-operator
helm repo update

# Verify repository
helm search repo nvidia/gpu-operator --versions | head -5
```

### Step 2: Install GPU Operator via Helm

```bash
# Create namespace
oc create namespace nvidia-gpu-operator

# Install GPU Operator with OpenShift-specific settings
# This is exactly what prerequisites_installer.go does
helm install gpu-operator nvidia/gpu-operator \
  --namespace nvidia-gpu-operator \
  --version v25.10.1 \
  --set operator.defaultRuntime=crio \
  --set driver.enabled=true \
  --set driver.repository="nvcr.io/nvidia/driver" \
  --set driver.image="driver" \
  --set driver.version="580.105.08" \
  --set driver.imagePullPolicy="IfNotPresent" \
  --set driver.rdma.enabled=false \
  --set driver.manager.env[0].name=DRIVER_TYPE \
  --set driver.manager.env[0].value=precompiled \
  --set toolkit.enabled=true \
  --set devicePlugin.enabled=true \
  --set dcgmExporter.enabled=true \
  --set migManager.enabled=false \
  --set gfd.enabled=true \
  --set cdi.enabled=true \
  --set cdi.default=false \
  --wait \
  --timeout 10m

# IMPORTANT NOTES:
# - operator.defaultRuntime=crio: OpenShift uses CRI-O, not containerd
# - driver.version="580.105.08": Specific driver version tested
# - driver.manager.env[0].value=precompiled: Use precompiled drivers (faster)
# - cdi.enabled=true: REQUIRED for DRA functionality
# - gfd.enabled=true: Enables Node Feature Discovery (auto-labels GPU nodes)
```

### Step 3: Wait for GPU Operator to be Ready

```bash
# Wait for GPU Operator deployment
oc wait --for=condition=Available deployment/gpu-operator \
  -n nvidia-gpu-operator --timeout=300s

# Wait for NVIDIA driver daemonset (CRITICAL - must be 2/2 Running)
oc wait --for=condition=Ready pod \
  -l app=nvidia-driver-daemonset \
  -n nvidia-gpu-operator --timeout=600s

# Wait for container toolkit
oc wait --for=condition=Ready pod \
  -l app=nvidia-container-toolkit-daemonset \
  -n nvidia-gpu-operator --timeout=300s

# Wait for device plugin
oc wait --for=condition=Ready pod \
  -l app=nvidia-device-plugin-daemonset \
  -n nvidia-gpu-operator --timeout=300s

# Verify all pods
oc get pods -n nvidia-gpu-operator

# Expected output:
# NAME                                       READY   STATUS      RESTARTS   AGE
# gpu-feature-discovery-xxxxx                1/1     Running     0          5m
# gpu-operator-xxxxx                         1/1     Running     0          5m
# nvidia-container-toolkit-daemonset-xxxxx   1/1     Running     0          5m
# nvidia-dcgm-exporter-xxxxx                 1/1     Running     0          5m
# nvidia-device-plugin-daemonset-xxxxx       1/1     Running     0          5m
# nvidia-driver-daemonset-xxxxx              2/2     Running     0          5m  ← MUST be 2/2
# nvidia-operator-validator-xxxxx            0/1     Completed   0          5m
```

### Step 4: Verify GPU Node Labeling

```bash
# NFD automatically labels GPU nodes - verify labels
oc get nodes -l nvidia.com/gpu.present=true

# Expected output should show your GPU node(s):
# NAME                                           STATUS   ROLES    AGE   VERSION
# ip-10-0-10-28.ap-south-1.compute.internal     Ready    worker   1h    v1.34.2

# Check GPU node labels in detail
oc describe node <gpu-node-name> | grep nvidia.com

# Expected labels (set by NFD):
# nvidia.com/gpu.present=true
# nvidia.com/gpu.product=Tesla-T4
# nvidia.com/gpu.memory=15360
# nvidia.com/cuda.driver.major=580
# nvidia.com/cuda.driver.minor=105
# nvidia.com/cuda.driver.rev=08
```

### Step 5: Verify nvidia-smi Access

```bash
# Get GPU node name
export GPU_NODE=$(oc get nodes -l nvidia.com/gpu.present=true -o jsonpath='{.items[0].metadata.name}')
echo "GPU Node: ${GPU_NODE}"

# Test nvidia-smi on the node
oc debug node/${GPU_NODE} -- chroot /host /run/nvidia/driver/usr/bin/nvidia-smi

# Expected output:
# +-----------------------------------------------------------------------------------------+
# | NVIDIA-SMI 580.105.08             Driver Version: 580.105.08     CUDA Version: 13.0     |
# +-----------------------------------------+------------------------+----------------------+
# | GPU  Name                 Persistence-M | Bus-Id          Disp.A | Volatile Uncorr. ECC |
# ...
# |   0  Tesla T4                       On  |   00000000:00:1E.0 Off |                    0 |
# ...
```

### Step 6: Install NVIDIA DRA Driver

```bash
# Create namespace for DRA driver
oc create namespace nvidia-dra-driver-gpu

# Grant SCC permissions (REQUIRED before Helm install)
# This is exactly what prerequisites_installer.go does
oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-controller \
  -n nvidia-dra-driver-gpu

oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-kubeletplugin \
  -n nvidia-dra-driver-gpu

oc adm policy add-scc-to-user privileged \
  -z compute-domain-daemon-service-account \
  -n nvidia-dra-driver-gpu

# Install NVIDIA DRA driver via Helm
# ⚠️ CRITICAL: nvidiaDriverRoot MUST be /run/nvidia/driver (NOT /)
helm install nvidia-dra-driver-gpu nvidia/nvidia-dra-driver-gpu \
  --namespace nvidia-dra-driver-gpu \
  --set nvidiaDriverRoot=/run/nvidia/driver \
  --set gpuResourcesEnabledOverride=true \
  --set "featureGates.IMEXDaemonsWithDNSNames=false" \
  --set "featureGates.MPSSupport=true" \
  --set "featureGates.TimeSlicingSettings=true" \
  --set "controller.tolerations[0].key=node-role.kubernetes.io/control-plane" \
  --set "controller.tolerations[0].operator=Exists" \
  --set "controller.tolerations[0].effect=NoSchedule" \
  --set "controller.tolerations[1].key=node-role.kubernetes.io/master" \
  --set "controller.tolerations[1].operator=Exists" \
  --set "controller.tolerations[1].effect=NoSchedule" \
  --wait \
  --timeout 5m

# CRITICAL SETTINGS EXPLAINED:
# - nvidiaDriverRoot=/run/nvidia/driver: Where GPU Operator installs drivers
#   ❌ WRONG: nvidiaDriverRoot=/ (causes kubelet plugin to fail at Init:0/1)
#   ✅ CORRECT: nvidiaDriverRoot=/run/nvidia/driver
#
# - gpuResourcesEnabledOverride=true: Enables GPU resource publishing
# - featureGates.MPSSupport=true: Enables Multi-Process Service support
# - featureGates.TimeSlicingSettings=true: Enables time-slicing for GPU sharing
# - controller.tolerations: Allows controller to run on control plane nodes
```

### Step 7: Verify DRA Driver Installation

```bash
# Check DRA driver pods
oc get pods -n nvidia-dra-driver-gpu

# Expected output:
# NAME                                                READY   STATUS    RESTARTS   AGE
# nvidia-dra-driver-gpu-controller-xxxxx              1/1     Running   0          2m
# nvidia-dra-driver-gpu-kubelet-plugin-xxxxx          2/2     Running   0          2m  ← MUST be 2/2

# Wait for kubelet plugin to be ready
oc wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=nvidia-dra-driver-gpu \
  -n nvidia-dra-driver-gpu --timeout=300s

# Verify ResourceSlices are published
oc get resourceslices

# Expected output (at least 2 slices per GPU node):
# NAME                                                  DRIVER                      POOL            AGE
# ip-10-0-10-28-compute-domain.nvidia.com-xxxxx        compute-domain.nvidia.com   <node-name>     2m
# ip-10-0-10-28-gpu.nvidia.com-xxxxx                   gpu.nvidia.com              <node-name>     2m

# Inspect ResourceSlice details
oc get resourceslice -o json | \
  jq -r '.items[] | select(.spec.driver=="gpu.nvidia.com") | .spec.devices[0]'

# Expected output shows GPU details:
# {
#   "name": "gpu-0",
#   "attributes": {
#     "dra.nvidia.com/architecture": "Turing",
#     "dra.nvidia.com/brand": "Tesla",
#     "dra.nvidia.com/cuda-compute-capability": "7.5",
#     "dra.nvidia.com/index": "0",
#     "dra.nvidia.com/memory": "15360",
#     "dra.nvidia.com/model": "Tesla-T4",
#     "dra.nvidia.com/product": "Tesla-T4-SHARED"
#   }
# }
```

### Step 8: Complete Verification Checklist

```bash
# 1. GPU Operator is running
oc get pods -n nvidia-gpu-operator | grep -v Completed
# All pods should be Running, nvidia-driver-daemonset MUST be 2/2

# 2. DRA Driver is running
oc get pods -n nvidia-dra-driver-gpu
# Expected:
# - nvidia-dra-driver-gpu-controller-* : 1/1 Running
# - nvidia-dra-driver-gpu-kubelet-plugin-* : 2/2 Running

# 3. ResourceSlices published
oc get resourceslices | wc -l
# Should be > 0 (typically 2 per GPU node)

# 4. GPU nodes labeled by NFD
oc get nodes -l nvidia.com/gpu.present=true -o name
# Should list your GPU nodes

# 5. nvidia-smi accessible
GPU_NODE=$(oc get nodes -l nvidia.com/gpu.present=true -o jsonpath='{.items[0].metadata.name}')
oc debug node/${GPU_NODE} -- chroot /host /run/nvidia/driver/usr/bin/nvidia-smi
# Should show GPU information

# ✅ If all checks pass, your cluster is ready for NVIDIA DRA tests!
```

## Critical Configuration Notes

### 1. nvidiaDriverRoot Setting ⚠️

**MOST COMMON ISSUE**: Incorrect `nvidiaDriverRoot` value

```bash
# ❌ WRONG - Causes kubelet plugin to fail (stuck at Init:0/1)
--set nvidiaDriverRoot=/

# ✅ CORRECT - GPU Operator installs drivers here
--set nvidiaDriverRoot=/run/nvidia/driver
```

**How to verify**:
```bash
# Check where driver is actually installed
GPU_NODE=$(oc get nodes -l nvidia.com/gpu.present=true -o jsonpath='{.items[0].metadata.name}')
oc debug node/${GPU_NODE} -- chroot /host ls -la /run/nvidia/driver/usr/bin/nvidia-smi
# Should show the nvidia-smi binary
```

### 2. CDI (Container Device Interface) Requirement

CDI **must be enabled** in GPU Operator for DRA to work:

```bash
# Required in GPU Operator installation
--set cdi.enabled=true
--set cdi.default=false
```

### 3. SCC Permissions for OpenShift

DRA driver service accounts require privileged SCC:

```bash
# Must be done BEFORE installing DRA driver
oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-controller \
  -n nvidia-dra-driver-gpu

oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-kubeletplugin \
  -n nvidia-dra-driver-gpu

oc adm policy add-scc-to-user privileged \
  -z compute-domain-daemon-service-account \
  -n nvidia-dra-driver-gpu
```

### 4. Node Feature Discovery (NFD)

NFD is **included with GPU Operator** and automatically labels GPU nodes:

```bash
# No manual labeling needed - NFD handles this automatically
# Labels added by NFD:
# - nvidia.com/gpu.present=true
# - nvidia.com/gpu.product=Tesla-T4
# - nvidia.com/gpu.memory=15360
# - nvidia.com/cuda.driver.major=580
# - etc.
```

### 5. Driver Type Selection

Use precompiled drivers for faster deployment:

```bash
# Recommended for OpenShift
--set driver.manager.env[0].name=DRIVER_TYPE \
--set driver.manager.env[0].value=precompiled
```

### 6. Feature Gates

Enable MPS and Time-Slicing support:

```bash
--set "featureGates.MPSSupport=true" \
--set "featureGates.TimeSlicingSettings=true"
```

## Cleanup

### Option 1: Automated Cleanup (Recommended)

Use the enhanced cleanup script that mirrors the test code's UninstallAll logic:

```bash
cd test/extended/node/dra/nvidia
./cleanup.sh
```

**What it does:**
1. Uninstalls NVIDIA DRA Driver via Helm (with proper wait/timeout)
2. Removes SCC permissions (ClusterRoleBindings for service accounts)
3. Deletes `nvidia-dra-driver-gpu` namespace
4. Uninstalls GPU Operator via Helm (with proper wait/timeout)
5. Deletes `nvidia-gpu-operator` namespace
6. Cleans up test resources (DeviceClasses, test namespaces)
7. Provides colored output for better visibility

**Features:**
- Matches the UninstallAll logic from prerequisites_installer.go
- Safe error handling (continues even if resources not found)
- Cleans up both Helm releases and namespaces
- Removes test artifacts (DeviceClasses, ResourceClaims in test namespaces)

### Option 2: Manual Cleanup

```bash
# Uninstall DRA Driver
helm uninstall nvidia-dra-driver-gpu -n nvidia-dra-driver-gpu --wait --timeout 5m
oc delete namespace nvidia-dra-driver-gpu

# Uninstall GPU Operator
helm uninstall gpu-operator -n nvidia-gpu-operator --wait --timeout 5m
oc delete namespace nvidia-gpu-operator

# Remove SCC permissions
oc delete clusterrolebinding \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-controller \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-kubeletplugin \
  nvidia-dra-privileged-compute-domain-daemon-service-account
```

**Note**: ResourceSlices are cluster-scoped and will be cleaned up automatically when DRA driver is uninstalled. GPU node labels managed by NFD are also removed automatically.

## CI Integration

### Recommended CI Job Configuration

```bash
#!/bin/bash
set -euo pipefail

# 1. Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# 2. Match origin version to cluster (CRITICAL)
CLUSTER_COMMIT=$(oc adm release info $(oc get clusterversion version -o jsonpath='{.status.desired.image}') \
  --commits | grep "^origin" | awk '{print $NF}')
echo "Cluster origin commit: ${CLUSTER_COMMIT}"

# Checkout matching commit and apply NVIDIA DRA tests
cd /path/to/origin
git checkout ${CLUSTER_COMMIT}
git checkout -b nvidia-dra-ci-${BUILD_ID}

# Apply your NVIDIA DRA test code
# (copy test files, cherry-pick commits, or use other method)

# 3. Build test binary
make WHAT=cmd/openshift-tests

# 4. Run tests (prerequisites installed automatically)
./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]' \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should handle pod deletion and resource cleanup [Suite:openshift/conformance/parallel]' \
  -n '[sig-scheduling] NVIDIA DRA Multi-GPU Workloads should allocate multiple GPUs to single pod [Suite:openshift/conformance/parallel]' \
  -o /logs/test-output.log \
  --junit-dir=/logs/junit

# 5. Exit with test status
exit $?
```

### CI Requirements Checklist

- ✅ OpenShift cluster with GPU worker nodes (g4dn.xlarge or similar)
- ✅ Helm 3 installed in CI environment
- ✅ Cluster-admin kubeconfig available
- ✅ Internet access to pull Helm charts and container images
- ✅ Origin repository checkout matching cluster version
- ⚠️ First test run takes ~15-20 minutes (includes GPU Operator + DRA Driver installation)
- ✅ Subsequent runs are faster (~5-10 minutes, prerequisites skipped if already installed)

### Expected Test Results

```
Test 1: Single GPU Allocation                    ✅ PASSED (6-8 seconds)
Test 2: Pod deletion and resource cleanup        ✅ PASSED (6-8 seconds)
Test 3: Multi-GPU workloads                      ⚠️ SKIPPED (only 1 GPU available)

Total: 2 Passed, 0 Failed, 1 Skipped
```

## Troubleshooting

### Issue 1: "version of origin needs to match the version of the cluster"

**Cause**: Your local origin checkout doesn't match the cluster's release commit.

**Solution**: Follow the "Version Matching Requirement" section above.

### Issue 2: nvidia-driver-daemonset stuck at 1/2 or Init:0/1

**Cause**: Incorrect `nvidiaDriverRoot` setting in DRA driver installation.

**Solution**:
```bash
# Uninstall and reinstall with correct setting
helm uninstall nvidia-dra-driver-gpu -n nvidia-dra-driver-gpu
# Wait for cleanup
sleep 30
# Reinstall with correct nvidiaDriverRoot
helm install nvidia-dra-driver-gpu nvidia/nvidia-dra-driver-gpu \
  --namespace nvidia-dra-driver-gpu \
  --set nvidiaDriverRoot=/run/nvidia/driver \
  ...
```

### Issue 3: ResourceSlices not appearing

**Cause**: DRA driver not fully initialized or SCC permissions missing.

**Solution**:
```bash
# 1. Check DRA driver logs
oc logs -n nvidia-dra-driver-gpu -l app.kubernetes.io/name=nvidia-dra-driver-gpu --all-containers

# 2. Verify SCC permissions
oc describe scc privileged | grep nvidia-dra-driver-gpu

# 3. Restart DRA driver if needed
oc delete pod -n nvidia-dra-driver-gpu -l app.kubernetes.io/name=nvidia-dra-driver-gpu
```

### Issue 4: Tests fail with PodSecurity violations

**Cause**: Namespace not using privileged security level.

**Solution**: The test code already uses `admissionapi.LevelPrivileged` in `nvidia_dra.go`. If you see this error, ensure your test code includes:

```go
oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra", admissionapi.LevelPrivileged)
```

## References

- **NVIDIA GPU Operator**: https://github.com/NVIDIA/gpu-operator
- **NVIDIA DRA Driver**: https://github.com/NVIDIA/k8s-dra-driver-gpu
- **Kubernetes DRA Documentation**: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
- **OpenShift Extended Tests**: https://github.com/openshift/origin/tree/master/test/extended

---

**Last Updated**: 2026-02-04
**Test Framework Version**: openshift-tests v4.1.0-10528-g690b329
**GPU Operator Version**: v25.10.1
**DRA Driver Version**: v25.8.1
**Tested On**: OCP 4.21.0, Kubernetes 1.34.2, Tesla T4
