# Node E2E Tests

This directory contains OpenShift end-to-end tests for node-related features.

## Test Suites

### Suite: openshift/disruptive-longrunning

- **kubeletconfig_features.go** - Tests applying KubeletConfig to custom machine config pools, requires node reboots
- **kubelet_secret_pulled_images.go** - Tests kubelet credential verification for image pulls (`KubeletEnsureSecretPulledImages` feature gate). Covers multi-tenancy isolation, credential rotation, ImagePullPolicy behavior, credential verification policy (NeverVerify/AlwaysVerify), and registry availability scenarios. Requires `TechPreviewNoUpgrade` or `CustomNoUpgrade` FeatureSet.
- **node_e2e/container_runtime_config.go** - ContainerRuntimeConfig pidsLimit (OCP-45351) and overlaySize (OCP-46313) - Verifies CTRCFG settings are applied via MCO rollout and reflected on nodes \[Disruptive\]
- **node_e2e/image_registry_config.go** - Container registry config change (OCP-44820) - Verifies search registry update triggers MCO rollout and lands on nodes \[Disruptive\]
- **node_e2e/netns_cleanup.go** - Network namespace cleanup - Verifies kubelet/CRI-O properly deletes network namespace when a pod is deleted \[OTP\]
- **node_e2e/pdb_drain.go** - PodDisruptionBudget drain blocking (OCP-67564) - Tests that node drain is blocked when PDB has minAvailable=100% with empty selector \[Disruptive\] \[Lifecycle:informing\]

