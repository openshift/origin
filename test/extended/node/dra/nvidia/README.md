# NVIDIA DRA Extended Tests for OpenShift

This directory contains extended tests for NVIDIA Dynamic Resource Allocation (DRA) functionality on OpenShift clusters with GPU nodes.

## Overview

These tests validate:
- NVIDIA DRA driver installation and lifecycle
- Single GPU allocation via ResourceClaims
- Multi-GPU workload allocation
- Pod lifecycle and resource cleanup
- GPU device accessibility in pods

## Enabling DRA on OpenShift - Quick Reference

This section provides a concise guide for enabling Dynamic Resource Allocation (DRA) for NVIDIA GPUs on OpenShift. This information is useful for documentation teams and administrators.

### Prerequisites

1. **OpenShift 4.21+** with Kubernetes 1.34.2+ (DRA GA support)
2. **NVIDIA GPU Operator** installed via OLM (Operator Lifecycle Manager)
   - Install from OperatorHub in OpenShift Console
   - **Critical**: Enable CDI (Container Device Interface) in ClusterPolicy
3. **GPU-enabled worker nodes** (e.g., AWS g4dn.xlarge with Tesla T4)

### Installation Steps Summary

1. **Install GPU Operator** (via OpenShift OperatorHub)
2. **Enable CDI** in GPU Operator ClusterPolicy:
   ```bash
   oc patch clusterpolicy gpu-cluster-policy --type=merge -p '
   spec:
     cdi:
       enabled: true
   '
   ```
3. **Label GPU nodes** for DRA:
   ```bash
   oc label nodes -l nvidia.com/gpu.present=true nvidia.com/dra-kubelet-plugin=true
   ```
4. **Install DRA Driver** with minimal configuration:
   ```bash
   helm install nvidia-dra-driver-gpu nvidia/nvidia-dra-driver-gpu \
     --namespace nvidia-dra-driver-gpu --create-namespace \
     --set nvidiaDriverRoot=/run/nvidia/driver \
     --set gpuResourcesEnabledOverride=true \
     --set image.pullPolicy=IfNotPresent \
     --set-string kubeletPlugin.nodeSelector.nvidia\.com/dra-kubelet-plugin=true \
     --set controller.tolerations[0].key=node-role.kubernetes.io/master \
     --set controller.tolerations[0].operator=Exists \
     --set controller.tolerations[0].effect=NoSchedule \
     --set controller.tolerations[1].key=node-role.kubernetes.io/control-plane \
     --set controller.tolerations[1].operator=Exists \
     --set controller.tolerations[1].effect=NoSchedule
   ```

### Key Configuration Parameters

| Parameter | Value | Why It's Required |
|-----------|-------|-------------------|
| `nvidiaDriverRoot` | `/run/nvidia/driver` | GPU Operator installs drivers here on OpenShift (not `/`) |
| `gpuResourcesEnabledOverride` | `true` | Enables DRA-based GPU allocation (vs. traditional device plugin) |
| `kubeletPlugin.nodeSelector` | `nvidia.com/dra-kubelet-plugin=true` | Ensures DRA components only run on labeled GPU nodes |
| `image.pullPolicy` | `IfNotPresent` | Improves performance by caching images |

### Using DRA in Workloads

Once DRA is enabled, use `ResourceClaim` and `DeviceClass` resources instead of traditional `nvidia.com/gpu` resource requests:

```yaml
# DeviceClass (cluster-scoped)
apiVersion: resource.k8s.io/v1
kind: DeviceClass
metadata:
  name: nvidia-gpu
spec:
  selectors:
  - cel:
      expression: device.driver == "gpu.nvidia.com"
---
# ResourceClaim (namespace-scoped)
apiVersion: resource.k8s.io/v1
kind: ResourceClaim
metadata:
  name: my-gpu-claim
spec:
  devices:
    requests:
    - name: gpu
      exactly:
        deviceClassName: nvidia-gpu
        count: 1
---
# Pod using the claim
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  containers:
  - name: cuda-app
    image: nvcr.io/nvidia/cuda:12.0.0-base-ubuntu22.04
    command: ["nvidia-smi"]
    resources:
      claims:
      - name: gpu
  resourceClaims:
  - name: gpu
    resourceClaimName: my-gpu-claim
```

