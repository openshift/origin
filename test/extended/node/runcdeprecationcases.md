# Test Plan: runc Deprecation Cases

## Metadata

| Field               | Value |
|---------------------|-------|
| **Test file**       | `test/extended/node/runcdeprecationcases.go` |
| **Package**         | `node` |
| **Test suite**      | `[Jira:Node][sig-node] runc deprecation cases` |
| **Feature**        | [OCPSTRAT-3154](https://redhat.atlassian.net/browse/OCPSTRAT-3154) — runc deprecation warning in OCP 5.0 for clusters upgrading from 4.22 |
| **Epic**            | [OCPNODE-4013](https://redhat.atlassian.net/browse/OCPNODE-4013) — Block upgrade from RHCOS 9 to RHCOS 10 when runc is in use |
| **User Story**      | [OCPNODE-4567](https://redhat.atlassian.net/browse/OCPNODE-4567) |
| **Assignee**        | Aditi Sahay (asahay@redhat.com) |
| **Component**       | `sig-node`, MCO, CRI-O |
| **Test type**       | E2E / functional |
| **Ginkgo label**    | `[Jira:Node][sig-node]` |
| **Disruptive**      | No |
| **Requires reboot** | No |

---

## UseCase3: Fresh Install on RHCOS 9 with crun

### Description

A freshly installed OpenShift cluster on RHCOS 9 uses **crun** as the default container runtime.
This is the standard out-of-the-box configuration — no custom `ContainerRuntimeConfig` should be
present, and `crun` must be the CRI-O default on all worker nodes.

### Scope

| Item                       | Value |
|----------------------------|-------|
| **OpenShift versions**     | 5.0, 5.1, 5.2 (fresh install) |
| **OS**                     | RHCOS 9 (`VARIANT_ID=coreos`, `VERSION_ID=9.x`) |
| **Default runtime**        | `crun` |
| **ContainerRuntimeConfig** | None (no custom CRC expected) |
| **Skip conditions**        | MicroShift, Hypershift (external control plane) |

### Test Function

```go
g.It("RHCOS 9 cluster install should use crun as the default container runtime")
```

---

## What This Test Validates

This test verifies that a fresh RHCOS 9 cluster has `crun` configured as the default container
runtime with no custom overrides, and that the cluster is fully healthy. It performs the following
validations in sequence:

1. **Cluster health** — Confirms the `ClusterVersion` is `Available=True` and `Progressing=False`,
   indicating the cluster is stable before any further checks.

2. **RHCOS 9 OS identity** — Selects a worker node and reads `/etc/os-release` via `oc debug` to
   confirm `VARIANT_ID=coreos` (confirms RHCOS, not plain RHEL) and `VERSION_ID=9.x` (confirms
   RHCOS 9 major version).

3. **No custom ContainerRuntimeConfig** — Lists all `ContainerRuntimeConfig` objects cluster-wide
   and asserts the list is empty. This confirms no administrator-applied runtime override is present,
   which is the expected state on a fresh install.

4. **CRI-O default runtime is crun** — Reads the effective CRI-O configuration on the worker node
   (`crio status config`) and asserts `default_runtime = "crun"`. This is the authoritative
   on-node check that crun is active, independent of the API-level CRC check above.

5. **machine-config ClusterOperator health** — Verifies the `machine-config` ClusterOperator
   conditions: `Available=True`, `Progressing=False`, `Degraded=False`. A degraded MCO could
   indicate the runc guard (from MCO PR [#5891](https://github.com/openshift/machine-config-operator/pull/5891))
   has incorrectly fired on a valid crun cluster.

6. **Functional workload execution** — Creates a test pod pinned to the inspected worker node using
   the framework shell image and waits for it to reach `Succeeded` phase. This confirms the crun
   runtime can successfully pull and execute containers end-to-end.

This test is **read-only and non-disruptive** — it does not create or modify any
`MachineConfig`, `ContainerRuntimeConfig`, or node configuration.

---

## Pass/Fail Criteria

| Check                                  | Pass condition |
|----------------------------------------|----------------|
| ClusterVersion Available               | `AVAILABLE=True`, `PROGRESSING=False` |
| RHCOS version on worker node           | `VARIANT_ID=coreos` and `VERSION_ID="9.x"` |
| No ContainerRuntimeConfig              | List is empty (no custom CRC present) |
| CRI-O default runtime                  | `default_runtime = "crun"` in `crio status config` |
| machine-config ClusterOperator healthy | `Available=True`, `Progressing=False`, `Degraded=False` |
| Test workload runs successfully        | Pod reaches `Succeeded` phase on the inspected node |

---

## Related Links

- **This PR**: [openshift/origin#31257](https://github.com/openshift/origin/pull/31257) — contains `test/extended/node/runcdeprecationcases.go` and `test/extended/node/runcdeprecationcases.md`
- Strategy: [OCPSTRAT-3154](https://redhat.atlassian.net/browse/OCPSTRAT-3154)
- Epic: [OCPNODE-4013](https://redhat.atlassian.net/browse/OCPNODE-4013)
- User Story: [OCPNODE-4567](https://redhat.atlassian.net/browse/OCPNODE-4567)
- Design doc: `OCPNODE-4013-design-and-use-cases.md` (§5g / UC-7)
- MCO guard PR: [openshift/machine-config-operator#5891](https://github.com/openshift/machine-config-operator/pull/5891)
- Related test: `test/extended/node/node_swap.go`
