# Test Plan: Kerberos Authentication on OpenShift (OCP 5.1)

**Test Plan ID**: TP-OCPNODE-4776-01  
**Version**: 3.1  
**Date**: 2026-11-20  
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

- **Seccomp**: `RuntimeDefault` filters `keyctl`, `add_key`, and `request_key` — the syscalls MIT Kerberos uses to store TGTs in the kernel keyring (`KEYRING:persistent:%{uid}`).
- **SELinux**: `container_t` does not grant the `key` object class permissions. Even if seccomp allowed the syscall, SELinux would deny the keyring operation.

The solution uses the **Security Profiles Operator (SPO)** to deploy custom SELinux and seccomp profiles that grant only the Kerberos-specific permissions, combined with a custom SCC (`kerberos-restricted`) that is otherwise equivalent to `restricted-v2`. This avoids privileged containers or weakening the overall security model.

**Credential cache (validated):** use `KEYRING:persistent:%{uid}`. In the OpenShift single-pod run, `KEYRING:session:%{uid}` failed with `Permission denied while getting default ccache` (clean audit log — not an SPO/SELinux/seccomp denial). A Linux session keyring can exist without PAM (e.g. systemd `KeyringMode=private`); that does **not** change the supported OpenShift path, which is `KEYRING:persistent`.

**Keyring isolation (validated):** `KEYRING:persistent` is looked up **per UID within the caller's Linux user namespace** (not a global host-wide table keyed only by numeric UID). Without `hostUsers: false`, OpenShift pods in a namespace typically share the host user namespace and reuse the same container UID, so same UID + same node → shared keyring (ticket read-sharing, overwrite, destroy cross-talk, stale inheritance after delete). There is no Kerberos/`krb5.conf` fix. The supported mitigation is pod-level user namespaces: **`spec.hostUsers: false`** (each pod gets its own user namespace / keyring register). Optional admin enforcement: SCC `userNamespaceLevel: RequirePodLevel` as the **only** usable Kerberos SCC for the workload SA.

No feature gate is required — the solution uses only GA OCP APIs (SCC, RBAC, CRDs) combined with the Security Profiles Operator installed from OperatorHub as a Rolling Stream operator independent of the OCP release.

### 1.1 Purpose

This document defines the test plan for validating Kerberos authentication support in OpenShift Container Platform 5.1. It describes what is tested, how it is tested, the pass/fail criteria, and the environment required to execute the test suite.

### 1.2 Scope

This plan covers the SPO-based Kerberos authentication solution on OCP 5.1, including:

- Security Profiles Operator profile deployment and lifecycle
- Custom SCC (`kerberos-restricted` and userns-compatible Kerberos SCC) admission and RBAC controls
- Kernel keyring access from containers (SELinux + seccomp)
- Full Kerberos authentication flow (`kinit`/`klist`/`kdestroy`) across supported deployment topologies
- Multi-replica keyring isolation via `hostUsers: false` (and optional `RequirePodLevel` mandate)
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
- Test `KEYRING:persistent`-based credential cache mechanism (not `KEYRING:session`)
- Validate multi-replica isolation with `hostUsers: false` and optional SCC `RequirePodLevel` enforcement
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
| **SELinux Policy** | Container access to kernel keyring | `KEYRING:persistent:%{uid}` accessible under the custom SELinux type (not `container_t`) |
| **Security Context Constraints** | `kerberos-restricted` SCC | Kerberos-enabled SCC equivalent to `restricted-v2` plus SELinux type + seccomp overrides (baseline single-pod path) |
| **Security Context Constraints** | Userns-compatible Kerberos SCC | Same posture with `fsGroup: RunAsAny` (or map-compatible GID), `runAsUser` compatible with the pod userns map (see below), and optional `userNamespaceLevel: RequirePodLevel` for multi-replica isolation |
| **Security Context Constraints** | Seccomp profile (`keyctl`, `add_key`, `request_key`) | SPO-managed seccomp profile allowing the three Kerberos kernel syscalls |
| **Credential Cache Infrastructure** | `KEYRING:persistent:UID` | Persistent kernel keyring credential cache; survives process boundaries for the same UID within a user namespace |
| **Credential Cache Infrastructure** | `FILE`-based cache (`/var/lib/kubelet/kerberos/`) | File-based TGT storage alternative (future scope — required for CSI driver integration) |
| **User namespaces** | `spec.hostUsers: false` | Gives each pod its own Linux user namespace (and thus its own persistent-keyring register) — required for KEYRING isolation across replicas. Default map size is typically container UIDs `0–65535`; OpenShift namespace ranges like `1000000000/10000` are **not** usable as container `runAsUser` under that map |
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
- **Multi-replica KEYRING isolation** with `hostUsers: false` (destroy / overwrite / post-delete independence)
- Optional SCC `RequirePodLevel` mandate (rejects missing `hostUsers: false` when that SCC is the sole usable path)
- Pod lifecycle: pod delete/replace and rolling image update lose tickets; in-pod container restart may **retain** `KEYRING:persistent` tickets; keytab reacquisition always succeeds after empty cache
- Ticket lifecycle: expiry, renewal (`kinit -R`), keytab reacquisition
- Init-container / sidecar keyring sharing behavior under `KEYRING:persistent`
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

