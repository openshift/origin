# Node E2E Tests

This directory contains OpenShift end-to-end tests for node-related features.

## Test Suites

### Suite: openshift/disruptive-longrunning

- **node_e2e/container_runtime_config.go** - ContainerRuntimeConfig pidsLimit (OCP-45351) and overlaySize (OCP-46313) - Verifies CTRCFG settings are applied via MCO rollout and reflected on nodes \[Disruptive\]
- **node_e2e/image_mirror_set.go** - ImageDigestMirrorSet and ImageTagMirrorSet (OCP-57401, OCP-70203) - Verifies registries.conf reflects IDMS/ITMS configuration and that ICSP/IDMS/ITMS can coexist \[Disruptive\] \[Serial\]
- **node_e2e/image_registry_config.go** - Container registry config change (OCP-44820) - Verifies search registry update triggers MCO rollout and lands on nodes \[Disruptive\]
- **node_e2e/pdb_drain.go** - PodDisruptionBudget drain blocking (OCP-67564) - Tests that node drain is blocked when PDB has minAvailable=100% with empty selector \[Disruptive\]

### Suite: openshift/conformance/parallel

- **node_e2e/initcontainer.go** - Init container restart behavior (OCP-38271) - Verifies init containers do not restart when the exited init container is removed from the node
- **node_e2e/netns_cleanup.go** - Network namespace cleanup (OCP-56266) - Verifies kubelet/CRI-O properly deletes the network namespace when a pod is deleted
- **node_e2e/node.go** - Kubelet log level (KUBELET_LOG_LEVEL), cgroupv2 default validation, and dev fuse enablement in CRI-O (OCP-80983, OCP-70987)
- **node_e2e/probe_termination.go** - Probe-level terminationGracePeriodSeconds (OCP-44493) - Tests configurable termination grace period for liveness and startup probes, including fallback to pod-level config when probe-level is not set

## Directory Structure

### Test Files
- All `*.go` files under `node_e2e/` are Ginkgo-based test suites
- Each file focuses on a specific node feature

### Utility Files
- **node_utils.go** - Shared helper functions for node selection, exec-on-node, and MachineConfigPool rollout waiting
- **node_mcp_helpers.go** - Custom MachineConfigPool creation/cleanup helpers
- **../imagepolicy/imagepolicy_helpers.go** - MachineConfigPool spec-name helpers shared with `node_e2e` tests

### Test Data
Test fixtures are referenced via `exutil.FixturePath` from:
- `testdata/node/node_e2e/` - Pod fixtures (e.g. dev fuse test pod)

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

## Submitting PRs

### Adding Tests to `openshift/disruptive-longrunning`

Before submitting a PR that adds a test to the `openshift/disruptive-longrunning` suite, run the following payload job and include the results in your PR:

```
/payload-job periodic-ci-openshift-release-main-nightly-4.19-e2e-aws-disruptive-longrunning
```

Useful links for `periodic-ci-openshift-release-main-nightly-4.19-e2e-aws-disruptive-longrunning`:
- [Previous runs (Sippy)](https://sippy.dptools.openshift.org/sippy-ng/jobs/4.19/analysis?filters=%7B%22items%22%3A%5B%7B%22columnField%22%3A%22name%22%2C%22operatorValue%22%3A%22equals%22%2C%22value%22%3A%22periodic-ci-openshift-release-main-nightly-4.19-e2e-aws-disruptive-longrunning%22%7D%5D%7D)
- [Job history for latest runs (Prow)](https://prow.ci.openshift.org/job-history/gs/test-platform-results/logs/periodic-ci-openshift-release-main-nightly-4.19-e2e-aws-disruptive-longrunning)

## Important Notes

- Note that dry-run option won't list the test as it does not connect to a live cluster
- Run `make update` if the test data is changed
