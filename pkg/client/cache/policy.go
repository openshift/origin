package cache

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	"github.com/openshift/origin/pkg/client"
)

type InformerToPolicyNamespacer struct {
	cache.SharedIndexInformer
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (i *InformerToPolicyNamespacer) LastSyncResourceVersion() string {
	return i.SharedIndexInformer.LastSyncResourceVersion()
}

func (i *InformerToPolicyNamespacer) Policies(namespace string) client.PolicyLister {
	return &indexerToPolicyLister{Indexer: i.GetIndexer(), namespace: namespace}
}

type indexerToPolicyLister struct {
	cache.Indexer
	namespace string
}

func (i *indexerToPolicyLister) List(options metav1.ListOptions) (*authorizationapi.PolicyList, error) {
	policyList := &authorizationapi.PolicyList{}
	labelSel, fieldSel, err := oapi.ListOptionsToSelectors(&options)
	if err != nil {
		return nil, err
	}
	matcher := policyregistry.Matcher(labelSel, fieldSel)

	if i.namespace == metav1.NamespaceAll {
		returnedList := i.Indexer.List()
		for i := range returnedList {
			policy := returnedList[i].(*authorizationapi.Policy)
			if matches, err := matcher.Matches(policy); err == nil && matches {
				policyList.Items = append(policyList.Items, *policy)
			}
		}
		return policyList, nil
	}

	key := &authorizationapi.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: i.namespace}}
	items, err := i.Indexer.Index(cache.NamespaceIndex, key)
	if err != nil {
		return policyList, err
	}

	for i := range items {
		policy := items[i].(*authorizationapi.Policy)
		if matches, err := matcher.Matches(policy); err == nil && matches {
			policyList.Items = append(policyList.Items, *policy)
		}
	}
	return policyList, nil
}

func (i *indexerToPolicyLister) Get(name string, options metav1.GetOptions) (*authorizationapi.Policy, error) {
	keyObj := &authorizationapi.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: i.namespace, Name: name}}
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := i.Indexer.GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kapierrors.NewNotFound(authorizationapi.Resource("policy"), name)
		return nil, existsErr
	}
	return item.(*authorizationapi.Policy), nil
}
