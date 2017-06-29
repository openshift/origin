package cache

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	"github.com/openshift/origin/pkg/client"
)

type InformerToPolicyBindingNamespacer struct {
	cache.SharedIndexInformer
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

func (i *indexerToPolicyBindingLister) List(options metav1.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	policyBindingList := &authorizationapi.PolicyBindingList{}
	labelSel, fieldSel, err := oapi.ListOptionsToSelectors(&options)
	if err != nil {
		return nil, err
	}
	matcher := policybindingregistry.Matcher(labelSel, fieldSel)

	if i.namespace == metav1.NamespaceAll {
		returnedList := i.Indexer.List()
		for i := range returnedList {
			policyBinding := returnedList[i].(*authorizationapi.PolicyBinding)
			if matches, err := matcher.Matches(policyBinding); err == nil && matches {
				policyBindingList.Items = append(policyBindingList.Items, *policyBinding)
			}
		}
		return policyBindingList, nil
	}

	key := &authorizationapi.PolicyBinding{ObjectMeta: metav1.ObjectMeta{Namespace: i.namespace}}
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

func (i *indexerToPolicyBindingLister) Get(name string, options metav1.GetOptions) (*authorizationapi.PolicyBinding, error) {
	keyObj := &authorizationapi.PolicyBinding{ObjectMeta: metav1.ObjectMeta{Namespace: i.namespace, Name: name}}
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

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
