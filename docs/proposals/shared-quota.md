# Shared Quota

## Problem
Cluster-administrators want to restrict the total number of resoruces a particular entity can create (users to start with) and don't care whether or how the user subdivides that quota.
Our mechanism for relating entities is label selection, so we can create a `ClusterResourceQuota` object at the cluster scope that mirrors the `ResourceQuota` at a cluster scope.
By associating projects to owners using structured labels, it becomes possible to allocate quota to the projects "owned" by a given user.


## Types
```go
// ClusterResourceQuota mirrors ResourceQuota at a cluster scope.  This object is easily convertible to 
// synthetic ResourceQuota object to allow quota evaluation re-use.
type ClusterResourceQuota struct {
	unversioned.TypeMeta
	kapi.ObjectMeta

	// Spec defines the desired quota
	Spec ClusterResourceQuotaSpec

	// Status defines the actual enforced quota and its current usage
	Status ClusterResourceQuotaStatus
}

type ClusterResourceQuotaSpec struct {
	// Selector is the label selector used to match projects.  It is not allowed to be empty
	// and should only select active projects on the scale of dozens (though it can select 
	// many more less active projects).  These projects will contend on object creation through
	// this resource.
	Selector map[string]string

	// Spec defines the desired quota
	Quota kapi.ResourceQuotaSpec
}

type ClusterResourceQuotaStatus struct {
	// Overall defines the actual enforced quota and its current usage across all namespaces
	Overall kapi.ResourceQuotaStatus

	// ByNamespace slices the usage by namespace.  This division allows for quick resolution of 
	// deletion reconcilation inside of a single namespace without requiring a recalculation 
	// across all namespaces.  This map can be used to pull the deltas for a given namespace.
	ByNamespace map[string]kapi.ResourceQuotaStatus
}
```


## Enforcement
`ClusterResourceQuota` will be enforced with an admission plugin that re-uses the bucketing and evalution code from the `ResourceQuota` admission plugin.
On a per-API server basis, the `ClusterResourceQuota` updates will be locked in-process.  On a per-cluster basis, the `ClusterResourceQuota` updates will be locked by condition updates on `resourceVersion`.

This introduces contention when resources are being created across namespaces.  For example, crq-a covers ns-1 and ns-2.  ns-1 and ns-2 are both creating pods.
Before the pods are admitted, crq-a is checked.  ns-1 pod creates are blocked waiting for ns-2 pods to complete admission.


## APIs
A new role called `project-owner` will be created.  `project-owners` are like `project-admins`, but they also have the power to update *some* of their `resourcequota` documents.
We don't want to allow a `project-owner` to modify any `resourcequota` document, because doing that would prevent a `cluster-admin` from being able to "lock down" a namespace by putting very low `resourcequota` into it.
In the short term, this will enforced by naming a series of "special" quota names that match today's scope: terminating, notTerminating, bestEffort, notBestEffort, default.
In the longer term, we could support permissions based on label selectors.

Cluster-scoped `resourcequotaallocation` resources which apply to a particular project will be projected into namespaces under the namespace-scoped  `localresourcequota` resource.
This allows the ACL rule inside of a project to be "get, list" on "localresourcequota".  That way a user can predict whether his resource creation will succeed or fail based on clusterresourcequota.


## Comparison to Label Constrained Quota Allocation
By bounding allocation (quota allocation), we can write a reasonably efficient implementation that only watches resourcequotas across the cluster.  This eliminates cross namespace contention during resource creation.
Unfortunately, quota allocation can be difficult to use.  Take new project creation as an example.  In order to be safe with respect to the allocation, new projects would have to be created with all quotaed objects set to zero.
We can't reasonably do this globally, since not all cluster-admins will want to quota all objects.  In addition, we can safely cover the individual scopes with the "all" scopes quota, but after his project is created
a project that admin has to inspect all his quota allocation limits and create and modify resource quota documents to match before he can do anything in his project.  This requires a pretty significant understanding
of the overall quota system before a project-admin can do anything.

By contrast, the shared quota approach doesn't require any action in "normal" usage by a project-admin.  The cluster-admin is the only one who has to worry about particulars and 
a project-admin just uses his project.  If he wants to, he can set up resourcequota objects to constraint particular projects and keep them from using all of his quota.
Using this approach, he can overcommit his clusterresourcequota.

I created a test in https://github.com/openshift/origin/pull/8686 to try to characterize the contention.  I created 100 namespaces all covered by a single CRQ.
For each namespace, I created an individual pod and waited for a response from the API server before trying to create another.
This is the worst case action since multiple requests for a given namespace are batched and the lock cost gets amortized.
I was able to create 2.6 pods per namespace, per second in that test.  With two concurrent clients (the way the replication controller manager works), I was able to get double that.

Neither approach precludes the other, but given the comparative easy of use of clusterresourcequota and its reasonable level of scaling, I think that's where we should start.