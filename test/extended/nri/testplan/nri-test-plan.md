**Plan Status:** Draft
**Feature:** [OCPNODE-4853](https://redhat.atlassian.net/browse/OCPNODE-4853) — NRI test suite: add `openshift/nri` extended tests and test plan
**Testing Epic:** [OCPNODE-4852](https://redhat.atlassian.net/browse/OCPNODE-4852) — Test coverage for NRI Plugin Support in CRI-O (GA 5.1)
**Component:** Node / CRI-O (neither OCPNODE ticket sets a component; scope taken from the epic)
**Snapshot pin:** PR [#31844](https://github.com/openshift/origin/pull/31844), based on `openshift/origin` `upstream/main` `895e3d93a0`; behavior sampled on OCP 4.22.21 with CRI-O 1.35.10 on RHCOS 9.8

# Test Plan: NRI plugin integration on OpenShift (`openshift/nri`)

**Generated with:** `openshift-developer:generate-test-plan` ([openshift-eng/ai-helpers](https://github.com/openshift-eng/ai-helpers)). Inputs were OCPNODE-4853, its testing epic OCPNODE-4852, and PR [#31844](https://github.com/openshift/origin/pull/31844).

---

## 1. Test Plan Identifier

| Field | Value |
|---|---|
| **Identifier** | OCPNODE-4853-test-plan |
| **Revision** | 2.0 |
| **Date** | 2026-11-03 |
| **Author** | Node Team |
| **PR classification** | Test-only. PR #31844 changes only test code, test fixtures, a test-only image and docs, so this is a test-code-as-deliverable plan: it describes what the new tests cover, not tests of a product change. |

---

## 2. Introduction

### 2.1 Purpose

CRI-O serves the [NRI](https://github.com/containerd/nri) socket, and OCP 4.17 and later ship CRI-O 1.30 or newer, where `enable_nri` defaults to true. Nothing downstream exercised the CRI-O to plugin to workload path, so a regression that disables NRI, breaks plugin registration, or drops container adjustments would only show up as a customer plugin not working (for example [kai-scheduler/gpu-fractioning](https://github.com/kai-scheduler/gpu-fractioning#prerequisites)).

This plan describes the coverage of the `openshift/nri` extended suite and the test plugin it deploys, tracks which scenarios are already automated in PR #31844, and lists the gaps that remain.

### 2.2 Scope

In scope for the test code:

- CRI-O exposing the NRI socket on an RHCOS node
- NRI plugin registration and synchronization with a running CRI-O
- Pod and container lifecycle events delivered to a plugin
- Container adjustments applied at create time: environment, OCI annotations, bind mounts (read-only and read-write), device nodes, cpuset
- NRI failure policy (a plugin error fails container creation)
- Opt-in gating and namespace confinement of the privileged plugin
- SCC/PodSecurity admission confinement of the plugin pod
- Two plugins at different indices, including invocation order
- Pod churn with a plugin registered
- Plugin downtime: pod creation keeps working, and the plugin re-registers and synchronizes existing containers

Out of scope: the plugin implementations themselves (gpu-fractioning and other operators), CDI, runtimes other than CRI-O, and upgrades.

### 2.3 Background — current behavior relevant to testing

- NRI is enabled by default in CRI-O 1.30+; the suite does not toggle it. When the plugin cannot connect, the suite skips instead of failing (detected from the plugin connection error).
- CRI-O's own [test/nri.bats](https://github.com/cri-o/cri-o/blob/main/test/nri.bats) starts a throwaway CRI-O and drives it with `crictl`. It cannot prove that an OCP node with MCO-managed CRI-O config, SELinux enforcing, PodSecurity admission and a payload image behaves the same. The downstream suite drives everything through the Kubernetes API: the plugin is a DaemonSet and the workloads are pods.
- The test plugin in `test/extended/nri/plugin` records NRI events as JSON lines on stdout; the suite reads them from the pod logs and adjusts only pods that carry `nri.openshift.io/test: "true"`.
- The plugin ships as its own payload image, `images/nri-test-plugin/Dockerfile.rhel`, so the release tests image is untouched.

### 2.4 References

| Ref | Document |
|---|---|
| R1 | [OCPNODE-4853](https://redhat.atlassian.net/browse/OCPNODE-4853) |
| R2 | [OCPNODE-4852](https://redhat.atlassian.net/browse/OCPNODE-4852) — testing epic |
| R3 | [OCPSTRAT-3973](https://redhat.atlassian.net/browse/OCPSTRAT-3973) — NRI Plugin Support in CRI-O: GA 5.1 |
| R4 | [openshift/origin PR #31844](https://github.com/openshift/origin/pull/31844) |
| R5 | `test/extended/nri/nri.go` — the suite (11 ordered specs) |
| R6 | `test/extended/nri/nri_helpers.go` — namespace, plugin, pod and log helpers |
| R7 | `test/extended/nri/plugin` — the test plugin and parser unit tests |
| R8 | `images/nri-test-plugin/Dockerfile.rhel` — the plugin payload image |
| R9 | CRI-O [test/nri.bats](https://github.com/cri-o/cri-o/blob/main/test/nri.bats) — upstream model |
| R10 | CRI-O [crio.conf.5 `[crio.nri]`](https://github.com/cri-o/cri-o/blob/main/docs/crio.conf.5.md#crionri-table) — `enable_nri` default |
| R11 | [containerd/nri](https://github.com/containerd/nri) — NRI API and plugin ordering |
| R12 | [gpu-fractioning prerequisites](https://github.com/kai-scheduler/gpu-fractioning#prerequisites) |

---

## 3. Test Items

| Component group | Component | Description |
|---|---|---|
| Container runtime | CRI-O | NRI server, plugin registration, adjustment application, failure policy |
| Node OS | RHCOS | NRI socket directory, SELinux `container_var_run_t`, cgroup v2 |
| NRI API | `github.com/containerd/nri` | Plugin stub and adjustment types used by the test plugin |
| Test plugin | `test/extended/nri/plugin` | Records NRI requests as JSON lines; applies annotation-driven adjustments |
| Payload image | `images/nri-test-plugin` | Ships the plugin binary as the `nri-test-plugin` release image |
| Test suite | `test/extended/nri` | Ginkgo suite `openshift/nri`, registered in `pkg/testsuites` and `test/extended/include.go` |
| Admission | PodSecurity admission, SCC | The plugin pod is privileged; the suite relaxes PSA and binds the `privileged` SCC |

---

## 4. Features to Be Tested

- NRI socket served by CRI-O and a plugin completing `Configure` and `Synchronize`
- Pod events (run, stop, remove) and container events (create, start, stop)
- Environment variable injection
- OCI annotation injection
- Bind mount injection, including read-only enforcement and writable binds
- Device node injection and usability
- cpuset adjustment
- Plugin error surfacing as a container creation failure
- Opt-in gating (`nri.openshift.io/test: "true"`)
- Namespace confinement of the privileged plugin
- SCC/PodSecurity admission confinement
- Two plugins registered at different indices, invocation order and merged adjustments
- Pod churn with a plugin registered
- Pod creation while the plugin is down, and resynchronization after it returns
- Suite skip behavior when NRI is disabled
- Single node clusters (control-plane fallback)

---

## 5. Features Not to Be Tested

| Feature | Reason |
|---|---|
| gpu-fractioning itself and other production plugins | Require NVIDIA GPUs, the GPU Operator and a CUDA 13.4 driver; the suite covers the NRI primitives they depend on |
| CDI device injection | Separate CRI-O path; gpu-fractioning gets its devices from the GPU Operator/CDI, not NRI |
| containerd | OpenShift nodes run CRI-O |
| MicroShift | Out of Node team scope; the suite skips it |
| Plugin ordering beyond two indices, plugin-to-plugin conflicts | TC-FUNC-005 covers two plugins at different indices; deeper ordering is not needed to prove the OCP path |
| Enabling/disabling NRI via MachineConfig/ContainerRuntimeConfig | Toggling NRI is MCO coverage; TC-DEPLOY-001 only verifies the suite skips when NRI is disabled |
| Runtime configuration injection (`nri.runtime.patchConfig`) | Upstream nri-plugins charts patch containerd config; OCP ships NRI enabled |
| Windows nodes, user namespaces | Not supported by the path under test |
| FIPS-specific cryptography | NRI does not add cryptography; FIPS is a topology variation (TC-NFR-003) |

### 5.1 Criteria Assessment

Acceptance criteria from OCPNODE-4853 and whether they are testable:

| Acceptance criterion | Testable? | Mapping |
|---|---|---|
| Suite merged in `openshift/origin` and green on a payload | Process criterion, not a test scenario | Verification evidence below; exit criteria |
| `nri-test-plugin` `images:` entry added to `openshift/release` | Process criterion | Testing task |
| CI job for `openshift/nri` added (periodic, informed pre-merge scope) | Process criterion | Testing task |
| SNO and multi-node runs green | Testable | TC-DEPLOY-003 |
| Polarion/OTP IDs added to the specs | Process criterion | Testing task |
| HyperShift run verified (epic acceptance criterion) | Testable | TC-DEPLOY-004 |
| Remaining test-plan gaps tracked | Process criterion | Appendix: Deferred Scenarios |

---

## 6. Approach

### 6.1 Strategy

Test-code-as-deliverable: the PR adds tests rather than product behavior, so the plan documents what the new tests cover, what is already covered upstream, and what remains uncovered. The suite is one ordered Ginkgo container on a real cluster; unit tests for the plugin's parsers run in the plugin image build.

### 6.2 Testing levels and CI tiers

| Tier | Use | Cost reasoning |
|---|---|---|
| `unit` | Plugin annotation parsers | No cluster; runs in the image build |
| `e2e-serial` | The ordered `openshift/nri` suite | Specs share one plugin and one namespace and must run in order, so the suite sets `Parallelism: 1` |
| `e2e-periodic` | Proposed disruptive, upgrade, architecture, FIPS and GPU scenarios | Long or hardware-dependent; not appropriate for every PR |

### 6.3 CI placement matrix

| Repository | Pre-merge | Post-merge |
|---|---|---|
| `openshift/origin` (`test/extended/nri`, `images/nri-test-plugin`) | Unit tests run by the plugin image build (TC-UNIT-001); suite runs when CRI-O, MCO or the suite changes | `openshift/nri` job runs all `e2e-serial` scenarios; periodic job runs the proposed disruptive/upgrade scenarios |
| `openshift/release` | Job configuration change when the suite or pre-merge scope changes (not test code) | Periodic and informing job configuration |

### 6.4 Existing upstream coverage (informational)

CRI-O's `test/nri.bats` already exercises registration, synchronization, pod and container events, environment, annotation, mount and device injection, and cpuset/memset adjustment against a throwaway CRI-O. It does not cover an OCP node with MCO-managed configuration, SELinux enforcing, PodSecurity admission, a payload image, or two plugins in one suite. This plan therefore focuses downstream effort on the OCP integration and the multi-plugin, confinement and churn cases.

### 6.5 Feature gates

None. NRI has no OpenShift feature gate; it is enabled by default since CRI-O 1.30 (OCP 4.17).

### 6.6 OCP version applicability

| OCP version | CRI-O | NRI default | Tests that run | Tests that skip |
|---|---|---|---|---|
| 4.17+ | 1.30+ | on | All implemented scenarios | None |
| ≤ 4.16 | ≤ 1.29 | off | None | TC-FUNC-001 and the rest of the suite skip when the plugin cannot connect |

---

## 7. Item Pass/Fail Criteria

1. All eleven suite specs pass in one ordered run (`openshift-tests run openshift/nri`) on a supported payload.
2. Three consecutive runs are green with no retries; a flake in any scenario is a failure for the purpose of this plan.
3. The plugin parser unit tests pass in the payload image build (TC-UNIT-001).
4. TC-FUNC-003 proves the gpu-fractioning prerequisites: environment and bind mount injected by a DaemonSet plugin at container-create time, with the read-only mount enforcing `EROFS`.
5. TC-SEC-001 and TC-SEC-002 keep the failure surface honest: a pod outside the plugin's namespace is untouched, and a privileged plugin pod is rejected without the suite's SCC/PSA setup.
6. TC-FUNC-006 proves pod creation works while no plugin is registered.
7. The suite leaves no state: plugin DaemonSet and namespace deleted, no host paths written.
8. The suite skips (not fails) when NRI is disabled, the node has a single online CPU, or the cluster is MicroShift. A skip on a payload that should have NRI is a failure of this plan.
9. No monitor failure is attributable to the suite. For example, the suite must not create force-deleted pods in a platform namespace.

### 7.1 Pre-existing verification evidence

- PR #31844 [run history]: the pre-redesign suite passed 11/11 in one ordered run on OCP 4.22.21 / CRI-O 1.35.10 (single worker node).
- The event-log redesign (plugin events read from the pod logs, no `oc debug`, no host paths) was built and unit-tested but not yet run end to end; the first run of that redesign failed in `BeforeAll` because the CLI log reader returned `ContainerCreating` for the polling window, and the reader was moved to the `pods/log` API afterwards. Re-run required before sign-off.
- Parser unit tests pass locally; real plugin log lines from the failed run parsed to 111 events, including `configure`.

---

## 8. Suspension and Resumption Criteria

### Suspension

- The selected node has no NRI socket / the plugin cannot register. The suite skips; repeated skips suspend the job rather than reporting green.
- The payload image `nri-test-plugin` is missing and `NRI_TEST_PLUGIN_IMAGE` is unset.
- The test namespace cannot get the `privileged` SCC or PSA labels.
- The `pods/log` subresource is unavailable for the plugin pods.

### Resumption

- NRI is confirmed enabled on the node, the plugin image is resolvable, and a single `run-test` of TC-FUNC-001 passes.

---

## 9. Test Deliverables

- `test/extended/nri/nri.go`, `nri_helpers.go` — the suite
- `test/extended/nri/plugin/` — the plugin, parser unit tests and local build
- `images/nri-test-plugin/Dockerfile.rhel` — the payload image
- `pkg/testsuites/standard_suites.go`, `test/extended/include.go` — suite registration
- CI job for `openshift/nri` and its artifacts
- Polarion/`[OTP]` IDs for the eleven automated specs
- This plan and its companion test case catalogue (`nri-test-cases.md`)

---

## 10. Testing Tasks

1. Re-run the full suite after the event-log redesign and attach results (blocking before merge).
2. Add the `nri-test-plugin` `images:` entry in the `openshift/release` ci-operator config for the branch.
3. Add the `openshift/nri` CI job: periodic, informing first; pre-merge when CRI-O, MCO or the suite changes.
4. Add Polarion/`[OTP]` IDs to the eleven specs for QE reporting.
5. Add the test cases to `openshift-eng/ci-test-mapping` so Component Readiness can track them (new suite, new tests).
6. Run SNO and multi-node clusters (TC-DEPLOY-003) and HyperShift (TC-DEPLOY-004).
7. Track the deferred scenarios in the appendix.

---

## 11. Environmental Needs

### 11.1 Cluster configurations

| Item | Requirement |
|---|---|
| OCP version | 4.17 or later (CRI-O 1.30+ defaults NRI on); validated on 4.22.21 with CRI-O 1.35.10 |
| Node OS | RHCOS, SELinux enforcing, cgroup v2 |
| Runtime | CRI-O with NRI enabled |
| Topology | Multi-node validated on a single worker; SNO and multi-node runs still required |
| Architecture | x86_64 validated; aarch64 proposed (TC-NFR-002) |
| FIPS | Proposed (TC-NFR-003) |
| Platform | AWS validated; bare metal proposed |
| Admission | Namespace with PSA `enforce=privileged` and the `privileged` SCC bound to `default` |
| Images | In-cluster `tools` imagestream for test pods; `nri-test-plugin` payload image |
| Access | Cluster admin (SCC binding, namespace creation); no node debug access needed |

### 11.2 SNO vs multi-node

The suite picks a Ready worker, falls back to a non-control-plane node, then to a control-plane-only node (SNO), and pins the plugin DaemonSet to it. The node fallback is implemented; SNO and multi-node runs are not yet part of the evidence.

### 11.3 Test data

| Item | Value |
|---|---|
| Opt-in annotation | `nri.openshift.io/test: "true"` |
| Plugin event source | Plugin pod logs, JSON lines |
| Mount sources | `/proc/version` (read-only regular file) and `/dev/null` (read-write character device), both container-readable without relabeling |
| Device | `/dev/nri-null`, char `1:3`, mode `0666` |
| cpuset | Last CPU from `node.Status.Capacity.Cpu()` |

### 11.4 Test environment strategy

| Practice | Detail |
|---|---|
| CI job | `openshift-tests run openshift/nri` on a payload containing the `nri-test-plugin` image |
| Job class | Informing first; promote to blocking once stable |
| Cadence | Nightly; pre-merge when CRI-O, MCO or the suite changes |
| SNO / multi-node | Same suite; only the cluster topology differs |
| Upgrade | TC-UPGRD-001 on a z-stream job |
| Manual | `NRI_TEST_PLUGIN_IMAGE=<image> openshift-tests run openshift/nri`, image built with `make -C test/extended/nri/plugin image` |

---

## 12. Responsibilities

| Role | Responsibility |
|---|---|
| Node team | Owns the suite, the plugin image and the test plan |
| CRI-O maintainers | NRI runtime behavior and review of the test expectations |
| CI / TRT | Job configuration and classification |

---

## 13. Staffing and Training Needs

- Ginkgo extended test structure in `openshift/origin`
- CRI-O NRI API and SELinux/cgroup behavior on RHCOS
- Understanding of PodSecurity admission and SCC for privileged node-level workloads

---

## 14. Schedule

TBD; GA target 5.1 per OCPSTRAT-3973. No test-specific milestones are defined yet.

---

## 15. Risks and Contingencies

| Risk | Impact | Contingency |
|---|---|---|
| Event-log redesign not yet run end to end | Merge blocker | Re-run the suite on a cluster before merge |
| The `nri-test-plugin` image is not in the payload | Suite cannot run in CI | `NRI_TEST_PLUGIN_IMAGE` override; land the `openshift/release` entry |
| The plugin pod is privileged and mints host mounts/devices on request | Node exposure if the plugin outlives the test | Only adjusts pods in its own namespace; namespace deletion removes the DaemonSet; annotation input documented as trusted |
| NRI-injected mounts keep the source SELinux label | Plugin-owned paths need relabeling, which the suite does not cover | Suite mounts container-readable host files; relabeling requirement documented |
| Single worker node validated only | Multi-node and SNO paths unverified | TC-DEPLOY-003 before sign-off |
| Upstream CRI-O changes the `enable_nri` default or socket path | Suite skips or fails for the wrong reason | TC-FUNC-001 fails loudly; update the check when the runtime changes |
| No Polarion/OTP IDs or conformance gate | Regressions go unnoticed and QE cannot report | Testing tasks 3 and 4 |
| Plugin parser accepts negative major/minor | Invalid input reaches runc | TC-UNIT-002 (deferred) |

---

## 16. Approvals

| Role | Name | Status |
|---|---|---|
| Feature owner | Peter Hunt | Pending |
| Node team reviewer | TBD | Pending |

---

## 17. Test Scenarios

Detailed steps and expected output are in the companion [nri-test-cases.md](nri-test-cases.md). All implemented scenarios target `openshift/origin/test/extended/nri` and apply to OCP 4.17+.

### 17.1 Implemented in PR #31844

| ID | Title | Tier | Target file | Coverage | Implementation | Traceability |
|---|---|---|---|---|---|---|
| TC-UNIT-001 | Annotation parser unit tests | `unit` | `test/extended/nri/plugin/main_test.go` | Covered | Implemented in #31844 | Parser correctness |
| TC-FUNC-001 | Plugin registers and synchronizes with CRI-O | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | OCPNODE-4852 scope |
| TC-FUNC-002 | Pod and container lifecycle events reach the plugin | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | OCPNODE-4852 scope |
| TC-FUNC-003 | Environment, annotations, read-only mount and cpuset adjustments | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | R12 prerequisites |
| TC-FUNC-004 | Device node injection and usability | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | OCPNODE-4852 scope |
| TC-FUNC-005 | Two plugins at different indices, in order, with merged adjustments | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | R11 plugin ordering |
| TC-FUNC-006 | Pod creation while the plugin is down, then resynchronization | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | NRI failure policy |
| TC-NEG-001 | A plugin error fails container creation and is logged | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | NRI failure policy |
| TC-NEG-002 | Pods without the opt-in annotation are not adjusted | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | Plugin gating |
| TC-SEC-001 | Pods outside the plugin's namespace are not adjusted | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | Privilege confinement |
| TC-SEC-002 | A privileged plugin pod is rejected without SCC/PSA privileges | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | Privilege confinement |
| TC-NFR-001 | 10-pod churn with a plugin registered | `e2e-serial` | `test/extended/nri/nri.go` | Covered | Implemented in #31844 | Scale sanity |

### 17.2 Proposed and deferred

| ID | Title | Tier | Target file | Coverage | Implementation | Traceability |
|---|---|---|---|---|---|---|
| TC-UNIT-002 | Reject negative device major/minor | `unit` | `test/extended/nri/plugin/main_test.go` | Not covered | To be automated | Parser robustness |
| TC-NEG-003 | Plugin killed during `CreateContainer`; request timeout enforced | `e2e-periodic` | `test/extended/nri/nri.go` | Not covered | To be automated | NRI failure policy |
| TC-DEPLOY-001 | NRI disabled: suite skips with the documented message | `e2e-periodic` | `test/extended/nri/nri.go` | Not covered | To be automated | MCO/CRI-O config |
| TC-DEPLOY-002 | CRI-O restart and node reboot: plugin re-registers, adjustments still apply | `e2e-periodic` | `test/extended/nri/nri.go` | Not covered | To be automated | Node lifecycle |
| TC-DEPLOY-003 | Suite green on SNO and multi-node | `e2e-periodic` | CI job | Partial (single worker only) | To be automated | OCPNODE-4853 AC |
| TC-DEPLOY-004 | Suite green on HyperShift | `e2e-periodic` | CI job | Not covered | To be automated | OCPNODE-4852 AC |
| TC-UPGRD-001 | z-stream upgrade keeps NRI and the plugin image working | `e2e-periodic` | CI job | Not covered | To be automated | Upgrade contract |
| TC-NFR-002 | Suite on aarch64 | `e2e-periodic` | CI job | Not covered | To be automated | Architecture |
| TC-NFR-003 | Suite on a FIPS-enabled cluster | `e2e-periodic` | CI job | Not covered | To be automated | Compliance |
| TC-NFR-004 | gpu-fractioning on a GPU node | `e2e-periodic` | manual | Not covered | Manual only (GPU + r615 driver) | R12 |
| TC-OPS-001 | CRI-O journal records NRI plugin errors | `e2e-periodic` | `test/extended/nri/nri.go` | Partial (plugin log only) | To be automated | Observability |

---

## 18. Existing Test Coverage and Gap Report

| Proposed scenario | Existing coverage | Status |
|---|---|---|
| TC-FUNC-001 to TC-FUNC-004, TC-NEG-001/002 | CRI-O `test/nri.bats` covers the same events and adjustments against a throwaway CRI-O | Partial: no OCP node, SELinux, admission, payload image |
| TC-FUNC-005 | CRI-O bats does not test two plugins together | Not covered upstream |
| TC-SEC-001, TC-SEC-002 | No upstream equivalent | Not covered |
| TC-NFR-001 | No upstream equivalent | Not covered |
| TC-UNIT-001 | CRI-O bats has no unit tests for the test plugin | Covered by this PR |

Before PR #31844, `openshift/origin` had no NRI tests. This PR is the first downstream coverage.

---

## 19. Test Adequacy Assessment

What the test code now covers:

- The full NRI lifecycle from CRI-O to a plugin to a pod, through the Kubernetes API
- The adjustment primitives production plugins use: environment, annotations, bind mounts (with `ro` enforcement), devices, cpuset
- Failure policy, opt-in gating, namespace and admission confinement, two-plugin ordering, churn, and plugin downtime
- Parser edge cases for env/annotations, mounts and device modes in unit tests

Gaps in the test code:

- Only one node is exercised per run, and only a single worker node has been run; SNO and multi-node are deferred (TC-DEPLOY-003)
- HyperShift has not been run (TC-DEPLOY-004)
- No real plugin (for example gpu-fractioning) is used in CI; the suite's plugin is a stand-in (TC-NFR-004 is manual)
- Plugin-owned path relabeling is not covered; the suite mounts host files containers already read (`/proc/version`, `/dev/null`)
- The plugin parser still accepts negative device major/minor (TC-UNIT-002)
- The event-log redesign is not yet re-run on a cluster; the last full green run predates it
- No Polarion/OTP IDs, so QE reporting cannot track the specs

---

## Appendix: Deferred Scenarios

| ID | Scenario | Effort | Value | Why deferred | Status |
|---|---|---|---|---|---|
| TC-UNIT-002 | Negative major/minor parser test | Low | Medium | Not needed for the current suite inputs | Not started |
| TC-NEG-003 | Plugin request timeout | Medium | Medium | Needs a controllable hang and risks flakiness | Not started |
| TC-DEPLOY-001/002 | NRI disabled, CRI-O restart/reboot | Medium | High | Disruptive; belongs in a dedicated job | Not started |
| TC-DEPLOY-003/004 | SNO, multi-node, HyperShift | Medium | High | Needs CI capacity and cluster variants | Not started |
| TC-UPGRD-001 | z-stream upgrade | Medium | Medium | Upgrade job, not this suite | Not started |
| TC-NFR-002/003 | aarch64, FIPS | Medium | Medium | Needs CI variants | Not started |
| TC-NFR-004 | gpu-fractioning on GPU | High | High | Needs GPU hardware and driver r615 | Blocked |
| TC-OPS-001 | CRI-O journal observability | Low | Low | Plugin log already asserted | Not started |

---

## Abbreviations

| Abbreviation | Full form |
|---|---|
| CI | Continuous Integration |
| CDI | Container Device Interface |
| CRI-O | Container Runtime Interface for OCP |
| E2E | End-to-End |
| FIPS | Federal Information Processing Standards |
| GPU | Graphics Processing Unit |
| IEEE 829 | IEEE Standard for Software Test Documentation |
| MCO | Machine Config Operator |
| NRI | Node Resource Interface |
| OCP | OpenShift Container Platform |
| PR | Pull Request |
| PSA | Pod Security Admission |
| RHCOS | Red Hat Enterprise Linux CoreOS |
| SCC | Security Context Constraints |
| SNO | Single Node OpenShift |
| TC | Test Case |

---

## Template Conformance Report (vs IEEE 829)

The repository has an existing plan convention (`test/extended/node/testplan/kerberos-test-plan.md`), which this plan mirrors. This table records the delta against the IEEE 829 checklist used by the generating skill.

| IEEE 829 Section | This plan | Status |
|---|---|---|
| Test Plan Identifier | §1 | ✅ Covered |
| Introduction (Purpose + Scope) | §2.1, §2.2 | ✅ Covered |
| References | §2.4 | ✅ Covered |
| Test Items | §3 | ✅ Covered |
| Features to Be Tested | §4 | ✅ Covered |
| Features Not to Be Tested | §5, §5.1 | ✅ Covered |
| Approach / Test Strategy | §6 | ✅ Covered |
| Pass/Fail Criteria | §7 | ✅ Covered |
| Suspension Criteria | §8 | ✅ Covered |
| Test Deliverables | §9 | ✅ Covered |
| Testing Tasks | §10 | ✅ Covered |
| Environmental Needs | §11 | ✅ Covered |
| Responsibilities | §12 | ✅ Added by skill — not in the Kerberos plan |
| Staffing and Training Needs | §13 | ✅ Added by skill — not in the Kerberos plan |
| Schedule | §14 | ✅ Added by skill — not in the Kerberos plan |
| Risks | §15 | ✅ Covered |
| Approvals | §16 | ✅ Added by skill — not in the Kerberos plan |
| Detailed Test Cases | Companion `nri-test-cases.md` | ✅ Covered (separate document) |

---

## Document History

| Version | Date | Author | Change |
|---|---|---|---|
| 1.0–1.7 | 2026-11-02/03 | Node Team | Iterations while building the suite: initial plan, suite-coverage gaps, implemented scenarios, cluster runs, SELinux mount workaround, node fallback and ordering, log-based event source |
| 2.0 | 2026-11-03 | Node Team | Regenerated with `openshift-developer:generate-test-plan`: plan status header, `TC-*` scenario IDs with tiers and traceability, CI placement matrix, criteria assessment, coverage/gap report, test adequacy assessment, deferred-scenario backlog, conformance report and metadata |

---

<!--
plan_version: 2
generated_at: 2026-11-03T00:00:00Z
generated_with: openshift-developer:generate-test-plan
jira_key: OCPNODE-4853
jira_status: To Do
prs_analyzed:
  - url: https://github.com/openshift/origin/pull/31844
    state: open
    base_branch: main
scenario_count: 23
scenarios_by_tier:
  unit: 2
  integration: 0
  e2e-serial: 11
  e2e-periodic: 10
coverage:
  covered: 12
  partial: 2
  gap: 9
  preliminary: 0
core_component: true
feature_gate: null
-->
