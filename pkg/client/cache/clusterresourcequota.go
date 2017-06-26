package cache

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	oapi "github.com/openshift/origin/pkg/api"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	clusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/clusterresourcequota"
)

type IndexerToClusterResourceQuotaLister struct {
	cache.Indexer
}

func (i *IndexerToClusterResourceQuotaLister) List(options metav1.ListOptions) ([]*quotaapi.ClusterResourceQuota, error) {
	returnedList := i.Indexer.List()
	ret := make([]*quotaapi.ClusterResourceQuota, 0, len(returnedList))
	labelSel, fieldSel, err := oapi.ListOptionsToSelectors(&options)
	if err != nil {
		return nil, err
	}
	matcher := clusterresourcequotaregistry.Matcher(labelSel, fieldSel)

	for i := range returnedList {
		clusterResourceQuota := returnedList[i].(*quotaapi.ClusterResourceQuota)
		if matches, err := matcher.Matches(clusterResourceQuota); err == nil && matches {
			ret = append(ret, clusterResourceQuota)
		}
	}
	return ret, nil
}

func (i *IndexerToClusterResourceQuotaLister) Get(name string) (*quotaapi.ClusterResourceQuota, error) {
	keyObj := &quotaapi.ClusterResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: name}}
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

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
