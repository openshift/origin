package cache

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
)

// IndexerToSecurityContextConstraintsLister gives a store List and Exists methods. The store must contain only SecurityContextConstraints.
type IndexerToSecurityContextConstraintsLister struct {
	cache.Indexer
}

// List all SecurityContextConstraints in the store.
func (s *IndexerToSecurityContextConstraintsLister) List() ([]*kapi.SecurityContextConstraints, error) {
	sccs := []*kapi.SecurityContextConstraints{}
	for _, c := range s.Indexer.List() {
		sccs = append(sccs, c.(*kapi.SecurityContextConstraints))
	}
	return sccs, nil
}

func (s *IndexerToSecurityContextConstraintsLister) Get(name string, options metav1.GetOptions) (*kapi.SecurityContextConstraints, error) {
	keyObj := &kapi.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: name}}
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := s.GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kerrors.NewNotFound(kapi.Resource("securitycontexconstraints"), name)
		return nil, existsErr
	}
	return item.(*kapi.SecurityContextConstraints), nil
}
