# Topology Manager testsuite

## summary

Tests in this directory tree check the behaviour of the topology manager.
To work correctly, the topology manager orchestrates various other components like cpu manager and device manager.
Furthermore, the topology manager is most useful to align device resources, like SRIOV VFs.
All of the above means that the configuration is complex and may require cluster-specific tuning.

## test configuration

The tests tries very hard to have sensible defaults, autodetect most of the parameters they need, and in general
to be a hassle-free experience. However, due to the amount of combinations, this is not always possible.

The topology manager testsuite behaviour can be configured using the following environment variables

### `TOPOLOGY_MANAGER_TEST_STRICT`

If the testing prerequisites aren't met, the relevant topology manager tests are skipped.
If this variable is defined, regardless of the value, the tests fail instead of skip.

### `ROLE_WORKER`

The testsuite need to inspect the node YAML to auto-tune themselves (e.g. discover the amount of available cores).
The testsuite thus need to find the worker nodes.
Use this variable to change the kubernetes label to be used to search for the worker nodes.

### `RESOURCE_NAME`

The testsuite need a SRIOV device resource to run alignment tests.
Use this variable to change the resource name the test should use.
The resource name depends on the
[SRIOV operator settings](https://docs.openshift.com/container-platform/4.2/networking/multiple_networks/configuring-sr-iov.html#configuring-sr-iov-devices_configuring-sr-iov).
Examples of resource names are: `openshift.io/sriovnic`, `openshift.io/dpdknic`.

### `SRIOV_NETWORK_NAMESPACE`

The testsuite runs a basic connectivity test to ensure the NUMA-aligned devices are functional.
Use this variable to set the namespace on which to look for the SRIOV network to join to exercise the connectivity.

### `SRIOV_NETWORK`

The testsuite runs a basic connectivity test to ensure the NUMA-aligned devices are functional.
Use this variable to set the SRIOV network to join to exercise the connectivity.

### `IP_FAMILY`

The testsuite runs a basic connectivity test to ensure the NUMA-aligned devices are functional.
Use this variable to set the IP family to use for the test: "v4" or "v6". Default is "v4".
