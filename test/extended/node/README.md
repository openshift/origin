# Node E2E Tests

This directory contains OpenShift end-to-end tests for node-related features.

## Test Suites

### Suite: openshift/disruptive-longrunning

Cluster-wide disruptive tests that trigger full worker/master MCP rollouts.
Runs serially (Parallelism: 1).

- **node_e2e/image_registry_config.go** - Container registry config change (OCP-44820) - Verifies search registry update triggers MCO rollout and lands on nodes \[Disruptive\]
- **node_e2e/image_mirror_set.go** - ImageTagMirrorSet and ImageDigestMirrorSet (OCP-57401, OCP-70203) - Verifies IDMS/ITMS coexist with ICSP, tests registries.conf rollout via custom MCP \[Disruptive\]
- **criocredentialprovider.go** - CRI-O credential provider config tests (`CRIOCredentialProviderConfig` feature gate) - Verifies credential provider configuration on worker nodes \[TechPreview\]
- **node_e2e/netns_cleanup.go** - Network namespace cleanup - Verifies kubelet/CRI-O properly deletes network namespace when a pod is deleted \[OTP\]

### Suite: openshift/nodes/isolated

Single-node isolated tests that require exclusive access to a dedicated worker node.
A 3-node worker pool is provisioned at suite start; tests run in parallel via the
NodeResource scheduler (Parallelism: 3).

- **kubeletconfig_features.go** - Tests applying KubeletConfig to custom machine config pools, requires node reboots
- **kubeletconfig_tls.go** - Kubelet TLS configuration upgrade from 1.2 to 1.3 on a custom pool
- **kubelet_secret_pulled_images.go** - Tests kubelet credential verification for image pulls (`KubeletEnsureSecretPulledImages` feature gate). Covers multi-tenancy isolation, credential rotation, ImagePullPolicy behavior, credential verification policy, and registry availability scenarios. Requires `TechPreviewNoUpgrade` or `CustomNoUpgrade` FeatureSet.
- **node_e2e/container_runtime_config.go** - ContainerRuntimeConfig pidsLimit (OCP-45351) and overlaySize (OCP-46313) - Verifies CTRCFG settings are applied via MCO rollout and reflected on nodes \[Disruptive\]
- **node_sizing.go** - Node sizing auto-sizing reserved CPU configuration
- **additional_storage_e2e.go** - Additional Storage configuration and functional tests - Verifies ContainerRuntimeConfig with additionalImageStores, additionalLayerStores, and additionalArtifactStores \[Disruptive\]
- **system_compressible.go** - System compressible CPU enforcement and configuration
- **node_swap_cnv.go** - Kubelet LimitedSwap drop-in configuration for CNV (12 test cases)
- **node_e2e/probe_termination.go** - Probe-level terminationGracePeriodSeconds (OCP-44493) - 3 test cases
- **node_e2e/pdb_drain.go** - PodDisruptionBudget drain blocking (OCP-67564) \[Disruptive\] \[Lifecycle:informing\]
- **runc_upgrade_cases.go** - runc RHCOS 10 upgrade guard tests (OSStreams feature gate) - 3 test cases

### Suite: openshift/conformance/parallel

- **Additional Storage Support API Validation** - API validation tests for additionalArtifactStores, additionalImageStores, and additionalLayerStores CRI-O configuration
  
  **Availability:**
  - **OCP 4.22:** TechPreview (requires TechPreviewNoUpgrade feature gate)
  - **OCP 4.23+/5.0+:** GA (Generally Available)
  
  **Suite:** `openshift/conformance/parallel`  
  **Feature Tag:** `[Feature:AdditionalStorageSupport]`  
  **Sig Tag:** `[sig-node]`  
  **OCPFeatureGate:** `AdditionalStorageConfig`
  
  **Test File:**
  - **additional_storage_api.go** - 13 API validation tests using DryRun (non-disruptive, parallel execution)
    - Combined Additional Stores (3 tests): invalid paths, max count enforcement, duplicate detection
    - Additional Layer Stores (8 tests): comprehensive path validation (empty, relative, spaces, special chars, length, max count, consecutive slashes, duplicates)
    - Additional Image Stores (1 smoke test): path validation wiring
    - Additional Artifact Stores (1 smoke test): path validation wiring
  
  **Requirements:**
  - AdditionalStorageConfig feature gate must be enabled
  - API tests are non-disruptive (use DryRun)
  - Run in parallel with other conformance tests
  - Skip on MicroShift (no MachineConfig support)
  - Skip on Microsoft Azure (known platform issues)
  
  **Running API validation tests:**
  ```bash
  # Run only Additional Storage API validation tests (fast, non-disruptive)
  ./openshift-tests run "openshift/conformance/parallel" --dry-run | \
    grep "\[Feature:AdditionalStorageSupport\]" | \
    ./openshift-tests run -f -
  ```
  
  **Test Coverage (13 tests, ~2-3 min total):**
  - Path format validation (absolute paths, character restrictions, length limits)
  - Count limits enforcement (5 for artifact/layer stores, 10 for image stores)
  - Duplicate path detection within store types
  - Combined store configurations with invalid paths

### Suite: openshift/usernamespace

- **nested_container.go** - Tests running nested containers (podman-in-pod) with user namespaces and nested-container SCC

### Default Suite

- **dra.go** - Tests that DRA (Dynamic Resource Allocation) v1 API is available and beta/alpha APIs are disabled
- **image_volume.go** - Tests mounting container images as volumes in pods, including subPath and error handling
- **node_swap.go** - Tests default kubelet swap settings (failSwapOn and swapBehavior) and rejection of user overrides
- **zstd_chunked.go** - Tests building and running images with zstd:chunked compression format
- **node_e2e/probe_termination.go** - (Moved to `openshift/nodes/isolated` suite)

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

The `openshift/disruptive-longrunning` suite runs cluster-wide disruptive tests (serial).

```bash
./openshift-tests run "openshift/disruptive-longrunning" --cluster-stability=Disruptive
```

### Running Node Isolated Tests

The `openshift/nodes/isolated` suite provisions a dedicated 3-node worker pool and runs
single-node tests in parallel via the NodeResource scheduler.

```bash
./openshift-tests run "openshift/nodes/isolated" --cluster-stability=Disruptive
```

### Running User Namespace Tests

```bash
./openshift-tests run "openshift/usernamespace"
```

## Prerequisites

- Make sure to set `oc` binary to match the cluster version
- Make sure to set the kubeconfig to point to a live OCP cluster

## Submitting PRs

### Adding Tests

- **Cluster-wide disruptive tests** (full worker/master MCP rollouts): tag with `[Suite:openshift/disruptive-longrunning]`.
- **Single-node isolated tests** (custom MCP on one node): tag with `[Suite:openshift/nodes/isolated]` and `[NodeResource:numNodes=1,label=<your_label>]`.

Before submitting a PR, run the relevant payload job and include the results:

```
/payload-job periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning
```

Useful links for `periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning`:
- [Previous runs (Sippy)](https://sippy.dptools.openshift.org/sippy-ng/jobs/4.22/analysis?filters=%7B%22items%22%3A%5B%7B%22columnField%22%3A%22name%22%2C%22operatorValue%22%3A%22equals%22%2C%22value%22%3A%22periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning%22%7D%5D%7D)
- [Job history for latest runs (Prow)](https://prow.ci.openshift.org/job-history/gs/test-platform-results/logs/periodic-ci-openshift-release-main-nightly-4.22-e2e-aws-disruptive-longrunning)

## Important Notes

- Note that dry-run option won't list the test as it does not connect to a live cluster
- Run `make update` if the test data is changed