**Automation assert rules (multi-replica):** do **not** treat `kinit` exit 0 alone as pass. After `kinit`, assert `klist` reports `Ticket cache: KEYRING:persistent:` in **each** replica (a `FILE:` ccache can pass isolation checks without exercising the kernel keyring). Also assert destroy-independence, overwrite-independence, and empty cache after pod replace. Matching `Valid starting` timestamps alone are not failures when `kinit` is near-simultaneous.

### 5.1 Testing Levels

| Level | Description | Coverage | Priority | Status |
|---|---|---|---|---|
| **Unit** | Not applicable — feature is YAML/configuration-driven with no isolatable application code | None | — | N/A |
| **Integration** | Component interactions: SPO ↔ node, SCC admission, SELinux/seccomp enforcement, profile lifecycle, userns mandate. No KDC required for most IT cases. | SPO install, profile deployment, SCC/RBAC, defense-in-depth, namespace boundary, lifecycle pitfalls, observability, RequirePodLevel | P0–P2 | To be automated / Manual |
| **System** | Full-stack tests: OCP + SPO + MIT KDC + cluster operations. End-to-end Kerberos, deployment topologies, ticket lifecycle, disruptive scenarios. | Single/multi-pod auth, StatefulSet, DaemonSet, pod lifecycle, ticket expiry/renewal, negative error cases, node reboot/drain, upgrade resilience | P0–P3 | To be automated / Manual |
| **Acceptance** | Validates the feature meets its P0 definition of done from a user perspective: profile installation, SCC admission, successful `kinit` on both multi-node and SNO. | Core SPO + SCC setup and single-pod Kerberos end-to-end | P0 | To be automated |

### 5.2 Testing Types

| Type | Description | Coverage | Priority | Status |
|---|---|---|---|---|
| **Functional** | Core Kerberos authentication flow, SCC and profile behavior, deployment topologies, pod and ticket lifecycle | SPO install, SCC admission, `kinit`/`klist`/`kdestroy`, multi-pod, StatefulSet, pod restart, ticket renewal, scheduler placement | P0–P1 | To be automated |
| **Security** | Defense-in-depth (both profiles required), SCC confinement, namespace isolation, keyring isolation under userns, privilege escalation prevention | Profile verification, SCC posture, unauthorized SA rejection, userns isolation, error handling for misconfigurations | P0–P2 | To be automated |
| **Compatibility** | x86_64 and aarch64 architectures, AES-256 and AES-128 encryption types, FIPS-enabled clusters | Encryption type validation, FIPS cluster behavior | P3 | Manual |
| **Upgrade** | OCP z-stream, SPO operator upgrade, RHCOS node update — zero regression across all upgrade paths | Profile and SCC survival across OCP/SPO/RHCOS updates | P3 | Manual |
| **Regression** | Once automated , P0–P1 tests are intended to run in OCP CI (Prow) nightly / pre-merge jobs to catch regressions | Core setup, single-pod Kerberos, production topologies, lifecycle, userns isolation | P0–P1 |  To be automated — no dedicated Kerberos CI job today |

