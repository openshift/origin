# runc RHCOS 10 Upgrade Guard Test Case

## Overview

Verify MCO blocks a pool move to RHCOS 10 when **runc** is configured via
`ContainerRuntimeConfig`, and that the cluster reports `Upgradeable=False` without
silently rebooting nodes onto an unsupported OS/runtime combination.

## Related Issues

- Epic: [OCPNODE-4013](https://issues.redhat.com/browse/OCPNODE-4013)
- Story: [OCPNODE-4494](https://issues.redhat.com/browse/OCPNODE-4494)
- MCO PR: [openshift/machine-config-operator#5891](https://github.com/openshift/machine-config-operator/pull/5891)
- Origin PR: [openshift/origin#31266](https://github.com/openshift/origin/pull/31266)

## User Story

**As a** cluster administrator upgrading from OCP 4.x to 5.0 with dual OS streams  
**I want** MCO to block RHCOS 10 rollout on pools still using runc  
**So that** nodes are not rebooted into RHCOS 10 where runc is unavailable and workloads break

## Description

RHEL 10 / RHCOS 10 does not ship runc. Pools that set `defaultRuntime: runc` through
a `ContainerRuntimeConfig` must not roll out when `spec.osImageStream` targets `rhel-10`.
The guard runs in the MCO render controller (`validateNoRuncOnRHEL10`) and surfaces
`RenderDegraded` on the pool before any node reboot.

The automated test uses an isolated MCP (`runc-rhcos10-guard`) on one pure worker so
the rest of the cluster is unaffected.

## Test Steps and Expected Results

| Step | Expected Result |
|:-----|:----------------|
| Preflight: confirm dual streams and skip unsupported platforms | Test runs only when `rhel-9` and `rhel-10` exist; skips MicroShift, Hypershift, SNO |
| Create MCP pinned to `rhel-9` and CRC with `defaultRuntime: runc` | Resources created for pool `runc-rhcos10-guard` |
| Label one pure worker into the custom pool | MCP reports 1 machine; pool rolls out healthy on RHCOS 9 |
| Verify baseline on test node | Node Ready; OSImage shows RHCOS 9; runc is default runtime |
| Patch MCP `spec.osImageStream` to `rhel-10` | Guard fires before node rollout |
| Verify render guard | MCP `RenderDegraded=True`; message contains `runc` and `rhel-10` |
| Verify node unchanged | Node Ready; `currentConfig == desiredConfig`; still RHCOS 9 with runc |
| Verify upgrade blocked | `co/machine-config` `Upgradeable=False` (reason `DegradedPool`); CO/CV `Degraded=False`, `Available=True` |
| Revert MCP to `rhel-9` | Pool recovers; node still RHCOS 9 with runc |
| If cluster default stream is `rhel-10`: delete CRC and patch MCP to `rhel-10` | Node rolls out to RHCOS 10 with crun (no guard without runc) |
| Cleanup | Node unlabeled; CRC and MCP deleted; worker MCP healthy |

## Prerequisites

- OCP 5.0+ with `OSImageStream` API and dual streams (`rhel-9`, `rhel-10`)
- MCO build with runc guard (PR 5891 or payload)
- At least one pure worker node
- Cluster-admin access

## Test Environment

- Self-managed HA cluster (not MicroShift / Hypershift / SNO)
- Tech Preview may be required on 4.x; often GA on 5.0 self-managed for `OSStreams`

## Test Data

- MCP: `runc-rhcos10-guard` (`spec.osImageStream.name: rhel-9`, then `rhel-10`)
- CRC: `99-runc-rhcos10-guard-runc` (`defaultRuntime: runc`)
- Node label: `node-role.kubernetes.io/runc-rhcos10-guard`

## Test Implementation

Automated in [`test/extended/node/runc_upgrade_cases.go`](runc_upgrade_cases.go)

- **Suite:** `[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive] runc RHCOS 10 upgrade guard`
- **Case:** `blocks upgrade of RHCOS 9 to 10 when ContainerRuntimeConfig sets default runtime to runc`
- **Lifecycle:** `ote.Informing()`

Run:

```bash
cd origin && make WHAT=cmd/openshift-tests
./openshift-tests run-test \
  "[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive] runc RHCOS 10 upgrade guard blocks upgrade of RHCOS 9 to 10 when ContainerRuntimeConfig sets default runtime to runc"
```

Suggested CI: `periodic-ci-openshift-release-main-nightly-5.0-e2e-aws-disruptive-longrunning-techpreview-1of2` (with MCO payload)

## Notes

- `RenderDegraded` is the authoritative guard signal; `Degraded` may appear shortly after.
- CO/CVO `Degraded=True` can take ~30 minutes on a stuck pool; the test asserts
  `Upgradeable=False` within 5 minutes and recovers before delayed Degraded propagation.
- Typical runtime: ~15–30 minutes when optional RHCOS 10 recovery runs on 5.0 clusters.
- MCP is pinned to `rhel-9` at creation so it does not inherit cluster default `rhel-10`.
- Runtime is configured via **ContainerRuntimeConfig** (supported path), not a hand-crafted MC drop-in.

## References

- [openshift/enhancements#2032](https://github.com/openshift/enhancements/pull/2032)
- [OCPSTRAT-3154](https://issues.redhat.com/browse/OCPSTRAT-3154) — runc deprecation warning (separate)
