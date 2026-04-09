# Node E2E Tests

## Running Long-Running Disruptive Tests

The `openshift/disruptive-longrunning` suite is a general-purpose suite for long-running disruptive tests
across all teams. Node team tests are tagged with `[sig-node]` to identify them.

To run the entire long-running disruptive test suite on a cluster manually, use the command:

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
