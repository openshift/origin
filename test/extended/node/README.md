# Node E2E Tests

This directory contains OpenShift end-to-end tests for node-related features.

## Test Suites

### Suite: openshift/disruptive-longrunning

- **kubeletconfig_features.go** - Tests applying KubeletConfig to custom machine config pools, requires node reboots
- **nested_container.go** - Tests running nested containers (podman-in-pod) with user namespaces and nested-container SCC

### Default Suite

- **dra.go** - Tests that DRA (Dynamic Resource Allocation) v1 API is available and beta/alpha APIs are disabled
- **image_volume.go** - Tests mounting container images as volumes in pods, including subPath and error handling
- **node_swap.go** - Tests default kubelet swap settings (failSwapOn and swapBehavior) and rejection of user overrides
- **zstd_chunked.go** - Tests building and running images with zstd:chunked compression format

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

## Prerequisites

- Make sure to set `oc` binary to match the cluster version
- Make sure to set the kubeconfig to point to a live OCP cluster

## Important Notes

- Note that dry-run option won't list the test as it does not connect to a live cluster
- Run `make update` if the test data is changed
