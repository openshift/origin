package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/registry/namespace"

	oapi "github.com/openshift/origin/pkg/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

// TODO move upstream along with the InformerFactory

type IndexerToNamespaceLister struct {
	cache.Indexer
}

func (i *IndexerToNamespaceLister) List(options kapi.ListOptions) ([]*kapi.Namespace, error) {
	returnedList := i.Indexer.List()
	ret := make([]*kapi.Namespace, 0, len(returnedList))
	matcher := namespace.MatchNamespace(oapi.ListOptionsToSelectors(&options))

	for i := range returnedList {
		clusterResourceQuota := returnedList[i].(*kapi.Namespace)
		if matches, err := matcher.Matches(clusterResourceQuota); err == nil && matches {
			ret = append(ret, clusterResourceQuota)
		}
	}
	return ret, nil
}

func (i *IndexerToNamespaceLister) Get(name string) (*kapi.Namespace, error) {
	keyObj := &kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := i.GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kapierrors.NewNotFound(quotaapi.Resource("namespace"), name)
		return nil, existsErr
	}
	return item.(*kapi.Namespace), nil
}