**Classification rules:**
- Can the test pass without running `kinit` against a KDC? → Integration
- Needs the full authentication flow? → System / E2E
- Reboots, drains, upgrades nodes, or corrupts policy? → Disruptive (separate CI job)

---

## 6. Testing Tasks

### Test Cases

#### 6.1 Integration Tests
> OCP + SPO installed. No KDC required (except where noted).

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| IT-01 | SPO Installation and Profile Deployment | SPO installs correctly; both SELinux and seccomp profiles deploy to all workers with correct status and timing | P0 | Manual PASS ; To be automated |
| IT-02 | SCC and RBAC Configuration | `kerberos-restricted` SCC matches `restricted-v2` security posture; bound only to dedicated SA | P0 | Manual PASS (baseline); To be automated |
| IT-03 | Profile Verification — keyctl and SELinux Label | Both profiles active: `keyctl` syscall succeeds AND SELinux label shows custom type (not `container_t`) | P0 | Manual PASS ; To be automated |
| IT-04 | Defense in Depth — Both Profiles Required | Neither profile alone is sufficient — `restricted-v2`, seccomp-only, and SELinux-only all fail with documented errno | P0 | To be automated |
| IT-05 | SCC Authorization and Pod Confinement | Unauthorized SAs cannot use the SCC; Kerberos pods cannot escalate beyond keyring operations | P0 | Partial manual; To be automated |
| IT-06 | ProfileBinding Namespace Boundary | ProfileBinding behavior for namespace-B workloads, including binding namespace and namespace enablement | P2 | To be automated |
| IT-07 | Profile Lifecycle Pitfalls | Premature scheduling (CRI-O error), same-named profile collision, finalizer behavior on deletion | P2 | Manual |
| IT-08 | Observability — AVC and Seccomp Logging | AVC denials and seccomp violations visible in kernel logs; SPO ProfileRecording captures Kerberos syscalls | P2 | Partial manual (clean audit on success path); Manual |

#### 6.1.1 User-namespace / KEYRING isolation (Integration + System)

> Requires kernel ≥6.3, CRI-O ≥1.25, crun ≥1.9 / runc ≥1.2. Multi-replica KEYRING tests **must** use `hostUsers: false` and a userns-compatible SCC: `fsGroup: RunAsAny` (or map-compatible GID); `runAsUser: RunAsAny` with workload UID inside the userns map (validated: `1000`), **or** `MustRunAsRange` constrained to UIDs within that map (typically `0–65535`) — not the default OpenShift namespace range (`1000000000/10000`). Pin `nodeName` for deterministic same-node checks.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| IT-USNS-01 | Same UID + same node + `hostUsers: false` | Distinct `/proc/1/uid_map` host ranges; no read-sharing; destroy-independent tickets | P0 | Manual PASS; To be automated |
| IT-USNS-02 | Pod delete/recreate under userns | Replacement `klist` empty before `kinit` (no stale inheritance) | P0 | Manual PASS; To be automated |
| IT-USNS-03 | Overwrite under userns | Sequential `kinit` in two pods; each keeps own ticket after the other kinits | P1 | Manual PASS; To be automated |
| IT-USNS-04 | Sidecar multi-replica + userns | Destroy/overwrite in one pod does not affect the other (sidecar alone is insufficient) | P1 | Manual PASS; To be automated |
| IT-USNS-05 | SCC `RequirePodLevel` rejects missing `hostUsers` | Create as workload SA → Forbidden (`Host Users must be set to false`); must be sole usable Kerberos SCC | P1 | Manual PASS; To be automated |
| IT-USNS-06 | SCC `RequirePodLevel` admits with `hostUsers: false` | Admitted under intended SCC; Running | P1 | Manual PASS; To be automated |
| NEG-USNS-01 | Competing permissive Kerberos SCC still bound | Mandate ineffective (document as negative / config error) | P2 | Manual PASS; To be automated |
| NEG-UID-01 | Same UID + same node **without** userns | Expect collision symptom (read-share, overwrite, destroy cross-talk, or stale inheritance) | P1 | Manual PASS (OCPNODE-4838/4843); To be automated |

