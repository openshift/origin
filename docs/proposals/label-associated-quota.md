# Label Constrained Quota Allocation

## Problem
Cluster-administrators want to allot a total quota cap to a particular entity (users to start with) and allow that entity to allocate the quota amongst "his" projects as he sees fit.
Our mechanism for relating entities is label selection, so we can create a `ResourceQuotaAllocation` object at the cluster scope that applies allotment totals against `ResourceQuota`
objects.  By associating projects to owners using structured labels, it becomes possible to allocate quota to the projects "owned" by a given user.


## Types
```go
// ResourceQuotaAllocation sets the maximum amount of resources that a group of projects may use.  It is enforced by preventing individual
// ResourceQuota allocations to the set of selected projects from exceeding set limits.  If a single project is selected by multiple
// ResourceQuotaAllocations, then the most restrictive limit wins.
type ResourceQuotaAllocation struct{
	unversioned.TypeMeta
	kapi.ObjectMeta

	Spec ResourceQuotaAllocationSpec
	Status ResourceQuotaAllocationStatus
}

type ResourceQuotaAllocationSpec struct{
	// Selector is used to find the set of projects that this ResourceQuotaAllocation is for
	Selector extensions.LabelSelector

	// Allocation is the description of the maximum quota sizes allowed across all the selected projects
	Allocation kapi.ResourceQuotaSpec
}

type ResourceQuotaAllocationStatus struct{
	// Allocated is the current state of usage across all selected projects
	Allocated kapi.ResourceQuotaStatus

	// UsedByProject makes association for users a little easier AND it allows us to update only when needed.
	UsedByProject NamespacedResourceList
}

type NamespacedResourceList struct{
	Namespace string

	Used kapi.ResourceList
}

```


## Enforcement
`ResourceQuotaAllocation` will be enforced using an admission plugin that watches creates and updates to the `resourcequota.spec`, combines the change with a cache of all other `resourcequota`
in the project, finds the associated project,  uses that project's labels to find matching `resourcequotaallocations`, makes sure that all quota'd resources are below the upper threshold, 
updates the affected `resourcequotaallocations.status`, and allows it.  To reduce update noise, we store the last `resourcequota.spec` on the `resourcequotaallocationstatus` to allow us to know whether the change matters to us.
Getting the max of everything allows individual `resourcequota` to not have to specify every possible resource from the `resourcequotaallocation`.
If you are reducing usage, then the action will be allowed regardless of the current usage.
Because this is an admisson plugin on a kube resource, if you run against an external kube, `ResourceQuotaAllocations` will not be enforced.  

In addition, a standard controller must also be written.  It should watch changes to `namespace.labels` to determine when new projects need to be taken into account or old projects no longer apply.
Similar to regular quota, it makes sense to periodically re-sync.  If we expect large numbers of project with infrequent overlaps, then `resourcequotaallocations` to `resourcequota` makes sense, otherwise do the reverse.
It must also watch `resourcequota` deletion to reclaim quota more quickly when possible.


## APIs
A new role called `project-owner` will be created.  `project-owners` are like `project-admins`, but they also have the power to update *some* of their `resourcequota` documents.
We don't want to allow a `project-owner` to modify any `resourcequota` document, because doing that would prevent a `cluster-admin` from being able to "lock down" a namespace by putting very low `resourcequota` into it.
In the short term, this will enforced by naming a series of "special" quota names that match today's scope: terminating, notTerminating, bestEffort, notBestEffort, default.
In the longer term, we could support permissions based on label selectors.

Cluster-scoped `resourcequotaallocation` resources which apply to a particular project will be projected into namespaces under the namespace-scoped  `localresourcequotaallocation` resource.
This allows the ACL rule inside of a project to be "get, list" on "localresourcequotaallocations".  That way a user can predict whether his `resourcequota` modification will succeed or fail.

