# DRA Node E2E Test Design for OpenShift on NVIDIA Hardware

**JIRA:** [OCPNODE-4664](https://redhat.atlassian.net/browse/OCPNODE-4664)
**Author:** Sai Ramesh Vanka
**Date:** 2026-08-25
**Status:** Draft

---

## 1. Objective

Design and implement sig-node focused DRA (Dynamic Resource Allocation) e2e tests for
OpenShift that validate kubelet, CRI-O runtime, and node-level behaviors on real NVIDIA
GPU hardware. These tests must **not duplicate** existing upstream or OpenShift tests but
instead cover OpenShift-specific node concerns that are currently untested.

**Scope:** All DRA node features **except** DRA Partitionable Devices (KEP-4815).

**Target Repository:** `openshift/origin` under `test/extended/node/dra/`

---

## 2. Existing Test Landscape

### 2.1 Upstream kubernetes/kubernetes

| Suite | Location | Driver | Focus |
|-------|----------|--------|-------|
| Cluster E2E | `test/e2e/dra/dra.go` (~4800 lines) | Mock test-driver | Scheduling, API lifecycle, conformance |
| Node E2E | `test/e2e_node/dra_test.go` (~2170 lines) | Mock test-driver | Kubelet plugin registration, prepare/unprepare, ResourceSlice lifecycle, health monitoring, kubelet restart recovery |
| Upgrade E2E | `test/e2e_dra/*.go` | Mock test-driver | Feature gate lifecycle (enable/disable/re-enable) |
| Integration | `test/integration/dra/` | None (unit) | Scheduler logic, binding conditions |

**Key detail:** All upstream e2e tests use an in-tree mock DRA driver with gRPC proxy
into the test binary. They test *mechanisms* but never exercise a real hardware driver or
CRI-O as the runtime.

### 2.2 OpenShift origin (`test/extended/node/dra/`)

| Suite | Driver | Tests |
|-------|--------|-------|
| `example/example_dra.go` | `gpu.example.com` (mock) | Single/multi device alloc, sharing, template, cleanup |
| `nvidia/nvidia_dra.go` | NVIDIA DRA driver | Single/multi GPU alloc, sharing, template, cleanup (validates nvidia-smi + CDI) |
| `partitionable/partitionable_dra.go` | `gpu.example.com` | SharedCounters, partition alloc, counter exhaustion |
| `dra.go` (API test) | None | Confirms only `resource.k8s.io/v1` API is available |

**Key detail:** The NVIDIA tests validate basic allocation scenarios (5 tests) but do not
exercise any node-level behaviors like kubelet restart, health monitoring, PodResources
API, SCC enforcement, SELinux, or CRI-O CDI specifics.

### 2.3 Coverage Gap Summary

| Area | Upstream | OCP Origin | Gap? |
|------|:--------:|:----------:|:----:|
| Basic device allocation on real GPU | - | Yes | No |
| Kubelet plugin lifecycle (mock) | Yes | - | No (mock is fine) |
| **CRI-O CDI injection validation** | - | - | **YES** |
| **SELinux with DRA devices** | - | - | **YES** |
| **SCC enforcement for DRA workloads** | - | - | **YES** |
| **Node drain with DRA-claimed pods** | - | - | **YES** |
| **Kubelet restart with real GPU claims** | Mock only | - | **YES** |
| **PodResources API for DRA (gRPC)** | - | - | **YES** |
| **Device health in pod status (real HW)** | Mock only | - | **YES** |
| **ResourceClaim status fields (real HW)** | Mock only | - | **YES** |
| **Admin access namespace gating** | Mock only | - | **YES** |
| **Device taints (real HW)** | Mock only | - | **YES** |
| **Node reboot with active DRA claims** | - | - | **YES** |
| **Multiple containers sharing GPU claim** | Mock only | - | **YES** |
| **DRA + extended resources (real HW)** | Mock only | - | **YES** |
| **CDI hot-reload on CRI-O** | - | - | **YES** |
| **DRA metrics (kubelet)** | Mock only | - | **YES** |

---

## 3. Proposed Test Categories

All tests are `[sig-node]` labeled and require real NVIDIA GPU hardware. They are
organized by node-level concern rather than by DRA feature, to emphasize the Kubelet/
runtime angle.

### Category 1: CRI-O CDI Integration

**Why this matters:** Upstream tests run on containerd. CRI-O has its own CDI
implementation with different hot-reload behavior, directory scanning, and SELinux
integration. These tests validate the runtime layer that is unique to OpenShift.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 1.1 | CDI device injection via CRI-O | Deploy pod with GPU claim, verify `/dev/nvidia*` devices present inside container, verify environment variables injected by CDI spec | None (GA) |
| 1.2 | CDI spec file presence | After GPU claim is allocated and pod is running, verify CDI spec JSON files exist under `/var/run/cdi/` on the node with correct structure | None (GA) |
| 1.3 | Multiple CDI devices in single container | Pod claiming 2+ GPUs, verify all CDI device IDs appear in the CRI ContainerConfig and all devices accessible in container | None (GA) |
| 1.4 | CDI injection with init containers | Pod with init container referencing GPU claim, verify CDI devices available in init container, then main container | None (GA) |

**Validation approach:**
- `oc debug node/<gpu-node>` or privileged pod to inspect `/var/run/cdi/` on the host
- `nvidia-smi` inside the container to confirm GPU visibility
- `oc get pod -o jsonpath` to inspect container device annotations
- Check CRI-O logs for CDI processing messages

### Category 2: SELinux and Security Context

**Why this matters:** OpenShift enforces SELinux in Enforcing mode by default. GPU device
nodes carry SELinux types (`nv_device_t`, `dri_device_t`) that must be accessible from
the container's `container_t` domain. CDI specs must handle relabeling. This is entirely
untested upstream.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 2.1 | GPU access under SELinux Enforcing | Deploy GPU workload pod, verify `nvidia-smi` works, verify no SELinux AVC denials in audit log | None (GA) |
| 2.2 | Restricted SCC with DRA workload | Deploy GPU pod using `restricted-v2` SCC (not privileged), verify GPU access works via CDI injection without privilege escalation | None (GA) |
| 2.3 | Non-privileged user with GPU | Pod with `runAsNonRoot: true` and a non-root user, verify GPU device access via DRA claim | None (GA) |
| 2.4 | SCC audit for DRA driver pods | Verify NVIDIA DRA driver DaemonSet pods run with expected SCC (privileged/custom), verify workload pods do NOT require privileged SCC | None (GA) |

**Validation approach:**
- `oc get pod -o jsonpath='{.metadata.annotations.openshift\.io/scc}'` to verify SCC
- `oc debug node/<gpu-node> -- ausearch -m AVC -ts recent` for SELinux denials
- `getenforce` inside debug pod to confirm Enforcing mode
- Run `nvidia-smi` and CUDA workload inside restricted pod

### Category 3: Kubelet Restart and State Recovery

**Why this matters:** Upstream tests this with a mock driver. On real hardware, kubelet
restart must re-discover the DRA driver plugin, re-read the checkpoint file, and ensure
active GPU claims remain prepared. CRI-O must continue serving containers with CDI
devices. This validates the real checkpoint + re-registration flow.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 3.1 | Pod survives kubelet restart with active GPU claim | Deploy GPU pod, verify running, restart kubelet (via `systemctl restart kubelet` on debug node), verify pod continues running with GPU access | None (GA) |
| 3.2 | DRA driver re-registers after kubelet restart | After kubelet restart, verify DRA driver plugin re-registers via plugin watcher, ResourceSlices are re-published | None (GA) |
| 3.3 | Pending GPU pod completes after kubelet restart | Create ResourceClaim + pod, before claim is fully prepared restart kubelet, verify pod eventually starts with GPU | None (GA) |
| 3.4 | Checkpoint integrity after kubelet restart | After kubelet restart with active GPU claims, verify `dra_manager_state` checkpoint file exists and is valid JSON with CRC32 | None (GA) |

**Validation approach:**
- `oc debug node/<gpu-node> -- systemctl restart kubelet` (requires privileged access)
- Monitor pod status transitions via `oc get pod -w`
- Inspect `/var/lib/kubelet/device-plugins/dra_manager_state` on the node
- Verify `oc get resourceslice` shows GPU resources after restart

### Category 4: Node Drain and Pod Lifecycle

**Why this matters:** Node drain with DRA-claimed pods exercises the
NodeUnprepareResources path on real hardware and validates that GPU resources are properly
released. This is a critical operational scenario not tested anywhere.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 4.1 | Node drain evicts DRA pods and releases claims | Deploy GPU pod, drain the node (`oc adm drain`), verify pod evicted, ResourceClaim released, GPU unprepared | None (GA) |
| 4.2 | Pod with PDB and GPU claim during drain | Deploy GPU pod with PodDisruptionBudget, drain node, verify PDB is respected before eviction | None (GA) |
| 4.3 | Uncordon restores GPU availability | After drain + uncordon, verify ResourceSlices re-published, new GPU pod can be scheduled | None (GA) |
| 4.4 | Graceful termination with GPU cleanup | Deploy GPU pod with `terminationGracePeriodSeconds: 60`, delete pod, verify NodeUnprepareResources called after graceful shutdown | None (GA) |

**Validation approach:**
- `oc adm drain <node> --ignore-daemonsets --delete-emptydir-data`
- `oc adm uncordon <node>`
- `oc get resourceclaim -o yaml` to check allocation status transitions
- `oc get resourceslice` to verify GPU re-availability
- Monitor kubelet logs for `NodeUnprepareResources` calls

### Category 5: PodResources API

**Why this matters:** The PodResources gRPC API (KEP-3695, GA in 1.36) exposes DRA device
information to monitoring agents (DCGM, GPU Feature Discovery). This is never tested in
any e2e suite despite being GA. OpenShift monitoring stack depends on this.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 5.1 | PodResources List returns DRA devices | Deploy GPU pod, call PodResources `List()` gRPC on kubelet socket, verify response includes DRA device info with correct driver name and CDI device IDs | None (GA) |
| 5.2 | PodResources Get returns DRA devices | Call PodResources `Get()` for specific GPU pod, verify DRA resource entries match allocated claim | None (GA) |
| 5.3 | PodResources after pod deletion | Delete GPU pod, call PodResources `List()`, verify DRA entries removed | None (GA) |

**Validation approach:**
- Deploy a privileged pod on the GPU node that mounts `/var/lib/kubelet/pod-resources/kubelet.sock`
- Use `grpcurl` or a small Go binary to call `v1.PodResourcesLister/List` and `v1.PodResourcesLister/Get`
- Parse the response for `dynamicResources` field entries
- Alternatively: use the existing `oc debug node` approach to access the kubelet socket

### Category 6: Device Health Status

**Why this matters:** ResourceHealthStatus (beta in 1.36) exposes device health in pod
status. This is tested upstream with mock driver health injection but never on real
hardware where actual GPU health transitions matter.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 6.1 | Healthy GPU reflected in pod status | Deploy GPU pod, verify `pod.status.containerStatuses[].allocatedResourcesStatus` shows `Healthy` for GPU device | `ResourceHealthStatus` (Beta, default on) |
| 6.2 | Device health fields populated | Verify health entry includes correct `name` (CDI device ID), `health` status, and `resourceClaimName` | `ResourceHealthStatus` |
| 6.3 | ResourceClaim status devices field | Verify `resourceclaim.status.devices` has populated entries with driver, pool, device names matching the allocation | `DRAResourceClaimDeviceStatus` (Beta) |

**Validation approach:**
- `oc get pod -o jsonpath='{.status.containerStatuses[0].allocatedResourcesStatus}'`
- `oc get resourceclaim <name> -o jsonpath='{.status.devices}'`
- These are read-only validations on running GPU pods

### Category 7: ResourceClaim Lifecycle on Real Hardware

**Why this matters:** Upstream tests ResourceClaim transitions with mock. These tests
validate the full lifecycle on real NVIDIA hardware, exercising the real driver's
prepare/unprepare implementation and verifying that claim status accurately reflects
hardware state.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 7.1 | Claim transitions: Pending -> Allocated -> Reserved -> Released | Create claim, deploy pod (triggers allocation + reservation), delete pod (triggers release), verify status transitions | None (GA) |
| 7.2 | Multi-container single claim | Pod with 2 containers both referencing same GPU claim, verify both containers see the GPU, verify single NodePrepareResources call | None (GA) |
| 7.3 | Claim reuse across pod recreations | Create persistent ResourceClaim, deploy pod, delete pod, deploy new pod with same claim, verify GPU re-prepared without re-allocation | None (GA) |
| 7.4 | ResourceClaim with CEL selector on GPU attributes | Create claim selecting GPU by attribute (e.g., `device.driver == 'gpu.nvidia.com'`), verify correct GPU allocated | None (GA) |
| 7.5 | Claim deletion with running pod | Attempt to delete a ResourceClaim while pod is running, verify finalizer prevents deletion until pod terminates | None (GA) |

**Validation approach:**
- `oc get resourceclaim -w` to monitor status transitions
- `nvidia-smi` inside both containers of multi-container pod
- `oc get resourceclaim -o yaml` for full status inspection

### Category 8: Admin Access on OpenShift

**Why this matters:** DRA admin access (KEP-5018, Beta in 1.34+) allows namespace-level
control over device access. This interacts with OpenShift RBAC/SCC model and namespace
management. The namespace label gating (`resource.kubernetes.io/admin-access`) is not
tested on OpenShift.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 8.1 | Admin access denied without namespace label | Create ResourceClaim with `adminAccess: true` in namespace WITHOUT the admin label, verify claim rejected / pod stays pending | None (Beta, default on) |
| 8.2 | Admin access granted with namespace label | Label namespace with `resource.kubernetes.io/admin-access: "true"`, create admin claim, verify GPU allocated | None (Beta, default on) |
| 8.3 | Admin claim coexists with normal claim | Both admin and non-admin pods on same node, verify both get GPU access, verify admin claim has expected additional access | None (Beta, default on) |

**Validation approach:**
- `oc label namespace <ns> resource.kubernetes.io/admin-access=true`
- `oc get resourceclaim -o yaml` to check allocation
- `oc describe pod` for events showing rejection

### Category 9: Device Taints and Tolerations

**Why this matters:** Device taints (KEP-5055) are GA in K8s 1.37 (OCP 5.0). Testing on
real hardware validates that taint/toleration logic works with the NVIDIA driver's
ResourceSlice publishing.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 9.1 | Pod stays pending with NoSchedule device taint | Apply NoSchedule taint to GPU device via DeviceTaintRule, deploy pod requesting that GPU, verify pod stays Pending | `DRADeviceTaints` + `DRADeviceTaintRules` |
| 9.2 | Pod tolerates device taint | Deploy pod with matching toleration, verify GPU allocated despite taint | `DRADeviceTaints` + `DRADeviceTaintRules` |
| 9.3 | NoExecute taint evicts running pod | Pod running with GPU, apply NoExecute DeviceTaintRule, verify pod evicted | `DRADeviceTaints` + `DRADeviceTaintRules` |

**Validation approach:**
- Create `DeviceTaintRule` objects targeting NVIDIA devices
- Monitor pod scheduling events for taint-related messages
- Verify eviction behavior with `oc get pod -w`

**Note:** These tests depend on the OCP version supporting DeviceTaintRule (K8s 1.37+/OCP 5.0+).
If the target cluster is < 1.37, these should be skipped.

### Category 10: DRA Metrics Validation

**Why this matters:** Kubelet exposes DRA-specific metrics that are critical for
monitoring GPU utilization at scale. These are never validated in e2e tests.

| # | Test | Description | Feature Gate |
|---|------|-------------|-------------|
| 10.1 | `dra_resource_claims_in_use` metric | Deploy GPU pod, scrape kubelet `/metrics`, verify `dra_resource_claims_in_use` gauge shows correct count per driver | None (GA) |
| 10.2 | `dra_operations_duration_seconds` metric | After GPU claim prepare/unprepare cycle, verify `dra_operations_duration_seconds` histogram has entries for `PrepareResources` and `UnprepareResources` methods | None (GA) |

**Validation approach:**
- `curl -sk https://<node-ip>:10250/metrics` (via debug pod or service account token)
- Parse Prometheus text format for `dra_` prefixed metrics
- Alternatively use OpenShift monitoring stack's Prometheus to query

---

## 4. Test Infrastructure Design

### 4.1 Location in openshift/origin

```
test/extended/node/dra/
  nvidia/
    nvidia_dra.go            -- existing basic tests
    node_lifecycle.go        -- NEW: Category 3, 4 (kubelet restart, drain)
    crio_cdi.go              -- NEW: Category 1 (CDI integration)
    security.go              -- NEW: Category 2 (SELinux, SCC)
    podresources.go          -- NEW: Category 5 (PodResources API)
    health_status.go         -- NEW: Category 6 (device health)
    claim_lifecycle.go       -- NEW: Category 7 (claim state transitions)
    admin_access.go          -- NEW: Category 8 (admin access)
    device_taints.go         -- NEW: Category 9 (device taints)
    metrics.go               -- NEW: Category 10 (kubelet metrics)
    gpu_validator.go         -- existing, may need extensions
    prerequisites_installer.go -- existing
    resource_builder.go      -- existing, extend for new test patterns
  common/
    node_ops.go              -- NEW: shared utilities for node operations
                                (kubelet restart, drain, debug pod, checkpoint read)
    podresources_client.go   -- NEW: PodResources gRPC client helper
    metrics_client.go        -- NEW: kubelet metrics scraper helper
```

### 4.2 Test Labels and Filtering

```go
// All new tests use these labels:
// [sig-node] - primary SIG ownership
// [Feature:NVIDIA-DRA] - requires NVIDIA GPU hardware
// [Suite:openshift/nvidia-dra] - test suite grouping
// [Serial] - cannot run in parallel (node-level operations)

// Category-specific additional labels:
// [Disruptive] - tests that restart kubelet or drain nodes (Cat 3, 4)
// [Slow] - tests with extended wait times (Cat 3, 4)
```

### 4.3 Ginkgo Test Structure

```go
var _ = g.Describe("[sig-node][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra]", func() {

    // Non-disruptive tests (can run on shared clusters)
    g.Context("CRI-O CDI Integration", func() { ... })
    g.Context("SELinux and Security", func() { ... })
    g.Context("PodResources API", func() { ... })
    g.Context("Device Health Status", func() { ... })
    g.Context("ResourceClaim Lifecycle", func() { ... })
    g.Context("Admin Access", func() { ... })
    g.Context("Kubelet Metrics", func() { ... })

    // Disruptive tests (require dedicated node or cluster)
    g.Context("[Disruptive] Kubelet Restart Recovery", func() { ... })
    g.Context("[Disruptive] Node Drain", func() { ... })
    g.Context("[Disruptive] Device Taints", func() { ... })
})
```

### 4.4 Node Operations Helper

A shared utility for tests that need to perform node-level operations:

```go
// NodeOps provides methods for node-level operations on GPU nodes
type NodeOps struct {
    client    kubernetes.Interface
    nodeName  string
}

// RestartKubelet restarts kubelet via systemctl and waits for ready
func (n *NodeOps) RestartKubelet(ctx context.Context) error

// DrainNode cordons and drains the node
func (n *NodeOps) DrainNode(ctx context.Context) error

// UncordonNode uncordons the node
func (n *NodeOps) UncordonNode(ctx context.Context) error

// ReadCheckpoint reads and returns the DRA manager checkpoint
func (n *NodeOps) ReadCheckpoint(ctx context.Context) (*DRACheckpoint, error)

// GetSELinuxAVCDenials returns recent AVC denials from audit log
func (n *NodeOps) GetSELinuxAVCDenials(ctx context.Context, since time.Time) ([]string, error)

// InspectCDISpecs lists CDI spec files on the node
func (n *NodeOps) InspectCDISpecs(ctx context.Context) ([]CDISpec, error)

// ScrapeKubeletMetrics returns kubelet Prometheus metrics
func (n *NodeOps) ScrapeKubeletMetrics(ctx context.Context) (map[string]float64, error)
```

These operations use `oc debug node/<name>` internally with appropriate privilege.

### 4.5 Prerequisites

| Prerequisite | Details |
|-------------|---------|
| NVIDIA GPU node(s) | At least 1 worker node with NVIDIA GPU |
| GPU Operator installed | NVIDIA GPU Operator with driver loaded |
| NVIDIA DRA driver installed | `nvidia-dra-driver` Helm chart deployed |
| ResourceSlices present | `oc get resourceslice` shows GPU devices |
| DeviceClass available | `gpu.nvidia.com` DeviceClass exists |
| Feature gates (for Cat 6,9) | `ResourceHealthStatus`, `DRADeviceTaints` enabled if testing those |

The existing `prerequisites_installer.go` handles driver setup. Tests should skip
gracefully if prerequisites are not met (e.g., no GPU node, driver not installed).

---

## 5. Priority and Phasing

### Phase 1: Core Node Validation (High Priority)

These tests provide the most unique value - they validate OpenShift-specific node
behavior that no other test suite covers.

| Category | Tests | Effort | Risk Coverage |
|----------|-------|--------|--------------|
| 1. CRI-O CDI Integration | 1.1, 1.2, 1.3 | Low | CRI-O CDI regression |
| 2. SELinux and Security | 2.1, 2.2, 2.3, 2.4 | Medium | Security policy breaks DRA |
| 7. ResourceClaim Lifecycle | 7.1, 7.2, 7.4 | Low | Real HW claim state machine |

**Estimated effort:** 2-3 days

### Phase 2: Operational Resilience (High Priority)

These tests validate that DRA survives real operational scenarios on OpenShift.

| Category | Tests | Effort | Risk Coverage |
|----------|-------|--------|--------------|
| 3. Kubelet Restart | 3.1, 3.2, 3.3 | Medium | GPU state lost on restart |
| 4. Node Drain | 4.1, 4.2, 4.3 | Medium | Drain fails with DRA pods |

**Estimated effort:** 2-3 days

### Phase 3: Observability and API (Medium Priority)

These tests validate monitoring and API surfaces.

| Category | Tests | Effort | Risk Coverage |
|----------|-------|--------|--------------|
| 5. PodResources API | 5.1, 5.2, 5.3 | Medium | Monitoring stack blind to GPUs |
| 6. Device Health Status | 6.1, 6.2, 6.3 | Low | Health not propagated to pod status |
| 10. Kubelet Metrics | 10.1, 10.2 | Low | Metrics missing/wrong |

**Estimated effort:** 2-3 days

### Phase 4: Advanced Features (Lower Priority, version-dependent)

| Category | Tests | Effort | Risk Coverage |
|----------|-------|--------|--------------|
| 8. Admin Access | 8.1, 8.2, 8.3 | Low | Privilege escalation via DRA |
| 9. Device Taints | 9.1, 9.2, 9.3 | Medium | Taint/eviction breaks on real HW |
| 7. Claim Lifecycle (rest) | 7.3, 7.5 | Low | Claim reuse/finalizer edge cases |

**Estimated effort:** 2-3 days

---

## 6. Non-Duplication Matrix

This table maps each proposed test to what already exists and confirms no duplication.

| Proposed Test | Upstream `test/e2e/dra/` | Upstream `test/e2e_node/` | OCP `nvidia_dra.go` | Why Ours is Different |
|--------------|:---:|:---:|:---:|------|
| 1.1 CDI via CRI-O | - | - | Partial (just nvidia-smi) | Validates CDI spec files + CRI-O processing, not just GPU visibility |
| 1.2 CDI spec files | - | - | - | Node-level CDI directory inspection |
| 2.1 SELinux | - | - | - | **OpenShift-only concern** |
| 2.2 Restricted SCC | - | - | - | **OpenShift-only concern** |
| 3.1 Kubelet restart | - | Yes (mock) | - | Real HW driver re-registration, real checkpoint |
| 3.2 Driver re-register | - | Yes (mock) | - | Real NVIDIA plugin socket discovery |
| 4.1 Node drain | - | - | - | **Not tested anywhere** |
| 5.1 PodResources API | - | - | - | **Not tested in any e2e suite** (GA feature) |
| 6.1 Health in pod status | Yes (mock health) | Yes (mock health) | - | Real GPU health values, not injected |
| 7.1 Claim transitions | Yes (mock) | - | - | Real HW state transitions |
| 7.2 Multi-container claim | Yes (mock) | - | - | Real GPU sharing across containers |
| 8.1 Admin access denied | Yes (mock) | - | - | OpenShift namespace management + SCC interaction |
| 9.1 Device taint NoSchedule | Yes (mock) | - | - | Real NVIDIA DeviceTaintRule |
| 10.1 DRA metrics | - | Yes (mock, partial) | - | Real metric values with real driver |

---

## 7. Test Environment Requirements

### Minimum Hardware
- 1 OpenShift cluster (4.21+ / K8s 1.34+)
- At least 1 worker node with NVIDIA GPU (any generation: T4, A100, H100, Blackwell)
- GPU Operator + NVIDIA DRA driver installed and functional

### CI Integration Options

| Option | Pros | Cons |
|--------|------|------|
| **NVIDIA DGX CI** | Real GPUs, NVIDIA-maintained | Access restrictions, scheduling |
| **AWS p-type instances** | On-demand, scriptable | Cost, limited GPU models |
| **Internal GPU lab** | Full control, diverse GPUs | Maintenance overhead |
| **ClusterBot with GPU profile** | Standard OCP CI | Limited GPU availability |

### Recommended Approach
Run disruptive tests (Cat 3, 4) on a dedicated SNO or single-worker cluster to avoid
impacting other workloads. Non-disruptive tests (Cat 1, 2, 5, 6, 7, 8, 10) can run on
shared GPU clusters.

---

## 8. Open Questions

1. **Target OCP version:** Should tests target OCP 4.21 (K8s 1.34, DRA GA) or also
   support OCP 5.0 (K8s 1.35+) with newer features (device taints GA, extended resources)?

2. **CI pipeline:** Which CI system will run these tests? Prow with GPU-equipped nodes?
   NVIDIA's internal CI? Both?

3. **Disruption tolerance:** For kubelet restart and node drain tests, is it acceptable
   to run on a dedicated cluster, or do we need to support shared multi-tenant GPU
   clusters?

4. **PodResources API client:** Should we build a small Go gRPC client for PodResources
   API testing, or use `grpcurl` via exec? The Go client is more robust but adds code.

5. **Device taints scope:** The NVIDIA DRA driver may not yet support reporting device
   taints. Should we test DeviceTaintRule (admin-applied) only, or also driver-reported
   taints?

---

## 9. References

- [KEP-4381: DRA Structured Parameters](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4381-dra-structured-parameters)
- [KEP-5018: DRA Admin Access](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/5018-dra-admin-access)
- [KEP-5055: Device Taints](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/5055-dra-device-taints)
- [KEP-4680: Resource Health Status](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4680-dra-health-status)
- [KEP-3695: PodResources API for DRA](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
- [Kubelet DRA Manager](https://github.com/kubernetes/kubernetes/tree/master/pkg/kubelet/cm/dra)
- [Upstream DRA E2E Tests](https://github.com/kubernetes/kubernetes/tree/master/test/e2e/dra)
- [Upstream Node DRA E2E Tests](https://github.com/kubernetes/kubernetes/tree/master/test/e2e_node/dra_test.go)
- [OpenShift Origin DRA Tests](https://github.com/openshift/origin/tree/master/test/extended/node/dra)
- [CDI Specification](https://github.com/cncf-tags/container-device-interface)
