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

The configuration files support the following automated tests in `node_swap_cnv.go`:

| TC | Description |
|----|-------------|
| TC1 | Verify silent creation and ownership of drop-in directory on CNV nodes |
| TC2 | Verify kubelet starts normally with empty directory |
| TC3 | Apply LimitedSwap configuration from drop-in file |
| TC4 | Revert to NoSwap when drop-in file is removed |
| TC5 | Verify control plane kubelets ignore drop-in config |
| TC6 | Verify drop-in directory is auto-recreated after deletion |
| TC7 | Validate security and permissions of drop-in directory |
| TC8 | Verify cluster stability with LimitedSwap enabled |
| TC9 | Verify non-CNV workers have no swap configuration |
| TC10 | Apply correct precedence with multiple files |
| TC11 | Maintain consistent configuration with checksum verification across CNV nodes |
| TC12 | Handle LimitedSwap config gracefully when OS swap is disabled |
| TC13 | Work correctly with various swap sizes |
| TC14 | Expose swap metrics correctly via Prometheus |

## Running Tests

```bash
# Run individual test
./openshift-tests run-test "[Jira:Node/Kubelet][sig-node][Feature:NodeSwap][Serial][Disruptive][Suite:openshift/nodes/cnv] Kubelet LimitedSwap Drop-in Configuration for CNV TC1: should verify silent creation and ownership of drop-in directory on CNV nodes"

# Run entire suite
./openshift-tests run openshift/nodes/cnv
```