### Troubleshooting Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| DRA driver pods stuck at `Init:0/1` | Wrong `nvidiaDriverRoot` | Set to `/run/nvidia/driver` (not `/`) |
| CDI device not injected | CDI disabled in GPU Operator | Enable `cdi.enabled=true` in ClusterPolicy |
| Kubelet plugin not scheduled | Nodes not labeled | Label GPU nodes with `nvidia.com/dra-kubelet-plugin=true` |

For complete documentation, see sections below.

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

### Required Cluster Setup (Before Running Tests)

**CRITICAL REQUIREMENTS:**
1. ✅ NVIDIA GPU Operator must be pre-installed (tests will FAIL if not present)
2. ✅ GPU Operator ClusterPolicy must have `cdi.enabled=true` (REQUIRED for DRA)
3. ✅ Cluster must have GPU-enabled worker nodes

### GPU Operator Installation

**Install the NVIDIA GPU Operator following the official documentation:**

📖 **[NVIDIA GPU Operator on OpenShift Installation Guide](https://docs.nvidia.com/datacenter/cloud-native/openshift/latest/install-gpu-ocp.html)**

The official guide covers:
- Installing GPU Operator via OLM (Red Hat OperatorHub)
- Creating and configuring the ClusterPolicy
- Verifying the installation
- Troubleshooting common issues

### DRA-Specific Configuration Requirements

After installing the GPU Operator, ensure the ClusterPolicy has the following settings for DRA to work:

**CRITICAL**: Add or verify these settings in your ClusterPolicy:

```bash
oc patch clusterpolicy gpu-cluster-policy --type=merge -p '
spec:
  operator:
    defaultRuntime: crio
  cdi:
    enabled: true
    default: false
'
```

**Required Settings Explained:**
- `operator.defaultRuntime: crio` - Required for OpenShift (uses CRI-O, not containerd)
- `cdi.enabled: true` - **CRITICAL** - Enables Container Device Interface required for DRA
- `cdi.default: false` - Don't make CDI the default device injection method

### Verification

After installing GPU Operator and configuring the ClusterPolicy:

```bash
# 1. Verify ClusterPolicy is ready
oc get clusterpolicy gpu-cluster-policy -o jsonpath='{.status.state}'
# Expected: "ready"

# 2. Verify CDI is enabled (CRITICAL for DRA)
oc get clusterpolicy gpu-cluster-policy -o jsonpath='{.spec.cdi.enabled}'
# Expected: "true"

# 3. Verify runtime is set to crio
oc get clusterpolicy gpu-cluster-policy -o jsonpath='{.spec.operator.defaultRuntime}'
# Expected: "crio"

# 4. Check GPU Operator pods are running
oc get pods -n nvidia-gpu-operator
# All pods should be in Running state

# 5. Check GPU nodes are labeled
oc get nodes -l nvidia.com/gpu.present=true
# Should list at least one GPU node
```

**If CDI is not enabled**, patch the ClusterPolicy:

```bash
oc patch clusterpolicy gpu-cluster-policy --type=merge -p '
spec:
  cdi:
    enabled: true
    default: false
'

# Wait for container toolkit to restart
oc rollout status daemonset/nvidia-container-toolkit-daemonset -n nvidia-gpu-operator
```

### DRA Driver Versioning

**Important:** Tests install the **latest version** of the NVIDIA DRA Driver from the Helm chart repository. This ensures:
- Early detection of compatibility issues with new DRA driver releases
- Testing against current upstream development
- Validation that latest driver works with cluster's GPU Operator version

If you need a specific DRA driver version, install it manually before running tests, and the test framework will detect and use the existing installation.

### Automatically Installed by Tests

The tests will **automatically install** the following if not already present:
- GPU node labeling with `nvidia.com/dra-kubelet-plugin=true`
- NVIDIA DRA Driver (**latest version** from Helm chart)
- All required SCC permissions for DRA driver
- Helm repository configuration

The test framework detects existing DRA driver installations and skips if already running.

### Required Before Running Tests

1. **OpenShift cluster** with GPU-enabled worker nodes (OCP 4.21+)
   - DRA support requires OpenShift 4.21 or later
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

## TL;DR - Quick Command Reference

For users who already have their cluster set up with GPU Operator and want to run tests immediately:

```bash
# Build test binary (ensure you've matched your origin checkout to cluster version)
cd /path/to/origin
make WHAT=cmd/openshift-tests

# Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# Run ALL NVIDIA DRA tests in one command
./openshift-tests run --dry-run all 2>&1 | \
  grep "NVIDIA DRA" | \
  ./openshift-tests run -f -

# Alternative: Run specific test
./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]'
```

**Prerequisites**: GPU Operator must be installed with CDI enabled. See full documentation below for details.

---

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

# 5. Run all NVIDIA DRA tests (single command)
./openshift-tests run --dry-run all 2>&1 | \
  grep "NVIDIA DRA" | \
  ./openshift-tests run -f -

# OR run tests individually:
./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]'