- **Additional Storage Support** - Tests for additionalArtifactStores, additionalImageStores, and additionalLayerStores CRI-O configuration
  
  **Availability:**
  - **OCP 4.22:** TechPreview (requires TechPreviewNoUpgrade feature gate)
  - **OCP 4.23+/5.0+:** GA (Generally Available)
  
  **Suite:** `openshift/disruptive-longrunning` (use TechPreview variant CI job for 4.22)  
  **Feature Tag:** `[Feature:AdditionalStorageSupport]`  
  **Sig Tag:** `[sig-node]`
  
  **Test Files:**
  - **additional_artifact_stores.go** - API validation and E2E tests for artifact stores (max 10 stores)
  - **additional_image_stores.go** - API validation and E2E tests for image stores (max 10 stores), including prepopulated image performance
  - **additional_layer_stores.go** - API validation and E2E tests for layer stores (max 5 stores), including stargz lazy pulling with :ref suffix
  - **additional_stores_combined.go** - API validation and E2E tests for combined storage configurations
  - **stargz_store_setup.go** - Helper for deploying/cleaning up stargz-store daemonset on worker nodes
  
  **Requirements:**
  - TechPreviewNoUpgrade feature gate must be enabled (only for OCP 4.22)
  - E2E tests are Serial and Disruptive (trigger MachineConfigPool updates and node reboots)
  - Tests that pull external images are tagged [Skipped:Disconnected] for air-gapped environments
  - API validation tests are non-disruptive and can run in parallel
  
  **Test Environments:**
  
  These tests run in CI on multiple platforms via `disruptive-longrunning-techpreview` jobs:
  - AWS (primary platform for Additional Storage Support E2E tests)
  - Azure (E2E tests explicitly skip Azure via platform detection)
  - GCP
  - vSphere
  - Metal IPI (IPv6 and dual-stack)
  
  **CI Job Configuration:**
  - **Feature Set:** `TechPreviewNoUpgrade` (enables tech preview features)
  - **Test Suite:** `openshift/disruptive-longrunning`
  - **Interval:** Weekly (168h)
  - **Sharding:** 2 shards to parallelize execution
  - **Job definitions:** See `ci-operator/config/openshift/release/openshift-release-main__nightly-4.XX.yaml` in [openshift/release](https://github.com/openshift/release) repo
  
  **Running these tests:**
  ```bash
  # Run all Additional Storage Support tests
  ./openshift-tests run "openshift/disruptive-longrunning" --dry-run | grep "\[Feature:AdditionalStorageSupport\]" | ./openshift-tests run -f - --cluster-stability=Disruptive
  
  # Run specific test file (e.g., artifact stores)
  ./openshift-tests run "openshift/disruptive-longrunning" --dry-run | grep "Additional Artifact Stores" | ./openshift-tests run -f - --cluster-stability=Disruptive
  ```
  
  **Test Coverage:**
  - Path format validation (absolute paths, character restrictions, length limits)
  - Count limits enforcement (10 for artifact/image stores, 5 for layer stores)
  - Duplicate path detection within store types
  - CRI-O storage.conf generation and verification
  - MachineConfigOperator (MCO) configuration updates and rollouts
  - Lazy pulling with eStargz images and stargz-store snapshotter
  - Prepopulated image store performance validation
  - Fallback behavior when lazy pulling prerequisites are not met

### Suite: openshift/usernamespace

- **nested_container.go** - Tests running nested containers (podman-in-pod) with user namespaces and nested-container SCC

### Default Suite

- **dra.go** - Tests that DRA (Dynamic Resource Allocation) v1 API is available and beta/alpha APIs are disabled
- **image_volume.go** - Tests mounting container images as volumes in pods, including subPath and error handling
- **node_swap.go** - Tests default kubelet swap settings (failSwapOn and swapBehavior) and rejection of user overrides
- **zstd_chunked.go** - Tests building and running images with zstd:chunked compression format
- **node_e2e/probe_termination.go** - Probe-level terminationGracePeriodSeconds (OCP-44493) - Tests configurable termination grace period for liveness and startup probes. Includes 3 test cases: probe-level config for liveness probe, probe-level config for startup probe, and fallback to pod-level config when probe-level is not set [Lifecycle:informing]

## Directory Structure

### Test Files
- All `*.go` files in the root directory are Ginkgo-based test suites
- Each file focuses on a specific node feature

### Utility Files
- **node_utils.go** - Shared helper functions for node selection and kubelet configuration retrieval

### Test Data
Test fixtures are referenced via `exutil.FixturePath` from:
- `testdata/node/machineconfigpool/` - Machine config pool fixtures
- `testdata/node/kubeletconfig/` - Kubelet config fixtures
- `testdata/node/zstd-chunked/`, `testdata/node/nested_container/` - Custom build fixtures

## Running Tests

### Running Long-Running Disruptive Tests

The `openshift/disruptive-longrunning` suite is a general-purpose suite for long-running disruptive tests
across all teams. Node team tests are tagged with `[sig-node]` to identify them.

To run the entire long-running disruptive test suite on a cluster manually:

```bash
./openshift-tests run "openshift/disruptive-longrunning" --cluster-stability=Disruptive
```

To run only node-specific long-running disruptive tests:

```bash
./openshift-tests run "openshift/disruptive-longrunning" --dry-run | grep "\[sig-node\]" | ./openshift-tests run -f - --cluster-stability=Disruptive
```

### Running User Namespace Tests

```bash
./openshift-tests run "openshift/usernamespace"
```

## Prerequisites

- Make sure to set `oc` binary to match the cluster version
- Make sure to set the kubeconfig to point to a live OCP cluster

## Submitting PRs

### Adding Tests to `openshift/disruptive-longrunning`

Before submitting a PR that adds a test to the `openshift/disruptive-longrunning` suite, run the following payload job and include the results in your PR:

```
/payload-job periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning
```

Useful links for `periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning`:
- [Previous runs (Sippy)](https://sippy.dptools.openshift.org/sippy-ng/jobs/4.22/analysis?filters=%7B%22items%22%3A%5B%7B%22columnField%22%3A%22name%22%2C%22operatorValue%22%3A%22equals%22%2C%22value%22%3A%22periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning%22%7D%5D%7D)
- [Job history for latest runs (Prow)](https://prow.ci.openshift.org/job-history/gs/test-platform-results/logs/periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning)

### Adding TechPreview Tests to `openshift/disruptive-longrunning`

For tests that require TechPreviewNoUpgrade feature gate (like Additional Storage Support in OCP 4.22), use the TechPreview variant of the disruptive-longrunning CI job:

```
# For OCP 4.22 TechPreview testing
/payload-job periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning-techpreview

# For OCP 4.23+ (GA), use standard job
/payload-job periodic-ci-openshift-release-main-nightly-4.23-e2e-aws-disruptive-longrunning
```

**Available platform variants:**
- `-e2e-aws-disruptive-longrunning-techpreview` (AWS - primary for storage tests)
- `-e2e-azure-disruptive-longrunning-techpreview` (Azure)
- `-e2e-gcp-disruptive-longrunning-techpreview` (GCP)
- `-e2e-vsphere-disruptive-longrunning-techpreview` (vSphere)
- `-e2e-metal-ipi-ovn-ipv6-disruptive-longrunning-techpreview` (Bare Metal IPv6)
- `-e2e-metal-ipi-ovn-dual-disruptive-longrunning-techpreview` (Bare Metal dual-stack)

**CI Job Configuration:**
- Job definitions: `ci-operator/config/openshift/release/openshift-release-main__nightly-4.XX.yaml` in [openshift/release](https://github.com/openshift/release) repo
- Feature Set: `TechPreviewNoUpgrade`
- Test Suite: `openshift/disruptive-longrunning`

## Important Notes

- Note that dry-run option won't list the test as it does not connect to a live cluster
- Run `make update` if the test data is changed
