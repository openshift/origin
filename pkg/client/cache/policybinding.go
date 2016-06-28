package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	"github.com/openshift/origin/pkg/client"
)

type InformerToPolicyBindingNamespacer struct {
	framework.SharedIndexInformer
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (i *InformerToPolicyBindingNamespacer) LastSyncResourceVersion() string {
	return i.SharedIndexInformer.LastSyncResourceVersion()
}

func (i *InformerToPolicyBindingNamespacer) PolicyBindings(namespace string) client.PolicyBindingLister {
	return &indexerToPolicyBindingLister{Indexer: i.GetIndexer(), namespace: namespace}
}

type indexerToPolicyBindingLister struct {
	cache.Indexer
	namespace string
}

func (i *indexerToPolicyBindingLister) List(options kapi.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	policyBindingList := &authorizationapi.PolicyBindingList{}
	matcher := policybindingregistry.Matcher(oapi.ListOptionsToSelectors(&options))

	if i.namespace == kapi.NamespaceAll {
		returnedList := i.Indexer.List()
		for i := range returnedList {
			policyBinding := returnedList[i].(*authorizationapi.PolicyBinding)
			if matches, err := matcher.Matches(policyBinding); err == nil && matches {
				policyBindingList.Items = append(policyBindingList.Items, *policyBinding)
			}
		}
		return policyBindingList, nil
	}

	key := &authorizationapi.PolicyBinding{ObjectMeta: kapi.ObjectMeta{Namespace: i.namespace}}
	items, err := i.Indexer.Index(cache.NamespaceIndex, key)
	if err != nil {
		return policyBindingList, err
	}

	for i := range items {
		policyBinding := items[i].(*authorizationapi.PolicyBinding)
		if matches, err := matcher.Matches(policyBinding); err == nil && matches {
			policyBindingList.Items = append(policyBindingList.Items, *policyBinding)
		}
	}
	return policyBindingList, nil
}

func (i *indexerToPolicyBindingLister) Get(name string) (*authorizationapi.PolicyBinding, error) {
	keyObj := &authorizationapi.PolicyBinding{ObjectMeta: kapi.ObjectMeta{Namespace: i.namespace, Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := i.Indexer.GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kapierrors.NewNotFound(authorizationapi.Resource("policyBinding"), name)
		return nil, existsErr
	}
	return item.(*authorizationapi.PolicyBinding), nil
}