./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should handle pod deletion and resource cleanup [Suite:openshift/conformance/parallel]'

./openshift-tests run-test \
  -n '[sig-scheduling] NVIDIA DRA Multi-GPU Workloads should allocate multiple GPUs to single pod [Suite:openshift/conformance/parallel]'
```

**What happens automatically:**
1. Tests verify GPU Operator is already installed (FAILS if not present)
2. Tests wait for GPU Operator to be ready
3. Tests check if DRA Driver is already installed
4. If DRA Driver not found:
   - GPU nodes are labeled with `nvidia.com/dra-kubelet-plugin=true`
   - Helm repository is added (`nvidia` repo)
   - DRA Driver (latest version) is installed via Helm with minimal configuration:
     - `nvidiaDriverRoot=/run/nvidia/driver` - Points to GPU Operator driver location
     - `gpuResourcesEnabledOverride=true` - Enables GPU allocation via DRA
     - `image.pullPolicy=IfNotPresent` - Caches images for faster startup
     - `kubeletPlugin.nodeSelector` - Targets labeled GPU nodes only
     - `controller.tolerations` - Allows controller to schedule on tainted control-plane nodes
   - SCC permissions are granted to DRA service accounts
5. Tests wait for DRA Driver to be ready (controller + kubelet plugin)
6. Tests execute against the configured GPU stack

**Important:** GPU Operator MUST be pre-installed. See Prerequisites section above.

**Re-running tests:** DRA Driver installation is automatically skipped if already installed (detection works with both Helm and manual installations).

### Option 2: Run with Regex Filter

Run all NVIDIA DRA tests that match a pattern:

```bash
# Run all tests containing "NVIDIA DRA" using regex
./openshift-tests run all --run-until-failure -o /tmp/nvidia-dra-results \
  --include-success --junit-dir /tmp/nvidia-dra-junit 2>&1 | \
  grep -E '\[sig-scheduling\] NVIDIA DRA'
```

**Note**: The command above runs all matching tests but filters output. For cleaner execution, use the method in Option 1.

### Option 3: List Available Tests

```bash
# List all NVIDIA DRA tests without running them
./openshift-tests run --dry-run all 2>&1 | grep "NVIDIA DRA"

# Example output:
# "[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]"
# "[sig-scheduling] NVIDIA DRA Basic GPU Allocation should handle pod deletion and resource cleanup [Suite:openshift/conformance/parallel]"
# "[sig-scheduling] NVIDIA DRA Multi-GPU Workloads should allocate multiple GPUs to single pod [Suite:openshift/conformance/parallel]"

# Count total NVIDIA DRA tests
./openshift-tests run --dry-run all 2>&1 | grep -c "NVIDIA DRA"
```

### Option 4: Run Standalone Validation (No Framework)

For quick manual validation without the test framework:

```bash
cd test/extended/node/dra/nvidia
export KUBECONFIG=/path/to/kubeconfig
./standalone_test.sh
```

**Features**: The standalone script now includes:
- **Automated DRA Driver installation** (via Helm if not already present)
- Detection of existing installations (via running pods, not just Helm releases)
- GPU Operator validation (fails if not present)
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

### Step 2: Verify GPU Operator Installation

The GPU Operator should already be installed on the cluster. Verify it's running:

```bash
# Check namespace exists
oc get namespace nvidia-gpu-operator

# Check GPU Operator pods
oc get pods -n nvidia-gpu-operator

# Expected output should include:
# - gpu-operator-xxxxx (Running)
# - nvidia-driver-daemonset-xxxxx (Running, 2/2)
# - nvidia-device-plugin-daemonset-xxxxx (Running)
# - nvidia-dcgm-exporter-xxxxx (Running)
# - gpu-feature-discovery-xxxxx (Running)

# Verify GPU nodes are labeled by NFD
oc get nodes -l nvidia.com/gpu.present=true

