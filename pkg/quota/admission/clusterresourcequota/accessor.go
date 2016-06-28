package clusterresourcequota

import (
	"time"

	lru "github.com/hashicorp/golang-lru"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	utilquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/storage/etcd"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	oclient "github.com/openshift/origin/pkg/client"
	ocache "github.com/openshift/origin/pkg/client/cache"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

// QuotaAccessor abstracts the get/set logic from the rest of the Evaluator.  This could be a test stub, a straight passthrough,
// or most commonly a series of deconflicting caches.
type QuotaAccessor interface {
	// UpdateQuotaStatus is called to persist final status.  This method should write to persistent storage.
	// An error indicates that write didn't complete successfully.
	UpdateQuotaStatus(newQuota *kapi.ResourceQuota) error

	// GetQuotas gets all possible quotas for a given namespace
	GetQuotas(namespace string) ([]kapi.ResourceQuota, error)
}

type quotaAccessor struct {
	quotaLister     *ocache.IndexerToClusterResourceQuotaLister
	namespaceLister *ocache.IndexerToNamespaceLister
	quotaClient     oclient.ClusterResourceQuotasInterface

	quotaMapper clusterquotamapping.ClusterQuotaMapper

	// updatedQuotas holds a cache of quotas that we've updated.  This is used to pull the "really latest" during back to
	// back quota evaluations that touch the same quota doc.  This only works because we can compare etcd resourceVersions
	// for the same resource as integers.  Before this change: 22 updates with 12 conflicts.  after this change: 15 updates with 0 conflicts
	updatedQuotas *lru.Cache
}

// newQuotaAccessor creates an object that conforms to the QuotaAccessor interface to be used to retrieve quota objects.
func newQuotaAccessor(quotaLister *ocache.IndexerToClusterResourceQuotaLister, namespaceLister *ocache.IndexerToNamespaceLister, quotaClient oclient.ClusterResourceQuotasInterface, quotaMapper clusterquotamapping.ClusterQuotaMapper) *quotaAccessor {
	updatedCache, err := lru.New(100)
	if err != nil {
		// this should never happen
		panic(err)
	}

	return &quotaAccessor{
		quotaLister:     quotaLister,
		namespaceLister: namespaceLister,
		quotaClient:     quotaClient,
		quotaMapper:     quotaMapper,
		updatedQuotas:   updatedCache,
	}
}

func (e *quotaAccessor) UpdateQuotaStatus(newQuota *kapi.ResourceQuota) error {
	quota, err := e.quotaLister.Get(newQuota.Name)
	if err != nil {
		return err
	}
	quota = e.checkCache(quota)

	// make a copy
	obj, err := kapi.Scheme.Copy(quota)
	if err != nil {
		return err
	}
	quota = obj.(*quotaapi.ClusterResourceQuota)
	usageDiff := utilquota.Subtract(newQuota.Status.Used, quota.Status.Total.Used)

	// re-assign objectmeta
	quota.ObjectMeta = newQuota.ObjectMeta
	quota.Namespace = ""

	oldNamespaceTotals, _ := quota.Status.Namespaces.Get(newQuota.Namespace)
	namespaceTotalCopy, err := kapi.Scheme.DeepCopy(oldNamespaceTotals)
	if err != nil {
		return err
	}
	newNamespaceTotals := namespaceTotalCopy.(kapi.ResourceQuotaStatus)
	newNamespaceTotals.Used = utilquota.Add(newNamespaceTotals.Used, usageDiff)
	quota.Status.Namespaces.Insert(newQuota.Namespace, newNamespaceTotals)

	quota.Status.Total.Used = utilquota.Add(quota.Status.Total.Used, usageDiff)

	updatedQuota, err := e.quotaClient.ClusterResourceQuotas().Update(quota)
	if err != nil {
		return err
	}

	e.updatedQuotas.Add(quota.Name, updatedQuota)
	return nil
}

var etcdVersioner = etcd.APIObjectVersioner{}

// checkCache compares the passed quota against the value in the look-aside cache and returns the newer
// if the cache is out of date, it deletes the stale entry.  This only works because of etcd resourceVersions
// being monotonically increasing integers
func (e *quotaAccessor) checkCache(quota *quotaapi.ClusterResourceQuota) *quotaapi.ClusterResourceQuota {
	uncastCachedQuota, ok := e.updatedQuotas.Get(quota.Name)
	if !ok {
		return quota
	}
	cachedQuota := uncastCachedQuota.(*quotaapi.ClusterResourceQuota)

	if etcdVersioner.CompareResourceVersion(quota, cachedQuota) >= 0 {
		e.updatedQuotas.Remove(quota.Name)
		return quota
	}
	return cachedQuota
}

func (e *quotaAccessor) GetQuotas(namespaceName string) ([]kapi.ResourceQuota, error) {
	var quotaNames []string
	// wait for a valid mapping cache.  The overall response can be delayed for up to 10 seconds.
	err := utilwait.PollImmediate(100*time.Millisecond, 8*time.Second, func() (done bool, err error) {
		var namespacelabels map[string]string
		quotaNames, namespacelabels = e.quotaMapper.GetClusterQuotasFor(namespaceName)
		namespace, err := e.namespaceLister.Get(namespaceName)
		if err != nil {
			return false, err
		}
		if kapi.Semantic.DeepEqual(namespacelabels, namespace.Labels) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	resourceQuotas := []kapi.ResourceQuota{}
	for _, quotaName := range quotaNames {
		quota, err := e.quotaLister.Get(quotaName)
		if kapierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		quota = e.checkCache(quota)

		// now convert to a ResourceQuota
		convertedQuota := kapi.ResourceQuota{}
		convertedQuota.ObjectMeta = quota.ObjectMeta
		convertedQuota.Namespace = namespaceName
		convertedQuota.Spec = quota.Spec.Quota
		convertedQuota.Status = quota.Status.Total
		resourceQuotas = append(resourceQuotas, convertedQuota)

	}

	return resourceQuotas, nil
}
