# Emptydir Volume Quotas

## Abstract

This proposal describes a new volume plugin which implements storage quota for emptyDir
volume types. The volume plugin is designed as a drop-in replacement for the
emptydir plugin.

## Constraints and Assumptions

1. This proposal describes a quota implementation which expresses limits at the
namespace level. Pod-scoped quota is possible but not addressed here.

2. XFS must be available on nodes for which quota must be enforced.

3. No scheduler integration is assumed. Overcommit must be avoided through other capacity
restraints. (For example, by enforcing a fixed quota size across all namespaces and
limiting the number of pods per node through CPU, memory, cadvisor capacity information, etc.) 

## Design

Each OpenShift project is assigned a cluster-unique FSGroup ID which is propagated to
the security context of all pods and made accessible to volume plugins.

Building on these features, the new volume plugin can configure XFS quota on the node
during volume setup to restrict disk usage per namespace per node. With this plugin,
disk usage of all pods within a namespace per node would count towards the namespace's
quota on that node.

For example, given:

1. A cluster configuration which dictates a fixed 100 MB emptyDir quota for namespaces
2. Two nodes *node-1* and *node-2* each with 50 GB allocated for emptyDir storage
3. The following cluster state:

| Pod        | Node   |
|------------|--------|
| ns1/pod-a  | node-1 |
| ns1/pod-b  | node-1 |
| ns2/pod-a  | node-1 |
| ns1/pod-c  | node-2 |

Any data written an emptyDir volume by *ns1/pod-a* or *ns1/pod-b* share a 100 MB quota
on *node-1*, while *ns1/pod-c* emptyDir data counts towards a 100 MB quota on *node-2*.
In total, *ns1* can consume up to 200 MB of emptyDir storage on this cluster.

It's important to note that there is no explicit overcommit prevention through scheduling. If
the scenario is modified such that *node-1* has only 100 MB of storage, *ns2/pod-a* may still
be scheduled to *node-1* and depending on the underlying storage configuration, the disk backing
all emptyDirs for the node could be filled.

### XFS Implementation

TODO: Outline the specifics around XFS quota application.

## Areas of Improvement

* Per-pod quota
  * Possible, but requires further considerations:
    1. Needs namespace-unique fsgroup assignment per pod
    2. Could increase overcommit probability in the absence of tighter scheduler integration 

## Existing Work

[Kubernetes Disk Accounting proposal](https://github.com/vishh/kubernetes/blob/disk-accounting/docs/proposals/disk-accounting.md)

* May address emptyDir quota enforcement upstream
* Planned for Kube 1.2