# Expected: At least one GPU node listed
```

If GPU Operator is not installed, install it following the Prerequisites section above.

### Step 3: Verify GPU Node Labeling

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

### Step 4: Verify nvidia-smi Access

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

### Step 5: Label GPU Nodes for DRA

Before installing the DRA driver, label all GPU nodes to indicate they should run the DRA kubelet plugin:

```bash
# Label all GPU nodes for DRA kubelet plugin scheduling
for node in $(oc get nodes -l nvidia.com/gpu.present=true -o name); do
  oc label $node nvidia.com/dra-kubelet-plugin=true --overwrite
done

# Verify the label was applied
oc get nodes -l nvidia.com/dra-kubelet-plugin=true

# Expected output: All GPU nodes should be listed
```

**Why is this label required?**

The `nvidia.com/dra-kubelet-plugin=true` label serves two purposes:

1. **Node Selection**: Ensures the DRA kubelet plugin DaemonSet only runs on GPU-enabled nodes
2. **Driver Manager Compatibility**: Works around a known issue where the NVIDIA Driver Manager doesn't properly evict DRA kubelet plugin pods during driver updates

This label is recommended by NVIDIA's official documentation and is used in the kubelet plugin's node selector configuration.

### Step 6: Install NVIDIA DRA Driver

```bash
# Create namespace for DRA driver
oc create namespace nvidia-dra-driver-gpu

# Grant SCC permissions (REQUIRED before Helm install on OpenShift)
oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-controller \
  -n nvidia-dra-driver-gpu

oc adm policy add-scc-to-user privileged \
  -z nvidia-dra-driver-gpu-service-account-kubeletplugin \
  -n nvidia-dra-driver-gpu

oc adm policy add-scc-to-user privileged \
  -z compute-domain-daemon-service-account \
  -n nvidia-dra-driver-gpu

# Install NVIDIA DRA driver via Helm with minimal configuration
helm install nvidia-dra-driver-gpu nvidia/nvidia-dra-driver-gpu \
  --namespace nvidia-dra-driver-gpu \
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
  --wait \
  --timeout 5m
