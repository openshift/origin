# Test Plan: Podman / CRI-O Shared Library Consolidation (OCP 5.0)

**Test Plan ID**: TP-NODE-PODMAN-CRIO-BUMP-002  
**Version**: 1.0 (Draft)  
**Date**: 2026-11-26  
**Owner**: Node Team  
**Tracking**: OCPNODE-4640

---

## Table of Contents

- [1. Introduction](#1-introduction)
- [2. Test Items](#2-test-items)
- [3. Features to be Tested](#3-features-to-be-tested)
- [4. Features Not to be Tested](#4-features-not-to-be-tested)
- [5. Test Approach](#5-test-approach)
- [6. Pass/Fail Criteria](#6-passfail-criteria)
- [7. Suspension and Resumption Criteria](#7-suspension-and-resumption-criteria)
- [8. Test Deliverables](#8-test-deliverables)
- [9. Environmental Needs](#9-environmental-needs)
- [10. Risks and Contingencies](#10-risks-and-contingencies)

---

## 1. Introduction

### 1.1 Purpose

This document defines the test plan for validating the RHEL/Container-tools-driven Podman version consolidation and the corresponding CRI-O container-libs (image, storage, common) bump for OpenShift Container Platform 5.0.

### 1.2 Background

RHEL/Container-tools is consolidating Podman versions across streams to reduce long-term CVE maintenance overhead. CRI-O and Podman share the same underlying container-libs code (storage, image, common), requiring a matching CRI-O library bump on OpenShift 5.0. Historical data indicates 2-3 boot-breaking bugs have resulted from CRI-O and Podman version mismatches.

### 1.3 Scope

This plan covers the CRI-O library bump validation for OCP 5.0:

| OCP Version | Current Podman | Target Podman | Change Complexity | Risk Tier |
|-------------|----------------|---------------|-------------------|-----------|
| 5.0 | n/a (new release) | 5.8.z (via podman-5.8-rhel downstream) | Large - full container-libs replace bundling 3 features (sync=filesystem, mirror fallback, tar-split fix) plus forced securejoin/selinux/runc downgrades and go-criu protobuf conflict resolution | P0 - Highest |

**Primary Risk**: Storage-format incompatibility between pre-bump and post-bump container-libs versions could corrupt /var/lib/containers/storage, causing image-pull failures or requiring a storage wipe after upgrade.

### 1.4 Objectives

- Confirm OCP 5.0 builds and ships CRI-O correctly with the proposed container-libs set
- Confirm existing container storage remains readable/writable after the library bump
- Identify storage, image-pull, and pod-lifecycle regressions before payload inclusion
- Validate the upgrade path from OCP 4.x to 5.0 does not break running workloads or cached images
- Produce a reusable test-criteria artifact for future consolidation cycles

### 1.5 Audience

| Audience | Use |
|---|---|
| OCP Node Team | Primary test execution and automation |
| QE / Test Engineering | Test coverage review and automation validation |
| Feature Owner | Validation status tracking against 5.0 criteria |
| Container-tools Team | Library version requirements and coordination |

### 1.6 High-Level Milestones

| Milestone | Target | Description | JIRA |
|---|---|---|---|
| M0 | TBD | Library pins and requirements agreed with Container-tools | OCPNODE-4644 |
| M1 | TBD | OCP 5.0 vendor-bump PR created and CI passing | OCPNODE-4648 |
| M2 | TBD | OCP 5.0 cluster validation completed | OCPNODE-4645 |
| M3 | TBD | Automated test suite merged and passing | PR #31722 |
| M4 | TBD | All validation criteria met, evidence documented | OCPNODE-4645 |
| M5 | TBD | Stakeholder sign-off received | - |

---

## 2. Test Items

### 2.1 Components Under Test

| Component | Description | Test Focus |
|---|---|---|
| CRI-O Runtime | Container runtime binary/RPM built from release-5.0 branch | Verification of functionality with updated libraries |
| Shared Libraries | go.podman.io/storage, image, common (version 5.8.z) | Library compatibility and version validation |
| Container Storage | /var/lib/containers/storage on each node | Storage format compatibility pre-bump to post-bump |
| Node Configuration | CRI-O configuration files via MCO | Configuration validity after update |
| Upgrade Path | Cluster upgrade from OCP 4.x to 5.0 | Workload continuity during upgrade |
| Node OS | rhel-coreos-10 (RHEL 10) and rhel-coreos (RHEL 9) | Validation on both OS bases |

---

## 3. Features to be Tested

### 3.1 Build & Packaging
- CRI-O builds cleanly with updated go.mod/vendor tree
- RPM/container packaging succeeds for release-5.0 branch
- No unintended config-schema breaks in default OpenShift CRI-O configuration

### 3.2 Container Storage Compatibility (Critical)
- Existing overlay layers remain accessible after CRI-O restarts with new libraries
- Image layers created before the bump remain usable after the bump
- New images pull and unpack correctly post-bump
- Mixed workload scenarios with pre-bump cached images and newly-pulled images function correctly
- crio check/repair operates safely without false "wipe required" warnings
- Storage metadata (layer IDs, mounts) maintains consistency across reboots

### 3.3 Image Operations
- Image pull from Quay, registry.redhat.io, and mirrored registries
- Multi-layer image handling
- Digest-pinned image operations
- Image list/remove via CRI
- Garbage collection under disk pressure
- Signature/policy behavior remains unchanged

### 3.4 Pod/Container Lifecycle
- Pod create, start, ready, exec, logs, delete operations
- Privileged pods and special networking modes (hostNetwork, hostPID)
- Liveness-probe-driven restarts without storage corruption
- Pod create/delete operations under light load

### 3.5 Node & Cluster Operations
- Fresh installation with bumped CRI-O
- Upgrade from OCP 4.x to 5.0
- Node drain, reboot, and CRI-O service restart
- MachineConfig/ContainerRuntimeConfig application
- CRI-O metrics endpoint availability (localhost:9537/metrics)

### 3.6 Podman ↔ CRI-O Interoperability

#### 3.6.1 Cross-tool Image Operations
- Image pulled by Podman runs via CRI-O without re-download
- Image pulled by CRI-O inspectable and runnable via Podman
- Same image tag pulled by both tools results in no duplicate layers or corruption

#### 3.6.2 Shared Storage Integrity
- Concurrent storage access by both tools without lock contention errors
- Cross-tool metadata compatibility (storage written by one tool readable by the other)
- Interrupted pull recovery without orphaned or corrupted layers

#### 3.6.3 Cross-tool Visibility
- Consistent image listing between podman images and crictl images
- Defined behavior for Podman visibility into CRI-O/kubelet-managed containers

#### 3.6.4 Concurrency and Locking
- Simultaneous image pulls by both tools without storage corruption
- Safe reference counting when one tool removes an image in use by the other

#### 3.6.5 Garbage Collection Interplay
- Manual Podman cleanup (podman system prune/rm) behavior with CRI-O-managed resources
- CRI-O image garbage collection under disk pressure maintains isolation from Podman-cached images

#### 3.6.6 Version Compatibility (Critical)
- Pre-bump Podman + Pre-bump CRI-O (baseline from 4.x)
- Post-bump Podman + Post-bump CRI-O (target state for 5.0)
- Pre-bump storage data readable by post-bump CRI-O (4.x to 5.0 upgrade scenario)
- Version mismatch scenarios documented

#### 3.6.7 Failure Recovery
- Node recovery after mixed podman/crio activity
- crio wipe/crio check --repair behavior after version bump

### 3.7 Regression / CI Gates
- Required Prow jobs on openshift/cri-o PR for release-5.0 branch
- Release-informing/payload jobs after CRI-O promotion

---

## 4. Features Not to be Tested

| Area | Rationale |
|---|---|
| Full Podman CLI conformance suite | Owned by Container-tools team |
| Non-OpenShift standalone CRI-O deployments | Out of Node team scope |
| Exhaustive CVE residual-risk analysis | Covered by separate security process |
| Performance SLOs / large-scale testing (1000+ pods/node) | Optional follow-up if regression detected |
| Go mod replace vendoring mechanism | Covered by openshift/enhancements#2076 |

---

## 5. Test Approach

### 5.1 Testing Levels

| Level | Description | Execution Trigger |
|---|---|---|
| L1 - Unit/Integration | CRI-O make test/integration | Every pull request |
| L2 - Prow e2e | Branch-configured e2e jobs | Every pull request |
| L3 - Cluster Smoke | Candidate build on real cluster | Before release promotion |
| L4 - Upgrade Validation | Upgrade with existing storage data | Required for P0 |
| L5 - Soak (Optional) | Extended runtime testing (1-2 hours) | P0 if capacity permits |

### 5.2 Testing Types

- **Functional**: Core CRI-O operations validation with updated libraries
- **Interoperability**: Podman ↔ CRI-O shared storage compatibility
- **Upgrade/Migration**: Storage data compatibility validation (4.x to 5.0)
- **Regression**: Verification that existing functionality remains intact
- **Security**: Signature/policy behavior verification, CVE prevention

### 5.3 Test Strategy

#### 5.3.1 Overall Approach
1. Obtain exact library pin requirements from Container-tools team
2. Create vendor-bump pull request for openshift/cri-o release-5.0 branch
3. Validate all required CI jobs pass (investigate failures, avoid blind overrides)
4. Execute manual cluster validation including storage upgrade scenario
5. Promote to 5.0 release upon meeting all pass criteria
6. Document all evidence in JIRA tracking system

#### 5.3.2 Storage Upgrade Scenario (Mandatory for P0)

This scenario validates storage written by pre-bump libraries remains readable by post-bump libraries.

| Step | Action | Expected Result |
|---|---|---|
| S1 | Install cluster with pre-bump CRI-O (from 4.x) | Cluster operational with all operators healthy |
| S2 | Deploy workloads and pull multiple container images | Images cached to /var/lib/containers/storage |
| S3 | Record crictl images output and sample storage paths | Baseline captured for comparison |
| S4 | Upgrade cluster to OCP 5.0 with bumped CRI-O | Nodes transition to Ready state |
| S5 | Verify existing pods continue running or restart cleanly | No ImagePullBackOff errors observed |
| S6 | Delete and recreate pods using cached images | No re-download required, pods start successfully |
| S7 | Pull new image and create pod | Operation succeeds without errors |
| S8 | Reboot worker node | Node and pods recover successfully |
| S9 | Verify no storage wipe was required | Storage remains intact, no manual intervention needed |

**Critical Failure Signature:**
```
error creating read-write layer with ID "...": Stat
/var/lib/containers/storage/overlay/...: no such file or directory
```

This error indicates storage corruption and constitutes a blocker for OCP 5.0 promotion.

---

## 6. Pass/Fail Criteria

### 6.1 Pass Criteria (Ready for Promotion)

The OCP 5.0 branch is ready for promotion when:
- All required CI jobs pass (or failures triaged as infrastructure issues with bugs filed)
- Smoke checklist completed with zero Sev1/Sev2 defects attributable to the library bump
- Storage upgrade scenario (S1-S9) passes on at least one representative cluster
- No unresolved storage-layer/ImagePullBackOff defects matching historical failure patterns
- Technical advisor provides approval for library pin set
- All evidence documented in OCPNODE-4645 JIRA story
- Automated test suite passes on 5.0 candidate builds

### 6.2 Fail Criteria (Do Not Promote)

Promotion is blocked if:
- CRI-O fails to start or exhibits crash-loop behavior
- Nodes report NotReady status due to runtime errors
- Existing cached images become unusable without storage wipe
- Systematic ImagePullBackOff or layer Stat errors occur
- Required CI jobs persistently fail for bump-related causes

### 6.3 Severity Classification

| Severity | Definition | Example | Required Action |
|---|---|---|---|
| Blocker | Critical functionality failure | Storage corruption, forced wipe, mass ImagePullBackOff | Stop promotion, escalate immediately |
| Sev1 | Major functionality impairment | Pod creation failures, CRI-O crashes under load | Fix before promotion |
| Sev2 | Minor functionality issue | Edge case configuration issues | Fix or document, apply judgment |
| Sev3 | Cosmetic or logging issue | Minor logging inconsistencies | May promote with tracked bug |

### 6.4 Coordination with Container-tools

Before declaring validation complete, confirm:
- Podman target version matches agreed specification (5.8.z for OCP 5.0)
- Library pin set approved by Container-tools team
- Any known one-time repair or migration flags documented

---

## 7. Suspension and Resumption Criteria

### 7.1 Suspension Criteria

Testing will be suspended when:
- Container-tools team modifies target Podman version mid-stream
- Blocker defect discovered affecting entire 5.0 release
- CI/payload infrastructure unable to produce installable candidates for 5+ business days

### 7.2 Resumption Criteria

Testing will resume when:
- New version requirements agreed and pull requests updated
- Blocker defect resolved and validation re-executed
- Infrastructure issues resolved and stable builds available

---

## 8. Test Deliverables

| Deliverable | Description |
|---|---|
| Test Plan | This document |
| Test Results Table | Pass/fail status per test case with execution evidence |
| CI Evidence | Links to passing Prow jobs and payload validation |
| Defect Reports | OCPBUGS tickets for issues discovered, linked to OCPNODE-4640 |
| Automated Test Suite | Ginkgo test implementation in openshift/origin (PR #31722) |
| Test Execution Report | Summary with environment details, test results, and recommendations |
| Sign-off Documentation | Formal approval from stakeholders |

---

## 9. Environmental Needs

### 9.1 Cluster Configurations

| Requirement | Specification |
|---|---|
| OCP Version | 5.0 (target release) |
| Cluster Topology | 3 control plane + 2+ worker nodes |
| Architecture | x86_64 (primary), aarch64 (optional) |
| Cloud Provider | AWS or GCP |

### 9.2 Topology Requirements

| Topology | Priority | Rationale |
|---|---|---|
| SNO (Single Node OpenShift) | Highest | Most storage-sensitive configuration, no HA fallback |
| Standard Multi-Node | Required | Baseline configuration coverage |
| MicroShift | Required | Edge deployment storage footprint validation |
| HyperShift | Required | Hosted control-plane architecture validation |

### 9.3 Node OS Base

Both rhel-coreos (RHEL 9-based) and rhel-coreos-10 (RHEL 10-based) require independent validation. RHEL 9 results do not transfer to RHEL 10 due to kernel/glibc differences affecting Syncfs() and overlay driver behavior.

| Node Base Image | Priority | Notes |
|---|---|---|
| rhel-coreos (RHEL 9-based) | Required | Current base OS |
| rhel-coreos-10 (RHEL 10-based) | Required | New for OCP 5.0 |

### 9.4 Test Data Requirements

Test scenarios require:
- Multi-layer container images
- Images from Quay and registry.redhat.io registries
- Digest-pinned images for reproducibility
- IDMS (ImageDigestMirrorSet) configurations for mirror testing

### 9.5 Automated Test Suite

**Location**: openshift/origin PR #31722  
**Implementation**: test/extended/node/node_e2e/podman_crio_bump.go, image_mirror_set.go  
**Coverage**: 9 automated test cases addressing critical scenarios

**Automated Test Cases:**

| Test Case |
|---|
| Verify sync=filesystem configuration and basic storage operations |
| Verify Podman and CRI-O concurrent image pulls do not corrupt shared storage |
| Verify Podman and CRI-O handle multiple multi-layer images with layer reuse and digest pulls |
| Verify IDMS mirror configuration is applied to Podman and CRI-O with fallback behavior |
| refuse to remove an image still in use by the other tool, in either direction |
| read shared image storage concurrently and agree on cross-tool metadata |
| recover cleanly when a pull is interrupted mid-transfer, in either direction |
| Confirm a node recovers cleanly after mixed Podman/CRI-O activity |
| Confirm CRI-O doesn't wrongly repair healthy storage after the bump |
| Verify Podman and CRI-O share pulled images and layers correctly |

**Cluster Type Detection:**
Tests automatically detect cluster topology: MNO (HighlyAvailable), SNO (SingleReplica), HyperShift (External), MicroShift (via configmap check).

**Execution:**
```bash
openshift-tests run-test "[sig-node] [Jira:Node/Kubelet] Podman and CRI-O version compatibility"
```

**CI Integration Plan:**
- Trigger: PRs affecting CRI-O, container-libs, or Podman/CRI-O compatibility tests
- Initial classification: Informing
- Promotion path: Release-blocking when automation stable

---

## 10. Risks and Contingencies

| Risk | Impact | Probability | Mitigation | Contingency |
|---|---|---|---|---|
| Storage format incompatibility (pre-bump to post-bump) | High - forces storage wipe, breaks upgrade path | Medium | Mandatory storage upgrade scenario (S1-S9), monitor for failure signature | Stop all promotions, engage Container-tools for root cause analysis |
| OCP 5.0 complexity (3 features + dependency changes) | High - increased failure surface | Medium | Full test coverage, extended validation timeline | Allocate additional validation resources, execute soak testing |
| Version skew (RHEL ships new Podman before CRI-O update) | Medium - temporary incompatibility window | Medium | Test mismatch scenarios, document safe combinations | Coordinate release timing with Container-tools team |
| RHEL 10 behavioral differences | Medium - kernel/glibc changes affect storage operations | Medium | Independent RHEL 9 and RHEL 10 validation | OS-specific debugging and issue resolution |
| CI infrastructure instability | Medium - validation timeline delays | Low | Separate infra issue tracking, no blind failure overrides | Suspend testing per criteria, escalate to Test Platform team |
| Automated test suite merge delays | Medium - reduced automation coverage | Low | Manual testing as fallback, prioritize PR review | Execute manual validation, merge automation for future iterations |

---

## Document History

| Version | Date | Author | Changes |
|---|---|---|
| 1.0 (Draft) | 2026-11-26 | Node QA Team | Initial test plan for OCP 5.0 based on design doc and kerberos template |

---

## Appendix A: Validation Checklist

| Validation Item | Status |
|---|---|
| Library versions confirmed with Container-tools team | |
| CRI-O vendor-bump PR created and merged | |
| All required CI jobs passing | |
| Fresh install successful (cluster operators healthy) | |
| CRI-O version verification (5.8.z) | |
| Sample deployment reaches Ready state | |
| CRI-O running without crashes | |
| Storage upgrade scenario (S1-S9) passed | |
| Podman/CRI-O interoperability tests passed | |
| Privileged pod validation | |
| Node reboot successful | |
| CRI-O metrics endpoint available | |
| No storage wipe required | |
| Automated test suite passing | |
| RHEL 9 and RHEL 10 validation completed | |
| Evidence documented in JIRA | |
| Stakeholder sign-off received | |

---

## Appendix B: Manual Test Cases

| TC ID | Test Case | Coverage Area |
|---|---|---|
| TC-01 | Confirm pre-bump setup operates normally | Baseline validation |
| TC-03 | Upgrade validation - old data remains usable after bump (S1-S9) | Critical upgrade path |
| TC-04 | Version mismatch scenario testing | Skew window validation |
| TC-14 | Podman visibility into CRI-O/kubelet-managed containers | Container visibility |
| TC-17 | podman system prune behavior with CRI-O resources | Manual cleanup safety |
| TC-18 | CRI-O garbage collection under disk pressure | GC isolation |
| TC-19 | MachineConfig/ContainerRuntimeConfig persistence | Configuration management |

---

## Glossary

**CRI-O**: Container Runtime Interface for OpenShift - the runtime that executes containers on worker nodes

**Podman**: Container management tool sharing storage libraries with CRI-O

**Container-libs**: Shared Go libraries (storage, image, common) used by both Podman and CRI-O

**Storage consolidation**: Library version synchronization between Podman and CRI-O to prevent incompatibilities

**MCO**: Machine Config Operator - manages node configuration in OpenShift

**SNO**: Single Node OpenShift - single-node deployment topology

**MicroShift**: Lightweight OpenShift variant for edge computing

**HyperShift**: OpenShift architecture with externally-hosted control plane

**IDMS**: ImageDigestMirrorSet - configuration for container image mirroring

---

**References**:
- Parent Epic: [OCPNODE-4640](https://issues.redhat.com/browse/OCPNODE-4640)
- Enhancement Documentation: [openshift/enhancements#2076](https://github.com/openshift/enhancements/pull/2076)
- OCP/RHEL Version Mapping: [https://access.redhat.com/articles/6907891](https://access.redhat.com/articles/6907891)
- Automated Test Implementation: [openshift/origin#31722](https://github.com/openshift/origin/pull/31722)
