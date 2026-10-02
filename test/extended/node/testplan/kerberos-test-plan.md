# Test Plan: Kerberos Authentication on OpenShift (OCP 5.1)

**Test Plan ID**: TP-OCPNODE-4776-01  
**Version**: 2.1  
**Date**: 2026-10-09  
**Owner**: Node Team  
**Tracking**: OCPNODE-4776 / OCPSTRAT-3418

---

## Table of Contents

- [1. Introduction](#1-introduction)
  - [1.1 Purpose](#11-purpose)
  - [1.2 Scope](#12-scope)
  - [1.3 Audience](#13-audience)
  - [1.4 Objectives](#14-objectives)
- [2. Test Items](#2-test-items)
  - [2.1 Components Under Test](#21-components-under-test)
- [3. Features to be Tested](#3-features-to-be-tested)
- [4. Features Not to be Tested](#4-features-not-to-be-tested)
- [5. Test Approach](#5-test-approach)
  - [5.1 Testing Levels](#51-testing-levels)
  - [5.2 Testing Types](#52-testing-types)
- [6. Testing Tasks](#6-testing-tasks)
- [7. Item Pass/Fail Criteria](#7-item-passfail-criteria)
- [8. Suspension and Resumption Criteria](#8-suspension-and-resumption-criteria)
- [9. Test Deliverables](#9-test-deliverables)
- [10. Environmental Needs](#10-environmental-needs)
  - [10.1 Cluster Configurations](#101-cluster-configurations)
  - [10.2 SNO vs Multi-Node](#102-sno-vs-multi-node)
  - [10.3 Test Data](#103-test-data)
  - [10.4 Test Environment Strategy](#104-test-environment-strategy)
- [11. Risks and Contingencies](#11-risks-and-contingencies)
- [Document History](#document-history)

---

## 1. Introduction

OpenShift's default security policies (`restricted-v2` SCC, `RuntimeDefault` seccomp, `container_t` SELinux type) block the kernel features that Kerberos requires:

- **Seccomp**: `RuntimeDefault` filters `keyctl`, `add_key`, and `request_key` — the syscalls MIT Kerberos uses to store TGTs in the kernel keyring (`KEYRING:session:%{uid}`).
- **SELinux**: `container_t` does not grant the `key` object class permissions. Even if seccomp allowed the syscall, SELinux would deny the keyring operation.

The solution uses the **Security Profiles Operator (SPO)** to deploy custom SELinux and seccomp profiles that grant only the Kerberos-specific permissions, combined with a custom SCC (`kerberos-restricted`) that is otherwise equivalent to `restricted-v2`. This avoids privileged containers or weakening the overall security model.

No feature gate is required — the solution uses only GA OCP APIs (SCC, RBAC, CRDs) combined with the Security Profiles Operator installed from OperatorHub as a Rolling Stream operator independent of the OCP release.

### 1.1 Purpose

This document defines the test plan for validating Kerberos authentication support in OpenShift Container Platform 5.1. It describes what is tested, how it is tested, the pass/fail criteria, and the environment required to execute the test suite.

### 1.2 Scope

This plan covers the SPO-based Kerberos authentication solution on OCP 5.1, including:

- Security Profiles Operator profile deployment and lifecycle
- Custom SCC (`kerberos-restricted`) admission and RBAC controls
- Kernel keyring access from containers (SELinux + seccomp)
- Full Kerberos authentication flow (`kinit`/`klist`/`kdestroy`) across supported deployment topologies
- Pod lifecycle behavior, ticket lifecycle, upgrade resilience, and error handling
- Testing on both standard multi-node clusters and Single Node OpenShift (SNO)

### 1.3 Audience

| Audience | Use |
|---|---|
| OCP Node Team | Primary test owners; execute and automate the test suite |
| QE / Test Engineering | Review test coverage, validate automation, sign off on results |
| Feature Owner | Track pass/fail status against 5.1 validation criteria |

### 1.4 Objectives

- Verify `krb5-workstation` and related Kerberos packages are available in UBI 10 for use in OCP pods
- Validate SELinux `kerberos_container` policy allows required operations
- Confirm SCC configuration enables Kerberos while maintaining security posture equivalent to `restricted-v2`
- Test `KEYRING:session`-based credential cache mechanism
- Ensure FIPS compliance for government use cases
- Verify upgrade paths do not break Kerberos functionality

---

## 2. Test Items

### 2.1 Components Under Test

| Component Group | Component | Description |
|---|---|---|
| **RHCOS Extensions** | `krb5-workstation` package | MIT Kerberos client tools (`kinit`, `klist`, `kdestroy`, `kinit -R`) on RHCOS nodes |
| **RHCOS Extensions** | `cifs-utils` package | CIFS/SMB client utilities required for Kerberized file share access (future scope) |
| **SELinux Policy** | `selinux-policy-targeted` with `kerberos_container` template | Custom SELinux type granting `key` object class permissions (`@self`, `gssd_t`, `kerberos_port_t`) |
| **SELinux Policy** | Container access to kernel keyring | `KEYRING:session:%{uid}` accessible under the custom SELinux type (not `container_t`) |
| **Security Context Constraints** | `kerberos-restricted` SCC | New Kerberos-enabled SCC equivalent to `restricted-v2` plus SELinux type + seccomp overrides |
| **Security Context Constraints** | Seccomp profile (`keyctl`, `add_key`, `request_key`) | SPO-managed seccomp profile allowing the three Kerberos kernel syscalls |
| **Credential Cache Infrastructure** | `KEYRING:persistent:UID` | Persistent kernel keyring credential cache surviving process boundaries within the same UID |
| **Credential Cache Infrastructure** | `FILE`-based cache (`/var/lib/kubelet/kerberos/`) | File-based TGT storage alternative (future scope — required for CSI driver integration) |
| **Configuration Management** | MachineConfig `/etc/krb5.conf` | Node-level Kerberos client configuration deployed via MachineConfig |
| **Configuration Management** | MachineConfig `/etc/request-key.d/cifs.spnego.conf` | CIFS SPNEGO request-key configuration for Kerberized SMB mounts |
| **Configuration Management** | Secret-based keytab distribution | Kubernetes Secret for delivering keytabs to pods without embedding credentials in images |
| **CSI Driver Integration** | SMB CSI Driver Operator (`--krb5-cache-directory`) | CSI driver flag pointing to the Kerberos credential cache directory (future scope) |
| **CSI Driver Integration** | ClusterCSIDriver API extensions | API extensions for Kerberos-aware CSI driver configuration (future scope) |

---

## 3. Features to be Tested

- SPO installation and custom profile lifecycle (install, status, update, delete)
- SELinux and seccomp profile installation timing and `SecurityProfileNodeStatus` reporting
- `kerberos-restricted` SCC admission — granted to dedicated SA, denied to others
- Kernel keyring access: `keyctl` syscall and SELinux process label verification
- Defense-in-depth: neither profile alone is sufficient
- Kerberos authentication flow: `kinit` (keytab + password), `klist`, `kdestroy`
- Deployment topologies: single pod, multi-replica Deployment, StatefulSet, DaemonSet, cross-namespace
- Pod lifecycle: restart, rolling update, deletion/recreation — ticket loss and keytab reacquisition
- Ticket lifecycle: expiry, renewal (`kinit -R`), keytab reacquisition
- `ProfileBinding` namespace boundary behavior
- Error handling: wrong keytab, wrong principal, unreachable KDC, corrupted `krb5.conf`, `subPath` + `optional: true` pitfall
- Profile lifecycle pitfalls: premature scheduling, same-named profile collision, finalizer behavior
- Observability: AVC denial logging, seccomp violation detection, SPO `ProfileRecording`
- Node resilience: reboot, drain, new worker auto-profile installation
- Upgrade resilience: OCP z-stream, SPO operator, RHCOS node update
- Encryption: AES-256, AES-128, FIPS cluster compatibility

---

## 4. Features Not to be Tested

| Feature | Reason |
|---|---|
| CIFS/SMB CSI Driver integration | Storage team scope; depends on this feature |
| Kerberized NFS mounts | Requires CSI driver work outside this plan |
| Active Directory integration | Tests use a self-contained in-cluster MIT KDC |
| Automatic keytab rotation controller | Not in OCP 5.1 scope |

---

## 5. Test Approach

This feature is **configuration-driven** — the deliverable is a set of YAML manifests and an operational procedure, not application code. Correctness can only be validated against a live cluster with SELinux enforcement and a running kernel.

**Upstream tests: not applicable.** The feature depends entirely on OCP-specific APIs — `SecurityContextConstraints`, the Security Profiles Operator, and RHCOS SELinux policy — none of which exist in upstream Kubernetes. All tests live downstream in `openshift/origin` under `test/extended/node/`.

### 5.1 Testing Levels

| Level | Description | Coverage | Priority | Status |
|---|---|---|---|---|
| **Unit** | Not applicable — feature is YAML/configuration-driven with no isolatable application code | None | — | N/A |
| **Integration** | Component interactions: SPO ↔ node, SCC admission, SELinux/seccomp enforcement, profile lifecycle. No KDC required. | SPO install, profile deployment, SCC/RBAC, defense-in-depth, namespace boundary, lifecycle pitfalls, observability | P0–P2 | To be automated / Manual |
| **System** | Full-stack tests: OCP + SPO + MIT KDC + cluster operations. End-to-end Kerberos, deployment topologies, ticket lifecycle, disruptive scenarios. | Single/multi-pod auth, StatefulSet, DaemonSet, pod lifecycle, ticket expiry/renewal, negative error cases, node reboot/drain, upgrade resilience | P0–P3 | To be automated / Manual |
| **Acceptance** | Validates the feature meets its P0 definition of done from a user perspective: profile installation, SCC admission, successful `kinit` on both multi-node and SNO. | Core SPO + SCC setup and single-pod Kerberos end-to-end | P0 | To be automated |

### 5.2 Testing Types

| Type | Description | Coverage | Priority | Status |
|---|---|---|---|---|
| **Functional** | Core Kerberos authentication flow, SCC and profile behavior, deployment topologies, pod and ticket lifecycle | SPO install, SCC admission, `kinit`/`klist`/`kdestroy`, multi-pod, StatefulSet, pod restart, ticket renewal, scheduler placement | P0–P1 | To be automated |
| **Security** | Defense-in-depth (both profiles required), SCC confinement, namespace isolation, privilege escalation prevention | Profile verification, SCC posture, unauthorized SA rejection, error handling for misconfigurations | P0–P2 | To be automated |
| **Compatibility** | x86_64 and aarch64 architectures, AES-256 and AES-128 encryption types, FIPS-enabled clusters | Encryption type validation, FIPS cluster behavior | P3 | Manual |
| **Upgrade** | OCP z-stream, SPO operator upgrade, RHCOS node update — zero regression across all upgrade paths | Profile and SCC survival across OCP/SPO/RHCOS updates | P3 | Manual |
| **Regression** | All P0–P1 automated tests run in OCP CI (Prow) nightly jobs and pre-merge checks to detect feature regression across builds | Core setup, single-pod Kerberos, production topologies, lifecycle | P0–P1 | To be automated |

**Classification rules:**
- Can the test pass without running `kinit` against a KDC? → Integration
- Needs the full authentication flow? → System / E2E
- Reboots, drains, upgrades nodes, or corrupts policy? → Disruptive (separate CI job)

---

## 6. Testing Tasks

### Test Cases

#### 6.1 Integration Tests
> OCP + SPO installed. No KDC required.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| IT-01 | SPO Installation and Profile Deployment | SPO installs correctly; both SELinux and seccomp profiles deploy to all workers with correct status and timing | P0 | To be automated |
| IT-02 | SCC and RBAC Configuration | `kerberos-restricted` SCC matches `restricted-v2` security posture; bound only to dedicated SA | P0 | To be automated |
| IT-03 | Profile Verification — keyctl and SELinux Label | Both profiles active: `keyctl` syscall succeeds AND SELinux label shows custom type (not `container_t`) | P0 | To be automated |
| IT-04 | Defense in Depth — Both Profiles Required | Neither profile alone is sufficient — `restricted-v2`, seccomp-only, and SELinux-only all fail with documented errno | P0 | To be automated |
| IT-05 | SCC Authorization and Pod Confinement | Unauthorized SAs cannot use the SCC; Kerberos pods cannot escalate beyond keyring operations | P0 | To be automated |
| IT-06 | ProfileBinding Namespace Boundary | ProfileBinding behavior for namespace-B workloads, including binding namespace and namespace enablement | P2 | To be automated |
| IT-07 | Profile Lifecycle Pitfalls | Premature scheduling (CRI-O error), same-named profile collision, finalizer behavior on deletion | P2 | Manual |
| IT-08 | Observability — AVC and Seccomp Logging | AVC denials and seccomp violations visible in kernel logs; SPO ProfileRecording captures Kerberos syscalls | P2 | Manual |

#### 6.2 E2E Functional Tests
> Full stack: OCP + SPO + MIT KDC pod.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| E2E-01 | Single Pod Kerberos Authentication | Complete lifecycle: `kinit` (keytab + password), `klist`, `kdestroy`, clean audit log | P0 | To be automated |
| E2E-02 | Multi-Replica Deployment with Keyring Isolation | Replicas authenticate independently with isolated keyrings; scale-up/down works | P1 | To be automated |
| E2E-03 | StatefulSet Kerberos Authentication | Ordered lifecycle + Kerberos; replacement pods reacquire tickets via keytab | P1 | To be automated |
| E2E-04 | Cross-Workload and Cross-Namespace Access | Multiple workloads in same/different namespaces authenticate independently; DaemonSet works | P1 | To be automated |
| E2E-05 | Pod Restart, Update, and Recreation | Tickets lost on all lifecycle events (expected); keytab reacquisition always succeeds | P1 | To be automated |
| E2E-06 | Init Container and Sidecar Keyring Sharing | Discovery: init/sidecar keyrings visible to main container? Drives renewal architecture recommendation | P2 | Manual |
| E2E-07 | Ticket Expiry, Renewal, and Reacquisition | `kinit -R` works before expiry, fails after; keytab always recovers; error messages documented | P1 | To be automated |
| E2E-08 | Scheduler Placement Without Node Pinning | Kerberos works without explicit `nodeName` — scheduler places pod on any profile-ready node | P1 | To be automated |
| E2E-09 | Encryption Types and FIPS Compatibility | AES-256 and AES-128 both work; FIPS cluster behavior documented | P3 | Manual |

#### 6.3 Negative / Error Handling Tests
> Full stack with intentionally broken configurations.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| NEG-01 | Invalid Keytab, Principal, and Config Errors | Clear errors for wrong keytab, wrong principal, corrupted `krb5.conf`, missing keytab file | P2 | To be automated |
| NEG-02 | KDC Unreachable and Expired Keytab | Clear errors for `Cannot contact any KDC` and `Preauthentication failed` | P2 | To be automated |
| NEG-03 | subPath Mount with Optional Nonexistent Volume | CRI-O rejects `subPath` + `optional: true` for missing Secret | P2 | Manual |

#### 6.4 E2E Disruptive Tests
> ⚠️ Node-level or cluster-level operations. Dedicated CI jobs only.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| DIS-01 | Node Reboot and Drain Recovery | Profiles survive reboot; Kerberos works after drain/reschedule | P3 | Manual |
| DIS-02 | New Node Auto-Profile Installation | SPO auto-installs profiles on new worker; timing documented | P3 | Manual |
| DIS-03 | OCP, SPO, and RHCOS Upgrade Resilience | Kerberos survives OCP z-stream, SPO operator, and RHCOS updates — zero regression | P3 | Manual |
| DIS-04 | SELinux Profile Naming Collision | Discovery: naming a SelinuxProfile "kerberos" overwrites system module — confirm and document recovery | P3 | Manual |

### Execution Order

| Phase | Test IDs | Type | Gate |
|---|---|---|---|
| 1 | IT-01, IT-02 | Integration | Infrastructure verified |
| 2 | IT-03, IT-04, IT-05 | Integration | Profiles and security model validated |
| 3 | E2E-01 | E2E | Single-pod Kerberos works |
| 4 | E2E-02, E2E-03, E2E-04 | E2E | Deployment topologies work |
| 5 | E2E-05, E2E-06 | E2E | Lifecycle and discovery |
| 6 | E2E-07, E2E-08 | E2E | Ticket lifecycle and scheduling |
| 7 | IT-06, IT-07, IT-08 | Integration | Namespace boundary, pitfalls, observability |
| 8 | NEG-01, NEG-02, NEG-03 | Negative | Error handling |
| 9 | E2E-09 | E2E | Encryption and FIPS |
| 10 | DIS-01, DIS-02 | Disruptive | Node operations |
| 11 | DIS-03, DIS-04 | Disruptive | Upgrades and naming collision |

**Phases 1–3 are P0 blockers.** Do not proceed to later phases if these fail.

---

## 7. Item Pass/Fail Criteria

The feature is validated for OCP 5.1 when **all** of the following are met:

1. No critical or major defects remain open for SPO profile deployment, SCC configuration, kernel keyring access, or downstream automation.

2. IT-01–IT-08 pass consistently: SPO v0.10.0 installs, both profiles reach `Installed` on every worker via `SecurityProfileNodeStatus`, `kerberos-restricted` SCC is bound only to the dedicated SA.

3. A pod with the correct SCC, SELinux profile, and seccomp profile successfully executes `kinit`/`klist`/`kdestroy` using `KEYRING:session` (E2E-01). A pod missing either profile is blocked with `EPERM` or `EACCES` (IT-04, IT-05).

4. Defense-in-depth confirmed: seccomp-only → syscall denied with the errno generated by the enforcing SELinux policy; SELinux-only → syscall denied with the errno generated by the configured seccomp profile; restricted-v2 → Kerberos keyring operation denied. Only the combination of both profiles succeeds (IT-03, IT-04). Exact errno values are to be documented after IT-04 runs on the target OCP 5.1 / RHCOS 10 environment. SCC is equivalent to `restricted-v2` except for SELinux type + seccomp (IT-02).

5. Multi-pod, StatefulSet, and cross-workload deployments authenticate independently with isolated per-session keyrings (E2E-02–E2E-04). Pod replacement and rolling updates lose tickets; keytab reacquisition always succeeds (E2E-05).

6. Negative scenarios produce clear, specific error messages and exit non-zero; no CrashLoopBackOff (NEG-01–NEG-03).

7. Profiles survive node reboot; `kinit` succeeds after drain/reschedule (DIS-01). SPO auto-installs profiles on new workers (DIS-02).

8. OCP z-stream, SPO operator, and RHCOS updates preserve profiles and Kerberos functionality; zero regression (DIS-03).

9. Discovery tests E2E-06 and DIS-04 produce documented findings with command output and architectural recommendations.

10. `kinit -R` works before expiry, fails after; keytab recovery always works (E2E-07). AES-256 and AES-128 both work; FIPS behavior documented (E2E-09).

### Failure Handling

| Condition | Action |
|---|---|
| Any P0 test fails | Stop. File a blocker. Root-cause before continuing. Feature not validated. |
| 1–2 P1 tests fail with workaround | Validated with known limitations. File high-priority bugs. |
| 3+ P1 tests fail, or no workaround | Not validated. Escalate to feature owner. |
| P2/P3 test fails | Continue. File normal bug. Note in test report. |
| Previously passing test fails | Regression — escalate one priority level. |
| Discovery test (E2E-06, DIS-04) | No pass/fail verdict. Record findings and architectural recommendation. |

---

## 8. Suspension and Resumption Criteria

### Suspension

Testing is suspended if:

- Any P0 test fails (IT-01–IT-05, E2E-01) — the environment or feature is broken; further results are unreliable.
- SPO is unavailable in OCP 5.1 OperatorHub — prerequisite for all tests.
- The in-cluster MIT KDC pod fails to start — E2E tests cannot run without a KDC.

### Resumption

Testing resumes when:

- The root cause of the P0 failure is identified and fixed, and the affected test passes on a clean re-run.
- SPO is confirmed available in the target OCP build.
- The KDC pod is healthy and principals are accessible.

---

## 9. Test Deliverables

| Deliverable | Description |
|---|---|
| This test plan | `test/extended/node/testplan/kerberos-test-plan.md` |
| Test runbook | Manual execution steps, exact commands, and expected output for each test ID (tracked under OCPNODE-4776) |
| Automated test suite | Ginkgo `.go` files under `test/extended/node/` for P0 and P1 tests |
| Test execution report | Pass/fail results per test ID, environment details, defects filed |
| Discovery findings | Documented output and recommendations for E2E-06 and DIS-04 |
| Configuration examples | Working YAML for `SelinuxProfile`, `SeccompProfile`, `kerberos-restricted` SCC, `krb5.conf` ConfigMap, keytab Secret |

---

## 10. Environmental Needs

### 10.1 Cluster Configurations

| Item | Requirement |
|---|---|
| OCP Version | 5.1 (target release; the solution is version-agnostic and can run on any OCP release that supports SPO v0.10.0) |
| Topology — primary | 3 control plane + ≥ 2 workers (required for DIS-01 drain, DIS-02 scale-up) |
| Topology | Single Node OpenShift & MultiNode |
| Architecture | x86_64 (primary), aarch64 |
| SPO Version | 0.10.0 via `release-alpha-rhel-8` channel |
| FIPS | FIPS-enabled cluster (for E2E-09) |

### 10.2 SNO vs Multi-Node

| Scenario | Multi-Node | SNO |
|---|---|---|
| SPO + profiles + SCC + `kinit` (P0) | Required | Required |
| Multi-replica / StatefulSet | Yes | Yes |
| Node drain to second worker (DIS-01) | Yes | N/A |
| New worker auto-install (DIS-02) | Yes | N/A |
| Node reboot (DIS-01) | Yes | Yes |

### 10.3 Test Data

| Data Item | Details |
|---|---|
| MIT KDC | In-cluster pod, realm `TEST.LOCAL`, privileged SCC for `kdb5_util` |
| Test principal | `testuser@TEST.LOCAL` |
| Service principal | `svc/testhost.test.local@TEST.LOCAL` |
| Keytab | Exported via `kadmin.local ktadd`; stored as Kubernetes Secret |
| `krb5.conf` | ConfigMap pointing to `kdc-service.kdc-infra.svc.cluster.local` |
| Invalid keytab | Mismatched principal keytab (for NEG-01) |
| Invalid `krb5.conf` | Syntactically broken config (for NEG-01) |
| Short-lived `krb5.conf` | `ticket_lifetime=5m`, `renew_lifetime=30m` (for E2E-07) |
| Test container image | UBI10-minimal + `krb5-workstation` + `keyutils-libs` + `python3` |

### 10.4 Test Environment Strategy

#### 10.4.1 Lab Environments

**Kerberos Infrastructure**

| Component | Details |
|---|---|
| MIT Kerberos KDC | RHEL 10 / UBI 10 in-cluster pod; realm `TEST.LOCAL` |
| Kerberized NFS server | For future NFS mount validation |
| Kerberized SMB/CIFS server | For future CIFS CSI driver integration testing |

**Platform Coverage**

| Platform | Priority |
|---|---|
| AWS | Primary |
| Azure | Secondary |
| GCP | Secondary |
| Bare metal | Secondary |
| VMware vSphere | Secondary |

#### 10.4.2 CI/CD Integration

| Practice | Details |
|---|---|
| Automated tests in OpenShift CI (Prow) | P0 and P1 Ginkgo tests live under `test/extended/node/` and run via `openshift-tests` in Prow jobs |
| SPO availability check | Tests skip gracefully (`g.Skip`) when SPO is not installed — SPO is an optional Rolling Stream operator, not in the OCP payload; standard CI jobs will not have it pre-installed |
| Dedicated nightly job | A separate Prow job with SPO pre-installed runs the Kerberos suite nightly against development builds |
| Pre-merge scope | Pre-merge CI is triggered only on PRs that touch SPO, CRI-O, kubelet, or the Kerberos test files themselves — not on all merges |
| Job classification | Tests start as **informing** (failures do not block the release). After automation matures and results stabilize, promotion to release-blocking is evaluated. |

---

## 11. Risks and Contingencies

| Risk | Impact | Probability | Contingency |
|---|---|---|---|
| SPO unavailable in OCP 5.1 OperatorHub | Blocker — full suite cannot run | Low | Verify in first 5.1 nightly; SPO is a Rolling Stream operator independent of OCP releases |
| MIT KDC pod requires root (`kdb5_util`) | Test infra only; no production impact | Low | Use privileged SCC for KDC pod only; it is test infrastructure, not the system under test |
| Sidecar keyring not shared with main container | Limits the recommended renewal architecture | Medium | E2E-06 determines this; if not shared, document in-app `kinit` as the recommended pattern |
| FIPS mode blocks some Kerberos encryption types | Compliance gap | Medium | Run E2E-09 on FIPS cluster; document which enctypes are allowed or blocked |
| SELinux profile install slow on large clusters | UX degradation at scale | Low | Measure timing in DIS-02; document "wait for SecurityProfileNodeStatus Installed" as best practice |
| SPO finalizer blocks profile deletion | Cleanup/CI pipeline stall | Medium | Workaround: `oc patch --type=merge -p '{"metadata":{"finalizers":null}}'`; document in IT-07 |

---

## Document History

| Version | Date | Author | Changes |
|---|---|---|---|
| 0.1 | 2026-10-05 | Test Engineering | Initial draft |
| 1.0 | 2026-10-05 | Test Engineering | Final version for approval |
| 2.0 | 2026-10-07 | Test Engineering | Restructured to IEEE 829; added Introduction subsections, Components Under Test, Testing Levels and Types, Test Environment Strategy, Document History; addressed reviewer comments |
| 2.1 | 2026-10-09 | Test Engineering | Moved Testing Tasks to §6 (before Pass/Fail Criteria) so test IDs are defined before they are referenced; addressed reviewer feedback on AD contradiction, out-of-scope duplication, upstream tests, and section ordering |

---

*End of Test Plan*