#### 6.2 E2E Functional Tests
> Full stack: OCP + SPO + MIT KDC pod. Multi-replica topologies use `hostUsers: false` unless testing NEG-UID-01.

| Test ID | Scenario | Objective | Priority | Status |
|---|---|---|---|---|
| E2E-01 | Single Pod Kerberos Authentication | Complete lifecycle: `kinit` (keytab + password), `klist`, `kdestroy`, clean audit log; `KEYRING:persistent` | P0 | Manual PASS ; To be automated |
| E2E-02 | Multi-Replica Deployment with Keyring Isolation | Replicas authenticate independently with **isolated** keyrings under `hostUsers: false`; scale-up/down works; assert destroy-independence | P1 | Manual PASS ; To be automated |
| E2E-03 | StatefulSet Kerberos Authentication | Ordered lifecycle + Kerberos under userns; replacement pods start empty then reacquire via keytab; siblings unaffected | P1 | Manual PASS (OCPNODE-4780); To be automated |
| E2E-04 | Cross-Workload and Cross-Namespace Access | Multiple workloads in same/different namespaces authenticate independently under userns; DaemonSet works | P1 | Manual PASS for Deployments cross-NS ; DaemonSet TBD; To be automated |
| E2E-05 | Pod Restart, Update, and Recreation | With `hostUsers: false`, **pod delete/replace** and **rolling image update**: ticket lost; keytab reacquisition succeeds. Without userns, same-UID replacement on the same node may inherit a stale ticket (NEG-UID-01). **In-pod container restart** (liveness): with `KEYRING:persistent`, ticket may **survive** (same userns UID map) — document actual state; keytab re-`kinit` still succeeds | P1 | Manual PASS with documented finding (OCPNODE-4781); To be automated |
| E2E-06 | Init Container and Sidecar Keyring Sharing | Document actual sharing: under `KEYRING:persistent` + same pod userns + **same UID** for init and main, init→main **shares** the ticket (validated). Different UIDs → different per-UID keyrings (no share). Multi-replica sidecar still requires `hostUsers: false` between pods | P2 | Manual findings recorded (OCPNODE-4781); To be automated |
| E2E-07 | Ticket Expiry, Renewal, and Reacquisition | Assert `renew until` is in the future before `kinit -R`; renewal works before expiry. After expiry, wait past the configured Kerberos `clockskew` grace period (MIT default `300s`) before requiring `kinit -R` to fail — an immediate post-expiry renew can still succeed without a defect. Keytab always recovers; error messages documented. Requires KDC `max_renewable_life` for the realm (see Test Data) | P1 | Manual PASS ; To be automated |
| E2E-08 | Scheduler Placement Without Node Pinning | Kerberos works without explicit `nodeName` — scheduler places pod on any profile-ready node | P1 | Manual PASS ; To be automated |
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
| DIS-01 | Node Reboot and Drain Recovery | Profiles survive reboot; Kerberos works after drain/reschedule | P3 | Partial manual ; reboot TBD |
| DIS-02 | New Node Auto-Profile Installation | SPO auto-installs profiles on new worker; timing documented | P3 | Manual |
| DIS-03 | OCP, SPO, and RHCOS Upgrade Resilience | Kerberos survives OCP z-stream, SPO operator, and RHCOS updates — zero regression | P3 | Manual |
| DIS-04 | SELinux Profile Naming Collision | Discovery: naming a SelinuxProfile "kerberos" overwrites system module — confirm and document recovery | P3 | Manual |

### Execution Order