```

### DRA Driver Helm Parameters Explained

The following table describes each Helm parameter used in the installation:

| Parameter | Value | Required | Purpose |
|-----------|-------|----------|---------|
| `nvidiaDriverRoot` | `/run/nvidia/driver` | **Yes** | Specifies where the NVIDIA GPU Operator installs GPU drivers on OpenShift nodes. The DRA driver needs this path to access the NVIDIA driver binaries and libraries.<br><br>**Critical**: Must be `/run/nvidia/driver` for GPU Operator installations. Using `/` (the default) will cause the kubelet plugin to fail with `Init:0/1` errors. |
| `gpuResourcesEnabledOverride` | `true` | **Yes** | Enables GPU allocation support in the DRA driver. This tells the driver to publish GPU resources to Kubernetes and handle GPU allocation requests via DRA.<br><br>Without this, only ComputeDomain resources would be available (for multi-node GPU configurations). |
| `image.pullPolicy` | `IfNotPresent` | Recommended | Caches container images locally after first pull. Improves pod startup time on subsequent deployments.<br><br>Recommended by NVIDIA documentation for production deployments. |
| `kubeletPlugin.nodeSelector.nvidia.com/dra-kubelet-plugin` | `true` | Recommended | Restricts the DRA kubelet plugin DaemonSet to only run on nodes labeled with `nvidia.com/dra-kubelet-plugin=true`.<br><br>**Benefits**:<br>- Prevents kubelet plugin from attempting to run on non-GPU nodes<br>- Works around NVIDIA Driver Manager pod eviction issues<br>- Follows NVIDIA's recommended deployment pattern<br><br>**Important**: Use `--set-string` (not `--set`) to ensure the value `true` is treated as a string. Kubernetes `nodeSelector` requires string values, and using `--set` will cause Helm to interpret `true` as a boolean, resulting in installation errors. |
| `controller.tolerations[*]` | Specific tolerations | **Required** | Allows the controller pod to schedule on control-plane/master nodes that have `NoSchedule` taints.<br><br>**Why needed**: The controller (a Deployment) has a node affinity requiring it to run on control-plane nodes. These nodes are typically tainted with `node-role.kubernetes.io/master:NoSchedule` or `node-role.kubernetes.io/control-plane:NoSchedule` to prevent regular workloads from scheduling there. Without these tolerations, the controller pod will remain in `Pending` state.<br><br>**Tolerations set**:<br>- `[0]`: Tolerates `node-role.kubernetes.io/master:NoSchedule`<br>- `[1]`: Tolerates `node-role.kubernetes.io/control-plane:NoSchedule`<br><br>These cover both legacy (`master`) and current (`control-plane`) node role naming conventions. |

### Optional Feature Gates (Not Set by Default)

The DRA driver supports several feature gates that can be enabled for specific use cases. **These are intentionally NOT set in the basic installation** to keep configuration minimal:

| Feature Gate | Default | When to Enable |
|-------------|---------|----------------|
| `featureGates.MPSSupport` | Platform default | Enable when testing NVIDIA Multi-Process Service (MPS) for GPU sharing |
| `featureGates.TimeSlicingSettings` | Platform default | Enable when testing GPU time-slicing for workload scheduling |
| `featureGates.ComputeDomainCliques` | Platform default | Enable for multi-node GPU configurations with NVLink (GB200/GB300 systems) |
| `featureGates.IMEXDaemonsWithDNSNames` | Platform default | Required if `ComputeDomainCliques` is enabled |

**Best Practice**: Only enable feature gates when you need to test or use those specific features. This keeps the configuration simple and avoids potential conflicts.

### Common Configuration Mistakes

❌ **Wrong**: Using default driver root
```bash
--set nvidiaDriverRoot=/
```
**Result**: Kubelet plugin pods stuck at `Init:0/1` because they can't find GPU drivers

✅ **Correct**: Specify GPU Operator driver location
```bash
--set nvidiaDriverRoot=/run/nvidia/driver
```

---

❌ **Wrong**: Not labeling GPU nodes
```bash
# Skipping node labeling step
helm install nvidia-dra-driver-gpu ...
```
**Result**: Kubelet plugin may attempt to run on non-GPU nodes, or driver manager issues occur

✅ **Correct**: Label nodes before installation
```bash
oc label node <gpu-node> nvidia.com/dra-kubelet-plugin=true
helm install nvidia-dra-driver-gpu ... \
  --set-string kubeletPlugin.nodeSelector.nvidia\.com/dra-kubelet-plugin=true
```

---

❌ **Wrong**: Enabling feature gates unnecessarily
```bash
--set featureGates.MPSSupport=true \
--set featureGates.TimeSlicingSettings=true \
--set featureGates.ComputeDomainCliques=false
```
**Result**: Adds complexity without benefit for basic GPU allocation testing

✅ **Correct**: Minimal configuration for basic DRA
```bash
--set nvidiaDriverRoot=/run/nvidia/driver \
--set gpuResourcesEnabledOverride=true
```
Enable feature gates only when testing specific features

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

## Important Notes

### DRA Driver Configuration

The NVIDIA DRA Driver (installed automatically by tests) requires the correct `nvidiaDriverRoot` setting:

```bash
# ✅ CORRECT - Points to where GPU Operator installs drivers
--set nvidiaDriverRoot=/run/nvidia/driver
```

This is automatically configured by the test framework. If you're manually installing the DRA driver, ensure this setting is correct.

### GPU Operator CDI Requirement

**CRITICAL**: The GPU Operator ClusterPolicy must have CDI enabled for DRA to work.

Verify CDI is enabled:
```bash
oc get clusterpolicy gpu-cluster-policy -o jsonpath='{.spec.cdi.enabled}'
# Expected: "true"
```

If not enabled, see the Prerequisites section above for instructions to patch the ClusterPolicy.

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
4. Cleans up test resources (DeviceClasses, test namespaces)
5. Provides colored output for better visibility

**Note:** GPU Operator is cluster infrastructure and is NOT removed by cleanup.

**Features:**
- Matches the UninstallAll logic from prerequisites_installer.go
- Safe error handling (continues even if resources not found)
- Cleans up DRA driver Helm release and namespace
- Removes test artifacts (DeviceClasses, ResourceClaims in test namespaces)

### Option 2: Manual Cleanup

```bash
# Uninstall DRA Driver
helm uninstall nvidia-dra-driver-gpu -n nvidia-dra-driver-gpu --wait --timeout 5m
oc delete namespace nvidia-dra-driver-gpu

