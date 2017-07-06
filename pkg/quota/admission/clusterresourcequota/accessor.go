package clusterresourcequota

import (
	"time"

	lru "github.com/hashicorp/golang-lru"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	etcd "k8s.io/apiserver/pkg/storage/etcd"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	utilquota "k8s.io/kubernetes/pkg/quota"

	oclient "github.com/openshift/origin/pkg/client"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotalister "github.com/openshift/origin/pkg/quota/generated/listers/quota/internalversion"
)

type clusterQuotaAccessor struct {
	clusterQuotaLister quotalister.ClusterResourceQuotaLister
	namespaceLister    kcorelisters.NamespaceLister
	clusterQuotaClient oclient.ClusterResourceQuotasInterface

	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper

	// updatedClusterQuotas holds a cache of quotas that we've updated.  This is used to pull the "really latest" during back to
	// back quota evaluations that touch the same quota doc.  This only works because we can compare etcd resourceVersions
	// for the same resource as integers.  Before this change: 22 updates with 12 conflicts.  after this change: 15 updates with 0 conflicts
	updatedClusterQuotas *lru.Cache
}

// newQuotaAccessor creates an object that conforms to the QuotaAccessor interface to be used to retrieve quota objects.
func newQuotaAccessor(
	clusterQuotaLister quotalister.ClusterResourceQuotaLister,
	namespaceLister kcorelisters.NamespaceLister,
	clusterQuotaClient oclient.ClusterResourceQuotasInterface,
	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper,
) *clusterQuotaAccessor {
	updatedCache, err := lru.New(100)
	if err != nil {
		// this should never happen
		panic(err)
	}

	return &clusterQuotaAccessor{
		clusterQuotaLister:   clusterQuotaLister,
		namespaceLister:      namespaceLister,
		clusterQuotaClient:   clusterQuotaClient,
		clusterQuotaMapper:   clusterQuotaMapper,
		updatedClusterQuotas: updatedCache,
	}
}

// UpdateQuotaStatus the newQuota coming in will be incremented from the original.  The difference between the original
// and the new is the amount to add to the namespace total, but the total status is the used value itself
func (e *clusterQuotaAccessor) UpdateQuotaStatus(newQuota *kapi.ResourceQuota) error {
	clusterQuota, err := e.clusterQuotaLister.Get(newQuota.Name)
	if err != nil {
		return err
	}
	clusterQuota = e.checkCache(clusterQuota)

	// make a copy
	obj, err := kapi.Scheme.Copy(clusterQuota)
	if err != nil {
		return err
	}
	// re-assign objectmeta
	clusterQuota = obj.(*quotaapi.ClusterResourceQuota)
	clusterQuota.ObjectMeta = newQuota.ObjectMeta
	clusterQuota.Namespace = ""

	// determine change in usage
	usageDiff := utilquota.Subtract(newQuota.Status.Used, clusterQuota.Status.Total.Used)

	// update aggregate usage
	clusterQuota.Status.Total.Used = newQuota.Status.Used

	// update per namespace totals
	oldNamespaceTotals, _ := clusterQuota.Status.Namespaces.Get(newQuota.Namespace)
	namespaceTotalCopy, err := kapi.Scheme.DeepCopy(oldNamespaceTotals)
	if err != nil {
		return err
	}
	newNamespaceTotals := namespaceTotalCopy.(kapi.ResourceQuotaStatus)
	newNamespaceTotals.Used = utilquota.Add(oldNamespaceTotals.Used, usageDiff)
	clusterQuota.Status.Namespaces.Insert(newQuota.Namespace, newNamespaceTotals)

	updatedQuota, err := e.clusterQuotaClient.ClusterResourceQuotas().UpdateStatus(clusterQuota)
	if err != nil {
		return err
	}

	e.updatedClusterQuotas.Add(clusterQuota.Name, updatedQuota)
	return nil
}

var etcdVersioner = etcd.APIObjectVersioner{}

// checkCache compares the passed quota against the value in the look-aside cache and returns the newer
// if the cache is out of date, it deletes the stale entry.  This only works because of etcd resourceVersions
// being monotonically increasing integers
func (e *clusterQuotaAccessor) checkCache(clusterQuota *quotaapi.ClusterResourceQuota) *quotaapi.ClusterResourceQuota {
	uncastCachedQuota, ok := e.updatedClusterQuotas.Get(clusterQuota.Name)
	if !ok {
		return clusterQuota
	}
	cachedQuota := uncastCachedQuota.(*quotaapi.ClusterResourceQuota)

	if etcdVersioner.CompareResourceVersion(clusterQuota, cachedQuota) >= 0 {
		e.updatedClusterQuotas.Remove(clusterQuota.Name)
		return clusterQuota
	}
	return cachedQuota
}

func (e *clusterQuotaAccessor) GetQuotas(namespaceName string) ([]kapi.ResourceQuota, error) {
	clusterQuotaNames, err := e.waitForReadyClusterQuotaNames(namespaceName)
	if err != nil {
		return nil, err
	}

	resourceQuotas := []kapi.ResourceQuota{}
	for _, clusterQuotaName := range clusterQuotaNames {
		clusterQuota, err := e.clusterQuotaLister.Get(clusterQuotaName)
		if kapierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		clusterQuota = e.checkCache(clusterQuota)

		// now convert to a ResourceQuota
		convertedQuota := kapi.ResourceQuota{}
		convertedQuota.ObjectMeta = clusterQuota.ObjectMeta
		convertedQuota.Namespace = namespaceName
		convertedQuota.Spec = clusterQuota.Spec.Quota
		convertedQuota.Status = clusterQuota.Status.Total
		resourceQuotas = append(resourceQuotas, convertedQuota)

	}

	return resourceQuotas, nil
}

func (e *clusterQuotaAccessor) waitForReadyClusterQuotaNames(namespaceName string) ([]string, error) {
	var clusterQuotaNames []string
	// wait for a valid mapping cache.  The overall response can be delayed for up to 10 seconds.
	err := utilwait.PollImmediate(100*time.Millisecond, 8*time.Second, func() (done bool, err error) {
		var namespaceSelectionFields clusterquotamapping.SelectionFields
		clusterQuotaNames, namespaceSelectionFields = e.clusterQuotaMapper.GetClusterQuotasFor(namespaceName)
		namespace, err := e.namespaceLister.Get(namespaceName)
		if err != nil {
			return false, err
		}
		if kapihelper.Semantic.DeepEqual(namespaceSelectionFields, clusterquotamapping.GetSelectionFields(namespace)) {
			return true, nil
		}
		return false, nil
	})
	return clusterQuotaNames, err
}