| Phase | Test IDs | Type | Gate |
|---|---|---|---|
| 1 | IT-01, IT-02 | Integration | Infrastructure verified |
| 2 | IT-03, IT-04, IT-05 | Integration | Profiles and security model validated |
| 3 | E2E-01 | E2E | Single-pod Kerberos works (`KEYRING:persistent`) |
| 4 | IT-USNS-05, IT-USNS-06, NEG-USNS-01 | Integration | Userns SCC mandate understood |
| 5 | NEG-UID-01, IT-USNS-01–04 | Integration / E2E | Collision without userns known; isolation with userns proven |
| 6 | E2E-02, E2E-03, E2E-04 | E2E | Deployment topologies work **with** `hostUsers: false` |
| 7 | E2E-05, E2E-06 | E2E | Lifecycle and init/sidecar findings |
| 8 | E2E-07, E2E-08 | E2E | Ticket lifecycle and scheduling |
| 9 | IT-06, IT-07, IT-08 | Integration | Namespace boundary, pitfalls, observability |
| 10 | NEG-01, NEG-02, NEG-03 | Negative | Error handling |
| 11 | E2E-09 | E2E | Encryption and FIPS |
| 12 | DIS-01, DIS-02 | Disruptive | Node operations |
| 13 | DIS-03, DIS-04 | Disruptive | Upgrades and naming collision |

**Phases 1–3 are P0 blockers.** Do not proceed to multi-replica E2E without IT-USNS-01 (or equivalent isolation proof).

---

## 7. Item Pass/Fail Criteria

The feature is validated for OCP 5.1 when **all** of the following are met:

1. No critical or major defects remain open for SPO profile deployment, SCC configuration, kernel keyring access, keyring isolation, or downstream automation.

2. IT-01–IT-08 pass consistently: SPO installs, both profiles reach `Installed` on every worker via `SecurityProfileNodeStatus`, Kerberos SCC is bound only to the intended SA(s).

3. A pod with the correct SCC, SELinux profile, and seccomp profile successfully executes `kinit`/`klist`/`kdestroy` using `KEYRING:persistent` (E2E-01). `KEYRING:session` is **not** the supported OpenShift path — the single-pod run failed with `Permission denied while getting default ccache` (clean audit log; not SPO/SELinux/seccomp). A pod missing either profile is blocked with `EPERM` or `EACCES` (IT-04, IT-05).

4. Defense-in-depth confirmed: seccomp-only → syscall denied with the errno generated by the enforcing SELinux policy; SELinux-only → syscall denied with the errno generated by the configured seccomp profile; restricted-v2 → Kerberos keyring operation denied. Only the combination of both profiles succeeds (IT-03, IT-04). Exact errno values are to be documented after IT-04 runs on the target OCP 5.1 / RHCOS 10 environment. Baseline SCC is equivalent to `restricted-v2` except for SELinux type + seccomp (IT-02). Userns SCC additionally requires `fsGroup` and `runAsUser` compatibility with the pod userns map and may set `RequirePodLevel`.

5. Multi-pod, StatefulSet, and cross-workload deployments authenticate independently with **isolated** keyrings under `hostUsers: false` (E2E-02–E2E-04, IT-USNS-01–04); each replica’s `klist` shows `KEYRING:persistent:`. Without userns, same UID + same node collides (NEG-UID-01). With `hostUsers: false`, **pod replacement** and **rolling updates** lose tickets; keytab reacquisition always succeeds (E2E-05). In-pod container restart may retain `KEYRING:persistent` tickets — document, do not treat as failure.

6. Negative scenarios produce clear, specific error messages and exit non-zero; no CrashLoopBackOff (NEG-01–NEG-03).

7. Profiles survive node reboot; `kinit` succeeds after drain/reschedule (DIS-01). SPO auto-installs profiles on new workers (DIS-02).

8. OCP z-stream, SPO operator, and RHCOS updates preserve profiles and Kerberos functionality; zero regression (DIS-03).

9. Discovery / architecture tests E2E-06 and DIS-04 produce documented findings with command output and recommendations. Validated (E2E-06): init-container `kinit` **is** visible to the main container under `KEYRING:persistent` + same pod userns + **same UID**; different UIDs do not share; multi-replica sidecar still requires `hostUsers: false` between pods.

