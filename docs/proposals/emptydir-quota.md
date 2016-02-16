# Emptydir Volume Quotas

## Abstract

This proposal describes a new volume plugin which implements storage quota for
emptyDir volume types. The volume plugin is designed as a drop-in replacement
for the emptydir plugin.

## Constraints and Assumptions

1. This proposal describes a quota implementation which expresses limits at the
namespace level. Pod-scoped quota is possible but not addressed here.

2. XFS must be available on nodes for which quota must be enforced.

3. No scheduler integration is assumed. Overcommit must be avoided through other
capacity restraints. (For example, by enforcing a fixed quota size across all
namespaces and limiting the number of pods per node through CPU, memory,
cadvisor capacity information, etc.)

## Design

Each OpenShift project is assigned a cluster-unique FSGroup ID which is
propagated to the security context of all pods and made accessible to volume
plugins.

Building on these features, the new volume plugin can configure XFS quota on the
node during volume setup to restrict disk usage per namespace per node. With
this plugin, disk usage of all pods within a namespace per node would count
towards the namespace's quota on that node.

For example, given:

1. A cluster configuration which dictates a fixed 100 MB emptyDir quota for
namespaces
2. Two nodes *node-1* and *node-2* each with 50 GB allocated for emptyDir
storage
3. The following cluster state:

| Pod        | Node   |
|------------|--------|
| ns1/pod-a  | node-1 |
| ns1/pod-b  | node-1 |
| ns2/pod-a  | node-1 |
| ns1/pod-c  | node-2 |

Any data written to an emptyDir volume by *ns1/pod-a* or *ns1/pod-b* share a 100 MB
quota on *node-1*, while *ns1/pod-c* emptyDir data counts towards a 100 MB quota
on *node-2*. In total, *ns1* can consume up to 200 MB of emptyDir storage on
this cluster.

It's important to note that there is no explicit overcommit prevention through
scheduling. If the scenario is modified such that *node-1* has only 100 MB of
storage, *ns2/pod-a* may still be scheduled to *node-1* and depending on the
underlying storage configuration, the disk backing all emptyDirs for the node
could be filled.

### XFS Implementation

To support the quota implementation, the volumes backing emptyDir storage
on each node must be mounted with XFS quota enabled:

```shell
mount -o gquota /dev/sdb2 /var/openshift/volumes/emptydir
```

The volume plugin implementation will use `xfs_quota` to constrain disk for a
given FSGroupID. For example, the plugin may issue a command such as:

```shell
xfs_quota -x -c 'limit -g bsoft=120m bhard=128m $FSGROUPID' /var/openshift/volumes/emptydir/$FSGROUPID
```

Repeated quota application on the same directory has no known issues.
Kubernetes will actually re-apply the quota often as part of its normal volume
reconciliation loop.  This has the added benefit of allowing changes to the
quota which will quickly be propagated out to the node.

Pods with the same FSGroupID will have a common root directly, e.g.
`/var/openshift/volumes/emptydir/$FSGROUPID/$POD_UID`.

XFS group/user ID quotas are stored as filesystem metadata in a [136 byte
struct](http://xfs.org/docs/xfsdocs-xml-dev/XFS_Filesystem_Structure//tmp/en-US/html/Internal_Inodes.html#Quota_Inodes).
Lookup for an ID involves seeking in this file to the ID offset x 136.  However
because of [sparse file](https://en.wikipedia.org/wiki/XFS#Sparse_files)
support, we can still use FSGroup IDs in the trillions without consuming an
impossible quantity of disk for metadata, we only pay the penalty for roughly
the actual number of IDs we do end up storing a quota for.

As an example using a 5Gb XFS partition which started with 4686512 Kb of
available disk, storing a quota for 100,000 group IDs starting at
4,000,000,000, the available space dropped to 4673176 Kb, a difference of
roughly 13Mb or almost exactly 136 bytes * 100,000.

There does not appear to be any in-memory cache, and as such it appears likely
that we can use our existing very large FSGroup IDs for quota on XFS even if
there were to be a very large number of them flowing through the node thoughout
its lifespan.

#### Caveats

* The XFS quota tools don't provide any means to delete a specific group's
quota configuration. Quota must be shutdown completely by disabling it and unmounting
the filesystem, then re-creating all quotas that are to be kept. This is
infeasible for our purposes so pre-existing quota definitions for pods which
are no longer on the node will be left in place. If the given FSGroup were to
re-appear on the node its quota would be reset to whatever the current
expected quota would be. Otherwise leaving the old quota records around should
have minimal impact beyond the storage considerations outlined above.

* The XFS quota reporting tools can behave poorly with large group ID ranges.
Using `xfs_quota report`, you can specify a range of gids to check and the code
checks each one in turn. For example:
    ```shell
    FSGROUPID=1000040000
    xfs_quota -x -c 'report -n -L 1 -U $FSGROUPID' /var/openshift/volumes/emptydir/$FSGROUPID
    ```
By default our FSGroupID's *start* in the 1,000,000,000, and so admins cannot
simply run a report from (1 - max fsgroup) as it will not return in a reasonable
timeframe. If admins need to check quotas on the filesystem it may be best to
check quotas individually. The plugin implementation does not use these reports,
and setting a quota with a huge gid is very fast; this is only an issue if
admins ever want to view the quotas in place.

## Areas of Improvement

* Per-pod quota
  * Possible, but requires further considerations:
    1. Needs namespace-unique fsgroup assignment per pod
    2. Could increase overcommit probability in the absence of tighter scheduler
    integration

## Existing Work

[Kubernetes Disk Accounting proposal](https://github.com/vishh/kubernetes/blob/disk-accounting/docs/proposals/disk-accounting.md)

* May address emptyDir quota enforcement upstream
* Planned for Kube 1.2
