# Workload-Aware Scheduling Tests and Test Plan

This directory holds the OpenShift e2e specs for workload-aware scheduling that
have no upstream e2e equivalent, and this README is the test plan for the
`openshift/workload-scheduling` suite that runs them together with the relevant
upstream Kubernetes e2e tests.

- **Tracking**: OCPKUEUE-888
- **Related feature**: OCPSTRAT-2622

This is a validation plan, not an execution report. It describes what to run and
how to judge the results; it does not record the outcome of any run.

## Scope

The plan covers the workload-aware scheduling feature gates on an OCP 5.1
cluster or greater.

- Workload and PodGroup API operations
- Gang and basic PodGroup scheduling
- Workload-aware preemption
- DRA resource claims declared on a PodGroup
- Topology-aware gang placement

## Suite definition

The `openshift/workload-scheduling` suite is defined in
[`pkg/testsuites/standard_suites.go`](../../../../pkg/testsuites/standard_suites.go)
and was added by [openshift/origin#31813](https://github.com/openshift/origin/pull/31813).
It selects tests whose name contains any of:

- `[Suite:openshift/workload-scheduling]`
- `[FeatureGate:GenericWorkload]`
- `[FeatureGate:DRAWorkloadResourceClaims]`
- `[FeatureGate:TopologyAwareWorkloadScheduling]`

plus the standard `[Early]`/`[Late]` conformance tests that
[`withStandardEarlyOrLateTests`](../../../../pkg/testsuites/filters.go) adds.
The topology specs in this directory are labelled
`[OCPFeatureGate:TopologyAwareWorkloadScheduling]` and are selected through their
`[Suite:openshift/workload-scheduling]` tag.

The suite runs with `Parallelism: 1`, because the topology specs label nodes and
add extended resources to them, and with a per-test timeout of 45 minutes.

| Feature gate | Covered by | Upstream enhancement |
|---|---|---|
| `GenericWorkload` | Upstream e2e: Workload/PodGroup API, gang scheduling, workload-aware preemption | [KEP-4671](https://github.com/kubernetes/enhancements/issues/4671) |
| `DRAWorkloadResourceClaims` | Upstream e2e: DRA PodGroup claims | [KEP-5729](https://github.com/kubernetes/enhancements/issues/5729) |
| `TopologyAwareWorkloadScheduling` | This directory: [`topology_aware.go`](topology_aware.go) | [KEP-5732](https://github.com/kubernetes/enhancements/issues/5732) |

All three gates are enabled by the `TechPreviewNoUpgrade` feature set (see
[`features/features.go`](https://github.com/openshift/api/blob/master/features/features.go)
in openshift/api). Upstream Kubernetes covers `TopologyAwareWorkloadScheduling`
only with integration tests
([`test/integration/scheduler/podgroup/topology_aware_scheduling`](https://github.com/openshift/kubernetes/tree/release-5.1/test/integration/scheduler/podgroup/topology_aware_scheduling)),
which run against an in-process API server, so the topology specs live here.

Before the run, `openshift-tests` checks the gates named in each test's
`[FeatureGate:...]` and `[OCPFeatureGate:...]` labels against the gates the
cluster reports as enabled or disabled (see
[`pkg/test/filters/cluster_state.go`](../../../../pkg/test/filters/cluster_state.go)):

- If any of those gates is reported as disabled, the test is removed from the
  run. It does not appear in the results, not even as skipped.
- Otherwise the filter keeps the test, even when it names a gate that the
  cluster does not report at all, such as an upstream gate that openshift/api
  does not define. It treats an unreported gate as enabled, so it does not keep
  out a test whose gate is actually off. The next section describes one such
  spec.

### Selected spec outside this plan

The `[FeatureGate:GenericWorkload]` selector also matches the upstream spec
`[sig-apps] Job should create Workload and PodGroup for gang-eligible Job`
([`test/e2e/apps/job.go`](https://github.com/openshift/kubernetes/blob/release-5.1/test/e2e/apps/job.go)).
That spec also requires `WorkloadWithJob`:

- In Kubernetes 1.37 the gate is alpha and defaults to off
  ([`pkg/features/kube_features.go`](https://github.com/openshift/kubernetes/blob/release-5.1/pkg/features/kube_features.go)).
- It is not in openshift/api's feature-gate definitions, so the
  `TechPreviewNoUpgrade` feature set does not turn it on.
- The kube-controller-manager operator builds `--feature-gates` only from the
  gates listed in the cluster's FeatureGate status
  ([`observe_config_controller.go`](https://github.com/openshift/cluster-kube-controller-manager-operator/blob/release-5.1/pkg/operator/configobservation/configobservercontroller/observe_config_controller.go),
  using library-go's
  [`observe_featuregates.go`](https://github.com/openshift/library-go/blob/master/pkg/operator/configobserver/featuregates/observe_featuregates.go)),
  so a gate missing from that list keeps its upstream default.

The cluster does not report the gate as disabled, so `openshift-tests` still
selects the spec. Record whether `WorkloadWithJob` is enabled on the tested
cluster. Unless it is, record the spec's result separately and do not count it
toward this plan's pass or fail decision.

## Environment and prerequisites

1. A disposable OCP 5.1 cluster installed with the `TechPreviewNoUpgrade` feature
   set, and an `openshift-tests` binary that contains this suite and the topology specs.
2. At least **three Ready, schedulable worker nodes**. Nodes labelled
   `node-role.kubernetes.io/master` or `node-role.kubernetes.io/control-plane`
   are not counted. With fewer than three, the topology specs skip; a skip is not
   validation of topology behavior.
3. A cluster-admin kubeconfig. The tests patch node labels and node status
   (extended resources), create cluster-scoped PriorityClasses and
   DeviceTaintRules, and deploy the upstream DRA test driver.
4. Run the suite as configured, serially. Do not raise its parallelism.
5. Record the payload, the `openshift-tests` revision, the cluster's enabled and
   disabled feature gates, the worker inventory, and the selected and skipped
   tests. The DRA PodGroup specs carry a `KubeletMinVersion:1.36` label; confirm
   that the node kubelet versions satisfy it before interpreting any DRA result.
6. Before the run, record a baseline for every Ready, schedulable node,
   including schedulable control-plane nodes: the
   `workload-scheduling.openshift.io/rack` label and the `status.capacity` and
   `status.allocatable` values of `example.com/gang-slot` and
   `example.com/combined-resource`, noting explicitly when a label or resource is
   absent. The topology specs put back the previous label and `allocatable`
   value, or remove what they added. Each upstream preemption spec instead sets
   `example.com/combined-resource` to `2` on a randomly chosen Ready, schedulable
   node and then deletes it, so an earlier value can only be restored from this
   baseline.

## Test scenarios and expected results

Upstream specs run from the Kubernetes test extension (`k8s-tests-ext`) shipped
in the release payload. The source links below point at the openshift/kubernetes
`release-5.1` branch; the code that runs is whatever the tested payload contains.

### 1. Workload and PodGroup APIs (upstream)

Source: [`test/e2e/scheduling/workload.go`](https://github.com/openshift/kubernetes/blob/release-5.1/test/e2e/scheduling/workload.go),
`[sig-scheduling] Workload` → `CRUD Tests`.

- `Workload API availability`: runs the standard resource conformance checks
  (create, get, list, watch, update, patch, delete and delete-collection) against
  the namespaced `scheduling.k8s.io/v1beta1` Workload.
- `PodGroup API availability`: the same checks on PodGroup, plus a status update
  that adds a `PodGroupInitiallyScheduled` condition.
- Expected: every operation and read-back assertion succeeds.

### 2. Gang and basic scheduling (upstream)

Source: [`test/e2e/scheduling/gang_scheduling.go`](https://github.com/openshift/kubernetes/blob/release-5.1/test/e2e/scheduling/gang_scheduling.go),
`[sig-scheduling] GangScheduling`.

- `should schedule pods only when quorum is reached`: with a gang PodGroup of
  `minCount: 2`, the first pod stays Pending for the observation window (10
  seconds); once the second pod is created, both reach Running.
- `should schedule pods with basic scheduling policy`: with a `Basic` policy,
  each pod reaches Running without waiting for other group members.

### 3. Workload-aware preemption (upstream)

Source: [`test/e2e/scheduling/workload_aware_preemption.go`](https://github.com/openshift/kubernetes/blob/release-5.1/test/e2e/scheduling/workload_aware_preemption.go),
`[sig-scheduling] WorkloadAwarePreemption`, gated by `GenericWorkload`.

Each entry adds two units of an extended resource to one node, fills it with a
low-priority victim gang (`minCount: 2`, one unit per pod), and then creates a
high-priority pod that needs one unit on the same node. In every entry the
high-priority pod must reach Running on that node.

- `should preempt entire group with All disruption mode by a pod group`: the
  victim group uses the `All` disruption mode and the preemptor belongs to a
  PodGroup. Expected: every victim pod is preempted (deleted or terminating)
  within 30 seconds after the preemptor is Running.
- `should preempt partial group with Single disruption mode by a pod group`: the
  victim group uses the `Single` disruption mode and the preemptor belongs to a
  PodGroup. Expected: at least one victim is preempted within 30 seconds after the
  preemptor is Running, and not all victims are preempted for the following 5
  seconds.
- `should preempt partial group with Single disruption mode by an individual pod`:
  same victim group as above, but the preemptor is a pod outside any PodGroup.
  Expected: the same partial-preemption outcome.

### 4. DRA PodGroup claims (upstream)

Source: [`test/e2e/dra/dra.go`](https://github.com/openshift/kubernetes/blob/release-5.1/test/e2e/dra/dra.go),
`[sig-node] [DRA]`, gated by `GenericWorkload` and `DRAWorkloadResourceClaims`.

- `allocates devices for PodGroups`: the test creates a ResourceClaim, a Workload
  whose PodGroup template references that claim, a PodGroup, and one pod that
  uses the claim.
- `generates claims from templates for PodGroups`: the same, but the PodGroup
  claim comes from a ResourceClaimTemplate.
- Expected for both: the pod reaches Running and its container sees the claim
  parameters that the DRA test driver injects.

Device-taint variants, which also carry the `DRADeviceTaints` gate:

- `NoSchedule keeps pod with PodGroup claim pending`: the driver's devices are
  tainted `NoSchedule`; the pod must become Unschedulable.
- `NoSchedule can be tolerated for PodGroup claims`: the claim template tolerates
  `NoSchedule`; the pod must reach Running.
- `DeviceTaintRule evicts pod with PodGroup claim` (also requires
  `DRADeviceTaintRules`): after the pod runs, a `NoExecute` DeviceTaintRule must
  evict it, leaving a `DisruptionTarget` condition with reason
  `DeletionByDeviceTaintManager`.

OCP 5.1 is based on Kubernetes 1.37, where `DRADeviceTaints` and
`DRADeviceTaintRules` are GA; `DRADeviceTaintRules` is also enabled by
`TechPreviewNoUpgrade`. All five DRA PodGroup specs are therefore expected to be
selected on a TechPreview cluster.

Each of these specs creates a single consuming pod, so none of them shows that
several pods in one PodGroup share a claim at the same time. If concurrent
multi-pod sharing is part of acceptance, add explicit multi-consumer validation
before treating it as covered.

### 5. Topology-aware gang placement (this directory)

Source: [`topology_aware.go`](topology_aware.go) and [`helpers.go`](helpers.go),
`[sig-scheduling][OCPFeatureGate:TopologyAwareWorkloadScheduling][Suite:openshift/workload-scheduling][Serial]`.

Both specs build their own topology with the node label
`workload-scheduling.openshift.io/rack` and create contention with the extended
resource `example.com/gang-slot`, one unit per selected worker. Each spec creates
a gang PodGroup with `minCount: 2` whose `spec.schedulingConstraints.topology`
names that key, then reads it back and requires the stored object to keep
exactly that one topology constraint. The API server drops the field when the
gate is off, so a missing field fails the spec.

#### `should place an entire gang inside a single topology domain`

1. Label two workers `rack-a` and a third `rack-b`.
2. Create two pods in the gang, each requesting one slot.
3. Expected: both pods reach Running, and both run in `rack-a`, the only domain
   that can hold the whole gang.

#### `should keep a gang pending when no single topology domain can hold it`

1. Put three workers into three different racks, one slot each. The cluster has
   enough slots for the gang, but no single rack does.
2. Create the two-pod gang.
3. Expected: both pods are reported Unschedulable and neither is assigned a node
   for a further 30 seconds of observation. Any partial placement is a failure.

### 6. Feature-set selection and cleanup

- Optionally compare the suite's test list on a default feature-set cluster
  with the list on TechPreview. On the default feature set the workload gates are
  disabled, so only the standard `[Early]`/`[Late]` tests remain. A default-only
  run is not positive validation of the feature.
- After the run, including after failures, confirm that test namespaces,
  PriorityClasses, DeviceTaintRules and DRA test-driver objects (such as
  DeviceClasses and ResourceSlices) are gone. Compare each node's topology label
  and extended resources with the baseline from prerequisite 6, and manually
  restore any value that was overwritten or removed; check
  `example.com/combined-resource` in particular. Treat any difference or leftover
  as a validation failure to investigate.

## Running the suite

```bash
make openshift-tests

# With KUBECONFIG pointing at the target cluster, list the tests the suite selects.
./openshift-tests run openshift/workload-scheduling --dry-run

# Run the suite.
./openshift-tests run openshift/workload-scheduling
```

## Continuous periodic coverage

[openshift/release#88950](https://github.com/openshift/release/pull/88950) added a
daily periodic for this suite, defined in
[`openshift-kubernetes-release-5.1__periodics.yaml`](https://github.com/openshift/release/blob/main/ci-operator/config/openshift/kubernetes/openshift-kubernetes-release-5.1__periodics.yaml):

- Job: `periodic-ci-openshift-kubernetes-release-5.1-periodics-e2e-aws-ovn-techpreview-workload-scheduling`
- Schedule: daily at 04:41 UTC (`41 4 * * *`)
- Release: latest OCP 5.1 nightly
- Workflow: `openshift-e2e-aws-ovn`, cluster profile `openshift-org-aws`
- Environment: `FEATURE_SET=TechPreviewNoUpgrade`, `TEST_SUITE=openshift/workload-scheduling`

Use this periodic as ongoing regression coverage. The configuration establishes
the job; it says nothing about whether any run has passed. For acceptance,
capture a completed run's URL, payload, test inventory and results.

## Execution evidence and acceptance criteria

- Run the suite through the periodic or an equivalent TechPreview environment.
- Keep the run URL, payload and `openshift-tests` revision, JUnit results, the
  reason for every skipped or unselected test and, for failures, the scheduler
  events, PodGroup and pod state, DRA objects and driver logs, and node topology
  and resource state. Leave out credentials and other sensitive data.
- **Pass**: the API, scheduling, preemption, DRA and both topology scenarios pass
  on the intended TechPreview environment, the negative assertions hold for their
  full observation windows, and cleanup is complete.
- **Fail**: any in-scope spec fails, a gang is partially placed, or test
  resources, labels or extended resources are left behind.
- **Skipped or not selected**: never counts as a pass. Required specs must not
  skip silently; fix the environment and rerun. If any DRA PodGroup spec is not
  selected, record why.
- Record unresolved failures and coverage gaps instead of treating them as passes.

## Out of scope and known gaps

- `PodGroupPreemptionPolicy` is deliberately not covered: upstream Kubernetes
  removes the gate in v1.38
  ([kubernetes/kubernetes#143266](https://github.com/kubernetes/kubernetes/pull/143266))
  and openshift/api `master` no longer registers it. The gate still exists in
  Kubernetes 1.37, so record whether the tested cluster enables it. The
  workload-aware preemption specs above are gated by `GenericWorkload` only.
- The Job-managed Workload spec that the suite also selects depends on
  `WorkloadWithJob`, which is off by default upstream and is not part of the
  `TechPreviewNoUpgrade` feature set (see
  [Selected spec outside this plan](#selected-spec-outside-this-plan)).
- Concurrent multi-pod claim sharing within a PodGroup (see scenario 4).
- Upgrade, large-scale and performance testing, vendor-specific DRA hardware
  certification, and broader platform matrices, unless separately requested.