# Remove SCC permissions
oc delete clusterrolebinding \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-controller \
  nvidia-dra-privileged-nvidia-dra-driver-gpu-service-account-kubeletplugin \
  nvidia-dra-privileged-compute-domain-daemon-service-account
```

**Note:** GPU Operator is cluster infrastructure and is NOT removed by cleanup. ResourceSlices are cluster-scoped and will be cleaned up automatically when DRA driver is uninstalled.

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

# 4. Run all NVIDIA DRA tests (single command - recommended for CI)
./openshift-tests run --dry-run all 2>&1 | \
  grep "NVIDIA DRA" | \
  ./openshift-tests run -f - \
  -o /logs/test-output.log \
  --junit-dir=/logs/junit

# Alternative: Run tests individually with explicit names
# ./openshift-tests run-test \
#   -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should allocate single GPU to pod via DRA [Suite:openshift/conformance/parallel]' \
#   -n '[sig-scheduling] NVIDIA DRA Basic GPU Allocation should handle pod deletion and resource cleanup [Suite:openshift/conformance/parallel]' \
#   -n '[sig-scheduling] NVIDIA DRA Multi-GPU Workloads should allocate multiple GPUs to single pod [Suite:openshift/conformance/parallel]' \
#   -o /logs/test-output.log \
#   --junit-dir=/logs/junit

# 5. Exit with test status
exit $?
```

### CI Requirements Checklist

- ✅ OpenShift cluster with GPU worker nodes (g4dn.xlarge or similar)
- ✅ GPU Operator pre-installed with CDI enabled (see Prerequisites section)
- ✅ Helm 3 installed in CI environment
- ✅ Cluster-admin kubeconfig available
- ✅ Internet access to pull Helm charts and container images
- ✅ Origin repository checkout matching cluster version
- ⚠️ First test run takes ~5-10 minutes (includes DRA Driver installation)
- ✅ Subsequent runs are faster (~2-5 minutes, DRA driver skipped if already installed)

### Expected Test Results

```
Test 1: Single GPU Allocation                    ✅ PASSED (6-8 seconds)
Test 2: Pod deletion and resource cleanup        ✅ PASSED (6-8 seconds)
Test 3: Multi-GPU workloads                      ⚠️ SKIPPED (only 1 GPU available)

Total: 2 Passed, 0 Failed, 1 Skipped
```

## Troubleshooting

### Issue 1: Tests fail with "GPU Operator not found"

**Cause**: GPU Operator is not installed on the cluster.

**Solution**: Install GPU Operator following the [NVIDIA GPU Operator on OpenShift Installation Guide](https://docs.nvidia.com/datacenter/cloud-native/openshift/latest/install-gpu-ocp.html), then ensure CDI is enabled in the ClusterPolicy (see Prerequisites section).

### Issue 2: Tests fail with CDI or device injection errors

**Cause**: CDI is not enabled in the GPU Operator ClusterPolicy.

**Solution**:
```bash
# Check if CDI is enabled
oc get clusterpolicy gpu-cluster-policy -o jsonpath='{.spec.cdi.enabled}'

# If not "true", patch the ClusterPolicy
oc patch clusterpolicy gpu-cluster-policy --type=merge -p '
spec:
  cdi:
    enabled: true
    default: false
'

# Wait for container toolkit to restart
oc rollout status daemonset/nvidia-container-toolkit-daemonset -n nvidia-gpu-operator
```

### Issue 3: "version of origin needs to match the version of the cluster"

**Cause**: Your local origin checkout doesn't match the cluster's release commit.

**Solution**: Follow the "Version Matching Requirement" section above.

### Issue 5: DRA driver kubelet plugin stuck at Init:0/1

**Cause**: DRA driver cannot find GPU drivers (usually already handled by test framework).

**Solution**: This is automatically configured correctly by the test framework. If manually installing DRA driver, ensure `nvidiaDriverRoot=/run/nvidia/driver`.

### Issue 6: ResourceSlices not appearing

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

### Issue 7: Tests fail with PodSecurity violations

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

**Last Updated**: 2026-02-13
**Test Framework Version**: openshift-tests v4.1.0-10528-g690b329
**GPU Operator**: Pre-installed (see Prerequisites)
**DRA Driver**: Latest version (auto-installed by tests)
**Tested On**: OCP 4.21.0, Kubernetes 1.34.2, Tesla T4
