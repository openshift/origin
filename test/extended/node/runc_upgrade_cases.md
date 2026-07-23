# runc RHCOS 10 Upgrade Guard Test Case

## Overview

Verify MCO blocks a pool move to RHCOS 10 when **runc** is configured via
`ContainerRuntimeConfig`, and that the cluster reports `Upgradeable=False` without
silently rebooting nodes onto an unsupported OS/runtime combination.

## Test Environment

- openshift 5.0 - 5.3 (dual stream cluster)
- skip on Microshift, Hypershift, SNO

## Test Implementation

Automated in [`test/extended/node/runc_upgrade_cases.go`](runc_upgrade_cases.go)

- **Suite:** `[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive][OCPFeatureGate:OSStreams] runc RHCOS 10 upgrade guard`
- **Case:** `blocks RHCOS 9 to 10 osImageStream upgrade when ContainerRuntimeConfig sets runc default runtime`
- **Lifecycle:** `ote.Informing()`

Run:

```bash
cd origin && make WHAT=cmd/openshift-tests
./openshift-tests run-test \
  "[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive][OCPFeatureGate:OSStreams] runc RHCOS 10 upgrade guard blocks RHCOS 9 to 10 osImageStream upgrade when ContainerRuntimeConfig sets runc default runtime"
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

- [openshift/enhancements#2032](https://github.com/openshift/enhancements/blob/master/enhancements/machine-config/block-runc-on-rhcos10-upgrade.md)
- [OCPSTRAT-3154](https://issues.redhat.com/browse/OCPSTRAT-3154) — runc deprecation warning (separate)
