# DRA Example Driver Extended Tests for OpenShift

This directory contains extended tests for the upstream [dra-example-driver](https://github.com/kubernetes-sigs/dra-example-driver) on OpenShift clusters. These tests provide **hardware-independent** DRA regression coverage — no GPU or special hardware is required.

## Overview

These tests validate:
- DRA example driver installation and lifecycle
- Single device allocation via ResourceClaims
- Multi-device allocation
- Pod lifecycle and resource cleanup
- Claim sharing behavior
- ResourceClaimTemplate-based claim creation and cleanup

## Prerequisites

1. **OpenShift 4.21+** cluster (DRA API enabled by default)
2. **Helm 3** installed and available in PATH
3. **git** installed and available in PATH
4. **Cluster-admin** access

The test framework automatically:
- Clones the upstream `dra-example-driver` repository
- Installs the driver via Helm with OpenShift SCC permissions
- Waits for driver components to be ready

## Quick Start

```bash
# 1. Build test binary
make WHAT=cmd/openshift-tests

# 2. Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# 3. Run all DRA example driver tests (local binary)
OPENSHIFT_SKIP_EXTERNAL_TESTS=1 \
  ./openshift-tests run --dry-run all 2>&1 | \
  grep "\[Feature:DRA-Example\]" | \
  OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests run -f -

# OR run a specific test
OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests run-test \
  '[sig-scheduling][Feature:DRA-Example][Suite:openshift/dra-example][Serial] Basic Device Allocation should allocate single device to pod via DRA'

# OR list all available tests
OPENSHIFT_SKIP_EXTERNAL_TESTS=1 \
  ./openshift-tests run --dry-run all 2>&1 | grep "\[Feature:DRA-Example\]"
```

> **Note**: `OPENSHIFT_SKIP_EXTERNAL_TESTS=1` is required when running a locally
> built binary. Without it, the `run` command attempts to extract test binaries
> from the cluster's release payload, which does not contain your local changes.
> This variable is NOT needed in CI where the binary is part of the payload.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DRA_EXAMPLE_DRIVER_REF` | `main` | Git ref (branch/tag) of the upstream dra-example-driver to install |

## Test Scenarios

### 1. Single Device Allocation
- Creates DeviceClass with CEL selector for `gpu.example.com` driver
- Creates ResourceClaim requesting 1 device
- Schedules pod with ResourceClaim
- Validates device allocation in ResourceClaim status

### 2. Resource Cleanup
- Creates pod with device ResourceClaim
- Deletes pod
- Verifies ResourceClaim persists after pod deletion but is unreserved

### 3. Multi-Device Allocation
- Creates ResourceClaim requesting 2 devices
- Schedules pod requiring multiple devices
- Validates all devices are allocated (driver publishes 9 virtual devices per node)

### 4. Claim Sharing
- Creates a single ResourceClaim
- Creates two pods referencing the same ResourceClaim
- Verifies behavior: both pods run (sharing supported) or second pod stays Pending

### 5. ResourceClaimTemplate
- Creates a ResourceClaimTemplate
- Creates pod with ResourceClaimTemplate reference
- Validates that ResourceClaim is auto-created from template
- Validates automatic cleanup of template-generated claim when pod is deleted

## OpenShift-Specific Adaptations

The upstream `dra-example-driver` Helm chart requires the following OpenShift adaptations (handled automatically by the test framework):

1. **SCC Grant**: The kubelet plugin DaemonSet runs with `privileged: true` and mounts hostPath volumes. A ClusterRoleBinding grants the `system:openshift:scc:privileged` ClusterRole to the driver ServiceAccount.

2. **SNO Tolerations**: Control-plane tolerations are added to allow scheduling on single-node OpenShift clusters.

## Troubleshooting

### Helm not found

**Cause**: Helm 3 not installed.

**Solution**: Install Helm following [official instructions](https://helm.sh/docs/intro/install/).

### SCC denied — kubelet plugin pod rejected

**Cause**: ClusterRoleBinding for privileged SCC not created.

**Solution**: The test framework creates this automatically. For manual debugging:

```bash
oc adm policy add-scc-to-user privileged \
  -n dra-example-driver \
  -z dra-example-driver-service-account
```

### ResourceSlices not appearing

**Cause**: DRA driver DaemonSet not ready.

**Solution**:

```bash
# Check DRA driver pods
oc get pods -n dra-example-driver

# Check DaemonSet logs
oc logs -n dra-example-driver -l app.kubernetes.io/name=dra-example-driver --all-containers
```

## References

- **Upstream repository**: https://github.com/kubernetes-sigs/dra-example-driver
- **Kubernetes DRA docs**: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
- **OpenShift Extended Tests**: https://github.com/openshift/origin/tree/master/test/extended
- **NVIDIA DRA tests (reference)**: `test/extended/node/dra/nvidia/`