10. Before `kinit -R`, `klist` shows a future `renew until` (KDC `max_renewable_life` set for the realm). Renewal works before expiry; after expiry, failure is asserted only after waiting past `clockskew` (default `300s`). Keytab recovery always works (E2E-07). AES-256 and AES-128 both work; FIPS behavior documented (E2E-09).

11. Optional mandate path: IT-USNS-05/06 and NEG-USNS-01 document that `RequirePodLevel` only enforces `hostUsers: false` when that SCC is the sole usable Kerberos SCC for the SA.

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

- Any P0 test fails — the environment or feature is broken; further results are unreliable.
- SPO is unavailable in OCP 5.1 OperatorHub — prerequisite for all tests.
- The in-cluster MIT KDC pod fails to start — E2E tests cannot run without a KDC.
- Cluster cannot run `hostUsers: false` (kernel/CRI-O/runtime too old) — multi-replica KEYRING isolation tests cannot pass.

### Resumption

Testing resumes when:

- The root cause of the P0 failure is identified and fixed, and the affected test passes on a clean re-run.
- SPO is confirmed available in the target OCP build.
- The KDC pod is healthy and principals are accessible.
- User-namespace prerequisites are met for multi-replica tests.

---

## 9. Test Deliverables

| Deliverable | Description |
|---|---|
| This test plan | `test/extended/node/testplan/kerberos-test-plan.md` |
| Test runbook | Manual execution steps, exact commands, and expected output for each test ID  |
| Automated test suite | Ginkgo `.go` files under `test/extended/node/` for P0 and P1 tests |
| Test execution report | Pass/fail results per test ID, environment details, defects filed |
| Discovery findings | Documented output and recommendations for E2E-06 and DIS-04 |
| Configuration examples | Working YAML for `SelinuxProfile`, `SeccompProfile`, `kerberos-restricted` / userns SCC, `krb5.conf` ConfigMap (`KEYRING:persistent`), keytab Secret, `hostUsers: false` workloads |

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
| User namespaces | Kernel ≥6.3, CRI-O ≥1.25, crun ≥1.9 / runc ≥1.2 for `hostUsers: false` |

### 10.2 SNO vs Multi-Node

| Scenario | Multi-Node | SNO |
|---|---|---|
| SPO + profiles + SCC + `kinit` (P0) | Required | Required |
| Multi-replica / StatefulSet + userns | Yes | Yes |
| Node drain to second worker (DIS-01) | Yes | N/A |
| New worker auto-install (DIS-02) | Yes | N/A |
| Node reboot (DIS-01) | Yes | Yes |

### 10.3 Test Data

| Data Item | Details |
|---|---|
| MIT KDC | In-cluster pod, realm `TEST.LOCAL`, privileged SCC for `kdb5_util`; realm stanza must set `max_renewable_life` (validated: `7d`) or `kinit -R` fails even when the client requests a renew lifetime |
| Test principal | `testuser@TEST.LOCAL` |
| Service principal | `svc/testhost.test.local@TEST.LOCAL` |
| Keytab | Exported via `kadmin.local ktadd`; stored as Kubernetes Secret |
| `krb5.conf` | ConfigMap pointing to `kdc-service.kdc-infra.svc.cluster.local`; `default_ccache_name = KEYRING:persistent:%{uid}` |
| Invalid keytab | Mismatched principal keytab (for NEG-01) |
| Invalid `krb5.conf` | Syntactically broken config (for NEG-01) |
| Short-lived `krb5.conf` | `ticket_lifetime=5m`, `renew_lifetime=30m` (for E2E-07); account for `clockskew` (default `300s`) when asserting post-expiry `kinit -R` failure |
| Test container image | UBI10-minimal (or equivalent) + `krb5-workstation` + `keyutils-libs` + `python3` baked in (prefer no runtime package install) |
| Userns SCC | `fsGroup: RunAsAny` (or map-compatible); `runAsUser: RunAsAny` with workload UID in userns map (e.g. `1000`), or `MustRunAsRange` within `0–65535`; optional `userNamespaceLevel: RequirePodLevel`; **sole** usable Kerberos SCC for the workload SA for IT-USNS-05/06 (NEG-USNS-01 binds a competing SCC) |

