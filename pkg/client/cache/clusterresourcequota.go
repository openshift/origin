package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"

	oapi "github.com/openshift/origin/pkg/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	clusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/clusterresourcequota"
)

type IndexerToClusterResourceQuotaLister struct {
	cache.Indexer
}

func (i *IndexerToClusterResourceQuotaLister) List(options kapi.ListOptions) ([]*quotaapi.ClusterResourceQuota, error) {
	returnedList := i.Indexer.List()
	ret := make([]*quotaapi.ClusterResourceQuota, 0, len(returnedList))
	matcher := clusterresourcequotaregistry.Matcher(oapi.ListOptionsToSelectors(&options))

	for i := range returnedList {
		clusterResourceQuota := returnedList[i].(*quotaapi.ClusterResourceQuota)
		if matches, err := matcher.Matches(clusterResourceQuota); err == nil && matches {
			ret = append(ret, clusterResourceQuota)
		}
	}
	return ret, nil
}

func (i *IndexerToClusterResourceQuotaLister) Get(name string) (*quotaapi.ClusterResourceQuota, error) {
	keyObj := &quotaapi.ClusterResourceQuota{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := i.GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kapierrors.NewNotFound(quotaapi.Resource("clusterresourcequota"), name)
		return nil, existsErr
	}
	return item.(*quotaapi.ClusterResourceQuota), nil
}
