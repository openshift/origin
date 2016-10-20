package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"
)

// StoreToServiceAccountLister gives a store List and Exists methods. The store must contain only ServiceAccounts.
type StoreToServiceAccountLister struct {
	cache.Indexer
}

func (s *StoreToServiceAccountLister) ServiceAccounts(namespace string) storeServiceAccountsNamespacer {
	return storeServiceAccountsNamespacer{s.Indexer, namespace}
}

// storeServiceAccountsNamespacer provides a way to get and list ServiceAccounts from a specific namespace.
type storeServiceAccountsNamespacer struct {
	indexer   cache.Indexer
	namespace string
}

// Get the  ServiceAccount matching the name from the cache.
func (s storeServiceAccountsNamespacer) Get(name string) (*kapi.ServiceAccount, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(kapi.Resource("serviceaccount"), name)
	}
	return obj.(*kapi.ServiceAccount), nil
}

// List all the ServiceAccounts that match the provided selector using a namespace index.
// If the indexed list fails then we will fallback to listing from all namespaces and filter
// by the namespace we want.
func (s storeServiceAccountsNamespacer) List(selector labels.Selector) ([]*kapi.ServiceAccount, error) {
	serviceAccounts := []*kapi.ServiceAccount{}

	if s.namespace == kapi.NamespaceAll {
		for _, obj := range s.indexer.List() {
			bc := obj.(*kapi.ServiceAccount)
			if selector.Matches(labels.Set(bc.Labels)) {
				serviceAccounts = append(serviceAccounts, bc)
			}
		}
		return serviceAccounts, nil
	}

	items, err := s.indexer.ByIndex(cache.NamespaceIndex, s.namespace)
	if err != nil {
		return nil, err
	}
	for _, obj := range items {
		bc := obj.(*kapi.ServiceAccount)
		if selector.Matches(labels.Set(bc.Labels)) {
			serviceAccounts = append(serviceAccounts, bc)
		}
	}
	return serviceAccounts, nil
}
