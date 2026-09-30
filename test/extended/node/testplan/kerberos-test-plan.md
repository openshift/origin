# Test Plan: Kerberos Authentication on OpenShift (OCP 5.1)

**Version**: 1.0  
**Date**: 2026-09-30  
**Owner**: Node Team 

---

## Table of Contents

- [1. Introduction](#1-introduction)
  - [1.1 Feature Overview](#11-feature-overview)
  - [1.2 Feature Gates](#12-feature-gates)
- [2. Test Objectives](#2-test-objectives)
- [3. Test Scope](#3-test-scope)
  - [3.1 In Scope](#31-in-scope)
  - [3.2 Out of Scope](#32-out-of-scope)
- [4. Test Environment](#4-test-environment)
- [5. Test Type Classification](#5-test-type-classification)
- [6. Test Cases](#6-test-cases)
  - [6.1 Integration Tests](#61-integration-tests)
  - [6.2 E2E Functional Tests](#62-e2e-functional-tests)
  - [6.3 Negative / Error Handling Tests](#63-negative--error-handling-tests)
  - [6.4 E2E Disruptive Tests](#64-e2e-disruptive-tests)
- [7. Priority Matrix](#7-priority-matrix)
- [8. Test Execution Order](#8-test-execution-order)
- [9. Test Data Requirements](#9-test-data-requirements)
- [10. Pass / Fail Criteria](#10-pass--fail-criteria)
- [11. Risks & Mitigations](#11-risks--mitigations)
- [12. Key Discovery Tests](#12-key-discovery-tests)
- [13. Test Count Summary](#13-test-count-summary)
- [14. References](#14-references)

---

## 1. Introduction

### 1.1 Feature Overview

**Feature**: [OCPSTRAT-3418](https://redhat.atlassian.net/browse/OCPSTRAT-3418) — Provide Supported Kerberos Authentication Implementation for OpenShift Pods  
**Epic**: [OCPNODE-4776](https://redhat.atlassian.net/browse/OCPNODE-4776) — Kerberos Authentication OCP Validation Test Suite for 5.1  
**Components**: Node, RHCOS, Security & Compliance

OpenShift's default security policies (`restricted-v2` SCC, `RuntimeDefault` seccomp, `container_t` SELinux type) explicitly block the two Linux kernel features that Kerberos requires:

1. **Seccomp**: The `RuntimeDefault` profile filters the `keyctl`, `add_key`, and `request_key` syscalls — these are how MIT Kerberos (`libkrb5`) stores TGTs in the kernel keyring (`KEYRING:session:%{uid}`).
2. **SELinux**: The `container_t` type does not grant the `key` object class permissions — even if seccomp allowed the syscall, SELinux would deny the keyring operation.

The solution uses the **Security Profiles Operator (SPO)** to install custom SELinux and seccomp profiles that add **only** the Kerberos-specific permissions, then a custom SCC (`kerberos-restricted`) referencing those profiles — as locked-down as `restricted-v2` in every other respect. This approach avoids requiring privileged containers or disabling the security model.

This test plan covers the validation of the SPO-based Kerberos configuration across multi-pod deployments, StatefulSets, ticket expiry/renewal, pod restart behavior, cross-workload access, and security posture verification.

### 1.2 Feature Gates

**No feature gate is required for this feature.**

This feature does not use an OCP platform feature gate (`TechPreviewNoUpgrade`, `CustomNoUpgrade`, etc.) because it is built entirely from:

| Component | How It's Delivered | Feature Gate Needed? |
|---|---|---|
| **Security Profiles Operator** | Installed from OperatorHub (Rolling Stream Operator, not tied to OCP release) | No — it's an optional operator |
| **SelinuxProfile / SeccompProfile CRDs** | Deployed by SPO as custom resources | No — standard CRD mechanism |
| **SecurityContextConstraints** | Cluster-scoped resource created by cluster-admin | No — SCC is a GA OCP API |
| **RBAC (ClusterRole/RoleBinding)** | Standard Kubernetes RBAC | No |
| **Kernel keyring** | Native Linux kernel feature, already present in RHCOS | No |

The nearest related feature gate is `SELinuxMount` (TechPreviewNoUpgrade), but that is for CSI driver SELinux volume relabeling — unrelated to kernel keyring Kerberos operations.

---

## 2. Test Objectives

| Objective | What We're Proving |
|---|---|
| **Functional Correctness** | Kerberos TGT acquisition works via kernel keyring (`KEYRING:session`) in OCP pods using SPO profiles |
| **Security Posture** | The solution adds only the minimum required permissions (3 syscalls + SELinux key class). Everything else remains as locked-down as `restricted-v2`. |
| **Production Readiness** | Works across Deployments, StatefulSets, DaemonSets, and survives pod restarts, rolling updates, and ticket expiry |
| **Operational Safety** | Clean install, clean uninstall, no side effects on the cluster's SELinux policy or other workloads |
| **Upgrade Resilience** | Profiles and functionality survive OCP z-stream upgrades, SPO operator upgrades, and RHCOS node updates |

---

## 3. Test Scope

### 3.1 In Scope

- SPO installation and custom profile lifecycle (SELinux + seccomp)
- Custom SCC creation, RBAC binding, and SCC admission verification
- Kerberos authentication flow: `kinit` (keytab and password), `klist`, `kdestroy`
- Deployment topologies: single pod, multi-replica Deployment, StatefulSet, DaemonSet, cross-namespace
- Pod lifecycle: restart, rolling update, deletion/recreation
- Ticket lifecycle: expiry, renewal (`kinit -R`), keytab reacquisition
- Security validation: defense-in-depth (both profiles needed), SCC confinement, namespace isolation
- Error handling: wrong keytab, wrong principal, unreachable KDC, corrupted krb5.conf
- Known pitfalls from the blog: profile naming, scheduling timing, finalizer behavior
- Upgrade resilience: OCP patch, SPO upgrade, RHCOS node update
- Encryption types (AES-256, AES-128) and FIPS mode

### 3.2 Out of Scope

- **CIFS/SMB CSI Driver integration** — Owned by Storage team, depends on this feature
- **Kerberized NFS mounts** — Depends on this feature + CSI driver work
- **Active Directory integration** — Tests use a self-contained MIT KDC, not real AD
- **Automatic keytab rotation controller** — Listed as open question in OCPSTRAT-3418, not in 5.1 scope

---

## 4. Test Environment

| Item | Requirement | Source / Justification |
|---|---|---|
| **OCP Version** | 5.1 (nightly or GA) | Target release per OCPSTRAT-3418 label `Node-5.1-plan` |
| **Cluster Size** | 3 masters + 3 workers  | Node drain/reschedule tests need ≥ 2 workers |
| **Architecture** | x86_64 (primary) | Primary supported architecture. aarch64 is listed in the blog's seccomp profile (`SCMP_ARCH_AARCH64`) for forward-compatibility, but is not a confirmed test requirement for 5.1 — test if aarch64 cluster is available. |
| **SPO Version** | 0.10.0 (latest Red Hat-supported, RHSA-2026:2852) | Verified from [Red Hat OCP 4.22 docs](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/security_and_compliance/security-profiles-operator). Rolling Stream Operator via `release-alpha-rhel-8` channel. |
| **KDC** | MIT KDC deployed as a pod in-cluster | Self-contained test infra — no external AD dependency required |
| **FIPS** | Separate FIPS-enabled cluster (if available) | For encryption compatibility tests only (TC E2E-09) |
| **Base Image** | `registry.access.redhat.com/ubi9/ubi-minimal:latest` | UBI9 provides `krb5-workstation`, `keyutils-libs`, `python3` |

---

## 5. Test Type Classification

This feature is **configuration-driven** — the deliverable is a set of YAML manifests (SELinux/seccomp profiles, SCC, RBAC) and an operational procedure, not application code. This significantly affects the test type distribution:

| Type | What It Needs | What It Tests |
|---|---|---|
| **Integration** | OCP cluster + SPO. **No KDC required.** | Component interactions: SPO ↔ node, SCC admission, SELinux/seccomp enforcement, profile lifecycle, observability |
| **E2E Functional** | OCP cluster + SPO + MIT KDC pod (full stack) | Complete Kerberos authentication flow across deployment topologies, ticket lifecycle, encryption types |
| **Negative / Error** | Full stack with intentionally broken configs | Error messages, failure modes, and misconfiguration behavior |
| **E2E Disruptive** | Full stack + cluster-disruptive operations | Node reboot, drain, scale-up, OCP/SPO upgrade, RHCOS update |

**Why unit tests are absent:** Traditional unit tests isolate a function without external dependencies. This feature has no application code to unit test — the "code" is YAML resource definitions whose correctness can only be validated by applying them to a real cluster with SELinux enforcement and a running kernel.

**Classification boundary:**
- **Integration vs E2E**: Can the test pass without running `kinit` against a KDC? → Integration. Needs the full authentication flow? → E2E.
- **E2E Functional vs Disruptive**: Does it reboot, drain, upgrade nodes, or corrupt policy? → Disruptive (separate CI job, longer runtime).

---

## 6. Test Cases

> Detailed manual steps, commands, and expected output for each test are in the [Test Execution Runbook].

### 6.1 Integration Tests

> OCP cluster + SPO installed. **No KDC required.**

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| IT-01 | SPO Installation and Profile Deployment | Verify SPO installs correctly, both SELinux and seccomp profiles deploy to all workers with correct status and timing | P0 | To be automated |
| IT-02 | SCC and RBAC Configuration | Verify `kerberos-restricted` SCC matches `restricted-v2` security posture and is bound only to a dedicated SA | P0 | To be automated |
| IT-03 | Profile Verification — keyctl and SELinux Label | Prove both profiles are active: `keyctl` syscall succeeds AND SELinux label shows custom type (not `container_t`) | P0 | To be automated |
| IT-04 | Defense in Depth — Both Profiles Required | Prove neither profile alone is sufficient — `restricted-v2`, seccomp-only, and SELinux-only all fail with documented errno | P0 | To be automated |
| IT-05 | SCC Authorization and Pod Confinement | Unauthorized SAs cannot use the SCC; Kerberos pods cannot escalate beyond keyring operations | P0 | To be automated |
| IT-06 | Namespace Isolation | SPO profiles are namespace-scoped — pods in namespace-B cannot use profiles from namespace-A | P2 | To be automated |
| IT-07 | Profile Lifecycle Pitfalls | Validate blog pitfalls: premature scheduling (CRI-O error), same-named profile collision, finalizer behavior on deletion | P2 | Manual |
| IT-08 | Observability — AVC and Seccomp Logging | SELinux AVC denials and seccomp violations visible in kernel logs; SPO ProfileRecording captures Kerberos syscalls | P2 | Manual |

### 6.2 E2E Functional Tests

> Full stack required: OCP cluster + SPO + MIT KDC pod.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| E2E-01 | Single Pod Kerberos Authentication | Complete Kerberos lifecycle: `kinit` (keytab + password), `klist`, `kdestroy`, clean audit log | P0 | To be automated |
| E2E-02 | Multi-Replica Deployment with Keyring Isolation | Multiple replicas authenticate independently with isolated per-session kernel keyrings; scale-up/down works | P1 | To be automated |
| E2E-03 | StatefulSet Kerberos Authentication | Ordered lifecycle + Kerberos works; replacement pods reacquire tickets via keytab | P1 | To be automated |
| E2E-04 | Cross-Workload and Cross-Namespace Access | Multiple workloads in same/different namespaces independently authenticate; DaemonSet topology works | P1 | To be automated |
| E2E-05 | Pod Restart, Update, and Recreation | Tickets are lost on all lifecycle events (expected); keytab reacquisition always succeeds | P1 | To be automated |
| E2E-06 | Init Container and Sidecar Keyring Sharing | **Discovery:** determine if init/sidecar keyrings are visible to main container — drives renewal architecture recommendation | P2 | Manual |
| E2E-07 | Ticket Expiry, Renewal, and Reacquisition | `kinit -R` works before expiry, fails after; keytab always recovers; error messages documented | P1 | To be automated |
| E2E-08 | Scheduler Placement Without Node Pinning | Kerberos works without explicit `nodeName` — scheduler places pod on any profile-ready node | P1 | To be automated |
| E2E-09 | Encryption Types and FIPS Compatibility | AES-256 and AES-128 both work; FIPS cluster behavior documented | P3 | Manual |

### 6.3 Negative / Error Handling Tests

> Verify clear, actionable error messages for misconfigurations.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| NEG-01 | Invalid Keytab, Principal, and Config Errors | Clear errors for wrong keytab, wrong principal, corrupted krb5.conf, missing keytab file | P2 | To be automated |
| NEG-02 | KDC Unreachable and Expired Keytab | Clear errors for network failure (`Cannot contact any KDC`) and stale keytab (`Preauthentication failed`) | P2 | To be automated |
| NEG-03 | subPath Mount with Optional Nonexistent Volume | CRI-O rejects `subPath` + `optional: true` for missing Secret — confirms blog pitfall | P2 | Manual |

### 6.4 E2E Disruptive Tests

> ⚠️ Node-level or cluster-level operations. Dedicated CI jobs only.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| DIS-01 | Node Reboot and Drain Recovery | Profiles survive reboot; Kerberos works after drain/reschedule to another node | P3 | Manual |
| DIS-02 | New Node Auto-Profile Installation | SPO auto-installs profiles on a newly added worker; installation timing documented | P3 | Manual |
| DIS-03 | OCP, SPO, and RHCOS Upgrade Resilience | Kerberos setup survives OCP z-stream, SPO operator, and RHCOS node updates — zero regression | P3 | Manual |
| DIS-04 | SELinux Profile Naming Collision | **Discovery:** naming a SelinuxProfile "kerberos" overwrites system module — confirm blog warning and recovery | P3 | Manual |

---

## 7. Priority Matrix

| Priority | Tests | Test Types | Rationale |
|---|---|---|---|
| **P0 — Must Pass** | IT-01, IT-02, IT-03, IT-04, IT-05, E2E-01 | Integration, E2E | Core setup, profile verification, security model, single-pod Kerberos. If these fail, nothing works. |
| **P1 — High** | E2E-02, E2E-03, E2E-04, E2E-05, E2E-07, E2E-08 | E2E | Production scenarios: multi-pod, StatefulSet, cross-workload, lifecycle, ticket expiry. Production readiness. |
| **P2 — Medium** | IT-06, IT-07, IT-08, E2E-06, NEG-01, NEG-02, NEG-03 | Integration, Negative, E2E | Namespace isolation, pitfalls, observability, discovery tests, error handling. Important for documentation and supportability. |
| **P3 — Low** | E2E-09, DIS-01, DIS-02, DIS-03, DIS-04 | E2E, Disruptive | Encryption/FIPS, upgrade resilience, disruptive operations. Longer-term stability and compliance. |

---

## 8. Test Execution Order

| Phase | Test IDs | Type | Gate |
|---|---|---|---|
| 1 | IT-01, IT-02 | Integration | Infrastructure setup verified |
| 2 | IT-03, IT-04, IT-05 | Integration | Profiles and security model validated |
| 3 | E2E-01 | E2E | Single pod Kerberos works |
| 4 | E2E-02, E2E-03, E2E-04 | E2E | Deployment topologies work |
| 5 | E2E-05, E2E-06 | E2E | Lifecycle events and discovery |
| 6 | E2E-07, E2E-08 | E2E | Ticket lifecycle and scheduling |
| 7 | IT-06, IT-07, IT-08 | Integration | Isolation, pitfalls, observability |
| 8 | NEG-01, NEG-02, NEG-03 | Negative | Error handling |
| 9 | E2E-09 | E2E | Encryption and FIPS |
| 10 | DIS-01, DIS-02 | Disruptive | Node operations |
| 11 | DIS-03, DIS-04 | Disruptive | Upgrades and naming collision |

**Phases 1–3 are P0 blockers** — do not proceed to later phases if these fail.

---

## 9. Test Data Requirements

| Data | Source | Notes |
|---|---|---|
| MIT KDC | Pod in-cluster (`kdc-infra` namespace) | Realm: `TEST.LOCAL`, self-contained, privileged SCC for `kdb5_util` |
| Test principal | `testuser@TEST.LOCAL` | Created by KDC setup script |
| Service principal | `svc/testhost.test.local@TEST.LOCAL` | For service ticket tests |
| Keytab | Exported from KDC pod via `kadmin.local ktadd` | Stored as Kubernetes Secret |
| krb5.conf | ConfigMap | Points to KDC Service DNS: `kdc-service.kdc-infra.svc.cluster.local` |
| Bad keytab | Keytab for svc principal (mismatched with testuser) | For NEG-01 |
| Bad krb5.conf | Syntactically invalid config | For NEG-01 |
| Short-lived krb5.conf | `ticket_lifetime=5m, renew_lifetime=30m` | For E2E-07 |
| Test container image | UBI9-minimal + `krb5-workstation` + `keyutils-libs` + `python3` | Custom Dockerfile |

---

## 10. Pass / Fail Criteria

The feature is validated for OCP 5.1 when **all** of the following conditions are met:

1. **No critical or major defects remain open** for the SPO profile deployment, SCC configuration, kernel keyring access path, or downstream e2e automation.

2. **Integration tests IT-01 through IT-08 pass consistently.** SPO v0.10.0 installs, both profiles reach `Installed` on every worker via `SecurityProfileNodeStatus`, and the `kerberos-restricted` SCC is correctly bound only to the dedicated ServiceAccount.

3. **A Kerberos-enabled pod with the correct SCC, SELinux profile, and seccomp profile is admitted by the API server, scheduled to a node, and successfully executes `kinit`/`klist`/`kdestroy` using `KEYRING:session` credential cache** (E2E-01). A pod missing either the SELinux profile or the seccomp profile, or running under a different SCC, is blocked from `keyctl` operations with `EPERM` or `EACCES` (IT-04, IT-05).

4. **Defense-in-depth is confirmed:** neither profile alone is sufficient for kernel keyring access. The seccomp profile blocks the `keyctl` syscall (`EPERM` under `restricted-v2`), the SELinux policy blocks `key` class operations (AVC denial under `container_t`), and only the combination of both profiles grants access (IT-03, IT-04). The custom SCC remains equivalent to `restricted-v2` in all other security posture fields — no privilege escalation, no host access, `ALL` capabilities dropped (IT-02).

5. **Multi-pod, StatefulSet, and cross-workload deployments authenticate independently** with isolated per-session kernel keyrings. Scaling up/down preserves existing tickets and new pods reacquire via keytab (E2E-02, E2E-03, E2E-04). Pod replacement, rolling update, and process restart all lose cached tickets, and keytab-based reacquisition always succeeds (E2E-05).

6. **Negative test scenarios produce clear, actionable error messages** for wrong principal (`Keytab contains no suitable keys`), missing keytab (`No such file or directory`), unreachable KDC (`Cannot contact any KDC for realm`), stale keytab after password rotation (`Preauthentication failed`), and subPath mount with optional nonexistent secret (`Not a directory` from CRI-O) (NEG-01, NEG-02, NEG-03). All error scenarios exit non-zero and do not cause CrashLoopBackOff in the application pod.

7. **On node reboot**, SPO profiles remain `Installed` on the recovered node and `kinit` succeeds without manual re-deployment (DIS-01). On **node drain**, the rescheduled pod lands on another node where profiles are already installed and reacquires a ticket via keytab (DIS-01). On a **new node** added via MachineSet scale-up, SPO auto-installs both profiles without operator intervention, and the installation timing is documented (DIS-02).

8. **OCP z-stream upgrade, SPO operator upgrade, and RHCOS node update** (MachineConfig-triggered) all preserve profile installation, SCC binding, and Kerberos functionality. Zero regression across all upgrade types (DIS-03).

9. **Discovery tests E2E-06 and DIS-04 produce documented findings** with exact command output and architectural recommendations. E2E-06 determines whether init-container and sidecar kernel keyrings are shared with the main container (which drives the recommended ticket renewal architecture). DIS-04 confirms the blog's warning about SELinux profile naming collision with the system `kerberos` module and documents recovery behavior.

10. **Ticket expiry and renewal behave as documented:** `kinit -R` succeeds within the renewable lifetime, fails after ticket expiration with a clear error, and keytab-based `kinit` always recovers regardless of ticket state (E2E-07). AES-256 and AES-128 encryption types both work; FIPS cluster behavior is documented if a FIPS environment is available (E2E-09).

11. **Downstream e2e scenarios IT-01 through DIS-04 are tracked in JIRA** ([OCPNODE-4776](https://redhat.atlassian.net/browse/OCPNODE-4776) stories OCPNODE-4777 through OCPNODE-4785), implemented in the test runbook, and passing in downstream CI on OCP 5.1.

12. **Supporting test results, configuration examples (SELinux profile, seccomp profile, SCC, krb5.conf, keytab Secret), and discovery findings are available** for the Docs team to produce the Kerberos-on-OpenShift documentation and KCS article.

### Failure Handling

| Condition | Action |
|---|---|
| Any P0 test (IT-01 through IT-05, E2E-01) fails | **Stop testing.** File a blocker bug against the failing component (SPO, SCC, CRI-O, or kubelet). Root-cause before continuing. Feature is **not validated**. |
| 1–2 P1 tests fail with a documented workaround | Feature is validated with **known limitations**. File high-priority bugs. Document workarounds in the blog/KCS. |
| 3+ P1 tests fail, or any P1 failure has no workaround | Feature is **not validated**. Escalate to feature owner for a fix-or-defer decision. |
| P2/P3 failure | Continue testing. File normal-priority bug. Include in test report as a known issue. |
| A previously passing test fails on re-run | **Regression.** Automatically escalate one priority level (P2 → P1, P1 → P0). |
| Discovery test (E2E-06, DIS-04) | No pass/fail verdict. Record exact output and architectural recommendation. Findings feed into documentation. |

---

## 11. Risks & Mitigations

| Risk | Impact | Probability | Mitigation |
|---|---|---|---|
| SPO not available in OCP 5.1 OperatorHub | Blocker — entire test suite cannot run | Low | Verify SPO availability in first 5.1 nightly. SPO is a Rolling Stream operator, not tied to OCP releases. |
| MIT KDC pod needs root to run `kdb5_util` | Test infra only | Low | Use privileged SCC for KDC pod. This is test infrastructure, not the system under test. |
| Sidecar keyring not shared with main container | Limits the recommended renewal architecture | Medium | Discovery test E2E-06 determines this. If not shared, document in-app renewal as the recommended pattern. |
| FIPS mode blocks some Kerberos enctypes | Compliance gap | Medium | Test early (E2E-09). Document which enctypes work in FIPS. This may require coordination with the security team. |
| SELinux profile install slow on large clusters (50+ nodes) | UX issue | Low | Measure timing (DIS-02). Document "wait for SecurityProfileNodeStatus Installed" as best practice. |
| SPO finalizer hangs on profile deletion | Cleanup/CI issue | Medium | Document `oc patch --type=merge -p '{"metadata":{"finalizers":null}}'` workaround (IT-07). |
| aarch64 cluster not available for testing | Cannot verify multi-arch | Medium | aarch64 comes from blog's seccomp profile forward-compatibility. Mark aarch64 as "test if available", not a blocker. |

---

## 12. Key Discovery Tests

These tests produce documented findings rather than pass/fail verdicts:

| Test | Question Being Answered | Impact on Architecture |
|---|---|---|
| **E2E-06** (Init container) | Does an init container's kernel keyring carry to the main container? | Determines if the init-container kinit pattern is viable for users |
| **E2E-06** (Sidecar) | Do sidecar and main container share the kernel keyring? | Determines the recommended ticket renewal architecture (sidecar loop vs in-app kinit) |
| **E2E-07** (Expiry observation) | What exactly happens when a ticket expires in the kernel keyring? | Determines what applications should expect and handle (error type, cache cleanup behavior) |
| **DIS-04** (Naming collision) | How badly does naming a profile "kerberos" break the system? | Determines severity of the blog warning and whether to add a validating webhook |

---

## 13. References

| # | Reference | Link |
|---|---|---|
| 1 | Feature: Provide supported Kerberos authentication for OpenShift pods | [OCPSTRAT-3418](https://redhat.atlassian.net/browse/OCPSTRAT-3418) |
| 2 | Epic: Kerberos Authentication OCP Validation Test Suite for 5.1 | [OCPNODE-4776](https://redhat.atlassian.net/browse/OCPNODE-4776) |
| 3 | Peter Hunt's Blog: Running Kerberos Workloads on OpenShift with SPO | [hackmd.io](https://hackmd.io/@haircommander/SyoDBaEwMg) |
| 4 | SPO 0.10.0 Release Notes (RHSA-2026:2852) | [Red Hat Docs — OCP 4.22 SPO](https://docs.redhat.com/en/documentation/openshift_container_platform/4.22/html/security_and_compliance/security-profiles-operator) |
| 5 | SELinux kerberos_container template | [SELINUX-3234](https://redhat.atlassian.net/browse/SELINUX-3234) |
| 6 | Upstream RFEs this feature unblocks | [RFE-4794](https://redhat.atlassian.net/browse/RFE-4794), [RFE-1673](https://redhat.atlassian.net/browse/RFE-1673), [RFE-7275](https://redhat.atlassian.net/browse/RFE-7275) |

