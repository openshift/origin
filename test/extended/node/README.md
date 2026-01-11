# Node E2E Tests

## Running Long-Running Node Tests

To run longrunning node tests on a cluster manually, use the command:

```bash
./openshift-tests run "openshift/node/longrunning" --cluster-stability=Disruptive
```

## Prerequisites

- Make sure to set `oc` binary to match the cluster version
- Make sure to set the kubeconfig to point to a live OCP cluster

## Important Notes

- Note that dry-run option won't list the test as it does not connect to a live cluster
- Run `make update` if the test data is changed
