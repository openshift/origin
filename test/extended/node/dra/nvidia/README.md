# NVIDIA DRA Extended Tests for OpenShift

This directory contains extended tests for NVIDIA Dynamic Resource Allocation (DRA) functionality on OpenShift clusters with GPU nodes.

## Overview

These tests validate:
- DRA driver readiness and GPU ResourceSlice publication
- Single- and multi-GPU allocation through ResourceClaims
- Claim sharing, ResourceClaimTemplate use, and cleanup after pod deletion
- CDI injection through CRI-O, including init containers and CDI spec cleanup
- GPU access under SELinux and restricted SCC, including non-root workloads
- Request-level CEL selectors and kubelet restart recovery

Upstream DRA tests cover generic allocation behavior; these tests verify NVIDIA driver integration on OpenShift, including CDI injection and GPU access under OpenShift security controls.

The `openshift/nvidia-dra` suite runs serially on GPU hardware. `crio_cdi.go` checks
the DRA driver's CDI environment edit, claim-specific CDI spec creation and
cleanup, multi-GPU access, and init containers. `security.go` checks GPU access
from confined workloads admitted under `restricted-v2`, including arbitrary
non-root UIDs. `claim_lifecycle.go` compares the GPU identity seen by two
containers sharing one claim with the allocated ResourceSlice device. The
common cleanup waits for claim release before the next serial test.

## Prerequisites

1. **OpenShift 4.21+** with NVIDIA GPU nodes
2. **Node Feature Discovery Operator** pre-installed
3. **NVIDIA GPU Operator** pre-installed in `nvidia-gpu-operator` with **CDI enabled**
   (`cdi.enabled=true`)
4. **Helm 3** installed and available in PATH if the suite needs to install the DRA driver
5. **Cluster-admin** access
6. Access to `nvcr.io/nvidia/cuda:12.0.0-base-ubuntu22.04`, used by the GPU workload pods

The GPU Operator's driver and CDI toolkit must be ready, and NFD must expose GPU nodes with the
`nvidia.com/gpu.present=true` label. The DRA tests do not require the GPU Operator device plugin;
the suite uses the NVIDIA DRA driver to allocate GPUs through ResourceClaims.

For installation instructions, see the [NVIDIA GPU Operator on OpenShift Installation Guide](https://docs.nvidia.com/datacenter/cloud-native/openshift/latest/install-gpu-ocp.html).

**Note**: The test framework installs the NVIDIA DRA driver if it is not already running. It uses
chart version `25.12.0` by default and leaves the driver installed after the suite. Installation
requires Helm access to NVIDIA's chart repository. The suite is marked `[Skipped:Disconnected]`
because the workload image and, when needed, the driver chart come from NVIDIA's registries.

## Quick Start

### Running Tests

```bash
# 1. Build test binary
# Note: Your origin checkout should match the cluster version to avoid version mismatch errors
make WHAT=cmd/openshift-tests

# 2. Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# 3. Run all NVIDIA DRA tests
OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests run openshift/nvidia-dra

# OR run a specific test
OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests run-test \
  '[sig-scheduling][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial][Skipped:Disconnected] Basic GPU Allocation should allocate single GPU to pod via DRA'

# OR list all available tests without running them
OPENSHIFT_SKIP_EXTERNAL_TESTS=1 \
  ./openshift-tests run --dry-run all 2>&1 | grep "\[Feature:NVIDIA-DRA\]"
```

Set `OPENSHIFT_SKIP_EXTERNAL_TESTS=1` when running a locally built binary so the runner does not
extract extension tests from the cluster release payload. `run-test` runs one spec without the
cluster monitors; `run openshift/nvidia-dra` runs the suite with monitors.

**What the tests do automatically:**
- Verify the GPU Operator is installed and the required GPU labels are present
- If no running DRA driver pod is detected, install the driver (version 25.12.0 by default), label
  GPU nodes, grant the driver service accounts privileged SCC, and wait for controller, plugin,
  and GPU ResourceSlice readiness

To use a different chart version, set `NVIDIA_DRA_DRIVER_VERSION` to that chart's version before
running the suite.

## Test Scenarios

The suite currently covers these scenarios:

- `nvidia_dra.go`: single and multiple GPU allocation, allocation release after pod
  deletion, two pods referencing one claim, and claims created from a
  ResourceClaimTemplate.
- `crio_cdi.go`: the DRA driver's `NVIDIA_VISIBLE_DEVICES=void` environment edit,
  claim-specific CDI spec creation and removal, multiple CDI devices, and
  init-container access.
- `security.go`: GPU access in the confined `container_t` SELinux domain with no
  matching AVC denial, GPU access under `restricted-v2`, and access from a
  namespace-assigned non-root UID.
- `claim_lifecycle.go`: physical GPU UUID identity for two containers sharing a
  claim, plus matching and non-matching request-level CEL selectors against
  NVIDIA device capacity.
- `kubelet_restart.go`: workload and claim recovery across kubelet restart, and
  preparation of a new claim after the plugin re-registers. These specs restart
  kubelet and are marked disruptive.

Specs run serially. Cleanup deletes workloads and waits for their ResourceClaims to release before
the next spec starts. Multi-GPU specs skip when the required number of GPUs is unavailable.

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
  oc label "$node" nvidia.com/dra-kubelet-plugin=true --overwrite
done

# Verify labels
oc get nodes -l nvidia.com/dra-kubelet-plugin=true
```

The DRA chart uses this label as the kubelet-plugin node selector, so the plugin runs only on GPU
nodes.

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
# Check DRA controller and kubelet plugin readiness
oc get deployment nvidia-dra-driver-gpu-controller -n nvidia-dra-driver
oc get daemonset nvidia-dra-driver-gpu-kubelet-plugin -n nvidia-dra-driver

# Verify ResourceSlices are published
oc get resourceslices -o wide
# Look for gpu.nvidia.com and compute-domain.nvidia.com slices on the GPU nodes.
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

# Remove the DRA scheduling label added during installation
for node in $(oc get nodes -l nvidia.com/dra-kubelet-plugin=true -o name); do
  oc label "$node" nvidia.com/dra-kubelet-plugin-
done
```

**Note**: The GPU Operator is not removed because it is cluster infrastructure. ResourceSlices are
removed when the DRA driver is uninstalled.

## Troubleshooting

### GPU Operator not found

**Cause**: GPU Operator is not installed in the `nvidia-gpu-operator` namespace or its operator pod
is not running.

**Solution**: Install GPU Operator following the [official guide](https://docs.nvidia.com/datacenter/cloud-native/openshift/latest/install-gpu-ocp.html).

### Version mismatch error

**Cause**: Local origin checkout doesn't match cluster's release commit.

**Solution**: Ensure your origin checkout matches the cluster version, or use `./openshift-tests run-test` command which bypasses version checks.

### DRA driver components are not ready

**Cause**: The DRA chart may use the wrong `nvidiaDriverRoot`, or the GPU Operator driver/CDI toolkit
is not ready.

**Solution**: Ensure `nvidiaDriverRoot=/run/nvidia/driver` (not `/`). The tests set this value when
installing the chart. Check the controller and kubelet plugin:

```bash
oc get deployment nvidia-dra-driver-gpu-controller -n nvidia-dra-driver
oc get daemonset nvidia-dra-driver-gpu-kubelet-plugin -n nvidia-dra-driver
```

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
