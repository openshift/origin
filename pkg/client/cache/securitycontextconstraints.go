package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
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

func (s *IndexerToSecurityContextConstraintsLister) Get(name string) (*kapi.SecurityContextConstraints, error) {
	keyObj := &kapi.SecurityContextConstraints{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

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