### 10.4 Test Environment Strategy

#### 10.4.1 Lab Environments

**Kerberos Infrastructure**

| Component | Details |
|---|---|
| MIT Kerberos KDC | RHEL / CentOS Stream in-cluster pod; realm `TEST.LOCAL` (`krb5-server` not in UBI — use full distro image for KDC) |
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
| Automated tests in OpenShift CI (Prow) | **Planned** : P0/P1 Ginkgo tests under `test/extended/node/`, run via `openshift-tests`. **Not present today.** |
| SPO availability check | When added, tests should skip gracefully (`g.Skip`) when SPO is not installed — SPO is an optional Rolling Stream operator, not in the OCP payload |
| Dedicated nightly job | **Planned:** a separate Prow job with SPO pre-installed runs the Kerberos suite nightly. **Not configured today.** The dedicated Kerberos CI job **must** pass these three checks using `KEYRING:persistent` (not `FILE:`): (1) a single pod can log in; (2) two pods on the same node keep tickets separate; (3) a replacement pod starts without the old pod’s ticket |
| Pre-merge scope | **Planned:** trigger only on PRs that touch SPO, CRI-O, kubelet, or Kerberos test files — not on all merges |
| Job classification | **Planned:** start as **informing**; promote to release-blocking only after automation is stable |

---

## 11. Risks and Contingencies

| Risk | Impact | Probability | Contingency |
|---|---|---|---|
| SPO unavailable in OCP 5.1 OperatorHub | Blocker — full suite cannot run | Low | Verify in first 5.1 nightly; SPO is a Rolling Stream operator independent of OCP releases |
| MIT KDC pod requires root (`kdb5_util`) | Test infra only; no production impact | Low | Use privileged SCC for KDC pod only; it is test infrastructure, not the system under test |
| Multi-replica KEYRING collision without userns | Silent ticket sharing / stale inheritance | High (confirmed) | Require `hostUsers: false` for E2E-02–E2E-04; automate NEG-UID-01 + IT-USNS-* |
| `RequirePodLevel` ineffective with competing SCC | Mandate appears broken | Medium (confirmed) | Document sole-SCC requirement; automate NEG-USNS-01; create pods as workload SA |
| Userns SCC `fsGroup: MustRunAs` large GID | `setgroups: Invalid argument` | Medium (confirmed) | Use `fsGroup: RunAsAny` (or map-compatible GID) on userns SCC |
| Userns + OpenShift namespace UID range as `runAsUser` | Container UID outside default `0–65535` map; pod fails to start | Medium | Use UID inside the userns map (validated: `1000`) or constrain SCC `MustRunAsRange` accordingly |
| Assuming ticket lost on every container restart | False failure on E2E-05 | Medium (confirmed) | Assert empty cache only for pod replace/rollout; document persistent survival on in-pod restart |
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
| 3.0 | 2026-11-17 | Test Engineering | Post-manual-validation update: `KEYRING:persistent` (not session); multi-replica isolation via `hostUsers: false`; IT-USNS-* / NEG-USNS-01 / NEG-UID-01; E2E-05 container-restart finding; E2E-06 init-share finding; RequirePodLevel mandate notes; pass/fail + execution order + risks updated from manual testing |
| 3.1 | 2026-11-20 | Test Engineering | PR #31917 review: userns-compatible `runAsUser`/UID map; E2E-05 requires `hostUsers: false` for empty-cache-on-replace; E2E-06 same-UID for init→main share; assert `KEYRING:persistent` in `klist`; KDC `max_renewable_life` + `renew until` check; post-expiry `kinit -R` wait past `clockskew`; soften session/PAM wording; keyring keyed per UID within user namespace; sole-SCC fixture for IT-USNS-05/06; dedicated CI must gate on login + same-node isolation + empty replace |

---

*End of Test Plan*
