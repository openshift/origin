# Topology transition suites

These suites cover SNO to compact HA on platform `None`. Enable the
`MutableTopology` feature gate. The cluster needs the API and CCO code that
reports `Infrastructure.status.topologyTransitionStatus`. The `oc` binary on
`PATH` must provide `oc adm transition topology`.

Set all six variables before either suite:

```bash
export TARGET_CONTROL_PLANE_TOPOLOGY=HighlyAvailable
export TARGET_INFRASTRUCTURE_TOPOLOGY=HighlyAvailable
export TARGET_CONTROL_PLANE_NODES=3
export TARGET_COMPUTE_NODES=0
export TARGET_ARBITER_NODES=0
export TARGET_CONTROL_PLANE_NODES_SCHEDULABLE=3
```

| Variable | Meaning |
| --- | --- |
| `TARGET_CONTROL_PLANE_TOPOLOGY` | Target control-plane topology |
| `TARGET_INFRASTRUCTURE_TOPOLOGY` | Target infrastructure topology |
| `TARGET_CONTROL_PLANE_NODES` | Exact number of control-plane nodes, excluding arbiters |
| `TARGET_COMPUTE_NODES` | Exact number of worker nodes, excluding control-plane and arbiter nodes |
| `TARGET_ARBITER_NODES` | Exact number of arbiter nodes |
| `TARGET_CONTROL_PLANE_NODES_SCHEDULABLE` | Exact number of ready, uncordoned control-plane nodes with the worker role |

Counts are required non-negative integers. Zero means exactly zero.
The schedulable count cannot exceed the control-plane count.
`TARGET_HA_COMPACT` and `TARGET_TRANSITION` are no longer used.

## Four-step workflow

1. Install SNO on platform `None` with `MutableTopology` enabled.
2. Run the preflight suite on the single-node cluster:

   ```bash
   ./openshift-tests run openshift/topology-transitions/preflight
   ```

   It waits for a current `TopologyTransitionsEvaluated=True` report and finds
   the entry by source and target topology. It passes when availability is
   `False` with reason `PreflightCheckFailed`, these five checks fail, and all
   remaining checks pass:

   - `ControlPlaneNodeCountSatisfied`
   - `ControlPlaneNodesSchedulable`
   - `ControlPlaneNodesReady`
   - `ControlPlaneNodesAreWorkers`
   - `EtcdVotingMembersSatisfied`

   It checks that only one node and one voting member exist. It makes no
   transition request, changes no nodes, and checks that the Infrastructure
   spec stays unchanged. It runs serially with stable monitoring and a
   30-minute limit.

3. Provision and join two more control-plane nodes. Node provisioning happens
   outside origin. All three nodes must be ready, uncordoned workers.
4. Run the transition suite:

   ```bash
   ./openshift-tests run openshift/topology-transitions
   ```

   It waits for passing preflight checks, creates and checks a baseline
   workload, then runs:

   ```bash
   OC_ENABLE_CMD_TRANSITION_TOPOLOGY=true oc adm transition topology \
     --control-plane="$TARGET_CONTROL_PLANE_TOPOLOGY" \
     --infrastructure="$TARGET_INFRASTRUCTURE_TOPOLOGY" \
     --confirm
   ```

   It requires `TopologyTransitionCompleted=True` for the current request,
   both target topologies, settled operators, exact node counts, healthy etcd
   with the expected voting members, and a ready baseline workload. It runs
   serially with disruptive monitoring and a 150-minute limit. The transition
   is one-way.

Both suites fail on missing or invalid configuration. Neither suite selects
tests from the other. To list their tests without running them:

```bash
./openshift-tests run openshift/topology-transitions/preflight --dry-run
./openshift-tests run openshift/topology-transitions --dry-run
```

## Add a transition type

Add an entry to `transitionDefinitions` in `transition_definitions.go`:

- `Name`: a label for test progress messages.
- `Source`: control-plane and infrastructure topology, plus an optional platform type.
- `Target`: both target topology values.
- `SourceNodes`: exact node counts expected during the preflight suite.
- `ExpectedPreflightChecks`: each named check's expected preflight result (`true` or `false`).

The suites pick one definition from the live source state and the configured
target topology. Missing or duplicate matches fail the test. The preflight
suite uses that definition's node counts and check results. Its expected etcd
voting count is the source control-plane count plus the arbiter count.
Availability must be false if any expected check fails, and true otherwise.
Checks absent from the map must pass. The main suite requires all checks to pass
before it makes the request. Target node counts still come from the lane variables above.
