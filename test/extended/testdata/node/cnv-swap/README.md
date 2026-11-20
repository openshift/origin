# CNV Swap Configuration Test Data

This directory contains YAML and configuration files used by the CNV swap tests (`node_swap_cnv.go`).

## Files

### CNV Operator Installation

- `cnv-namespace.yaml` - Namespace for CNV operator (openshift-cnv)
- `cnv-operatorgroup.yaml` - OperatorGroup for CNV
- `cnv-subscription.yaml` - Subscription to install CNV operator from OperatorHub
- `cnv-hyperconverged.yaml` - HyperConverged CR to deploy CNV components

### Kubelet Drop-in Configurations

- `kubelet-limitedswap-dropin.yaml` - Kubelet config with LimitedSwap enabled
- `kubelet-noswap-dropin.yaml` - Kubelet config with NoSwap (default)

## Drop-in Directory Paths

- CNV uses: `/etc/openshift/kubelet.conf.d/`
- Drop-in file: `99-kubelet-limited-swap.conf`

## Usage

### Manual CNV Installation

```bash
# Create namespace
oc apply -f cnv-namespace.yaml

# Create operator group
oc apply -f cnv-operatorgroup.yaml

# Subscribe to CNV operator
oc apply -f cnv-subscription.yaml

# Wait for operator to install
oc get csv -n openshift-cnv -w

# Create HyperConverged CR
oc apply -f cnv-hyperconverged.yaml

# Wait for CNV to be ready
oc get hyperconverged -n openshift-cnv -w
```

### Manual Drop-in Configuration

```bash
# Debug into a node
oc debug node/<node-name>

# Create drop-in file
chroot /host
mkdir -p /etc/openshift/kubelet.conf.d
cat > /etc/openshift/kubelet.conf.d/99-kubelet-limited-swap.conf << 'EOF'
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
memorySwap:
  swapBehavior: LimitedSwap
EOF

# Restart kubelet
systemctl restart kubelet
```

## Test Cases

The configuration files support the following automated tests in `node_swap_cnv.go`.
They are split into two tests:

- `... Kubelet LimitedSwap Drop-in Configuration for CNV with CNV installed` installs CNV
  once, runs the CNV swap cases, then removes CNV and every artifact it adds (namespace,
  autopilot MachineConfigs, kubelet drop-in, `psi=1` and node labels).
- `... Kubelet LimitedSwap Drop-in Configuration without CNV non-CNV kubelet drop-in cases`
  runs on a CNV-free cluster and skips if CNV is already installed.

| TC   | Test        | Description                                                                                                      |
| ---- | ----------- | ---------------------------------------------------------------------------------------------------------------- |
| TC1  | with CNV    | Verify the drop-in directory exists on all worker and control-plane nodes with correct ownership and permissions |
| TC2  | with CNV    | Verify kubelet starts normally with an empty drop-in directory                                                   |
| TC3  | without CNV | Apply LimitedSwap configuration from a drop-in file                                                              |
| TC4  | without CNV | Revert to NoSwap when the drop-in file is removed                                                                |
| TC5  | without CNV | Validate security and permissions of the drop-in directory                                                       |
| TC6  | without CNV | Verify cluster stability with LimitedSwap enabled                                                                |
| TC7  | without CNV | Verify non-CNV workers have no swap configuration                                                                |
| TC8  | with CNV    | Verify 99-* takes precedence over 98-* with multiple drop-in files                                               |
| TC10 | with CNV    | Handle LimitedSwap gracefully when OS-level swap is disabled                                                     |
| TC11 | with CNV    | Work correctly with various swap sizes                                                                           |
| TC12 | with CNV    | Expose swap metrics via Prometheus                                                                               |

TC9 (multi-node consistency: identical drop-in checksum and `swapBehavior` across CNV
nodes) is not included. It required at least two CNV-labeled NodeResource nodes, but the
`openshift/nodes/isolated` suite reserves a single node per test, so it always skipped.

## Running Tests

```bash
# CNV cases (installs CNV once, then removes it and all its artifacts)
./openshift-tests run-test "[Jira:Node/Kubelet][sig-node][Feature:NodeSwap][Suite:openshift/nodes/isolated] Kubelet LimitedSwap Drop-in Configuration for CNV with CNV installed [Timeout:90m]"

# Cases that expect a CNV-free cluster
./openshift-tests run-test "[Jira:Node/Kubelet][sig-node][Feature:NodeSwap][Suite:openshift/nodes/isolated] Kubelet LimitedSwap Drop-in Configuration without CNV non-CNV kubelet drop-in cases"

# Run the whole isolated suite
./openshift-tests run openshift/nodes/isolated
```
