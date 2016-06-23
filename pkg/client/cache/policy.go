package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	"github.com/openshift/origin/pkg/client"
)

type InformerToPolicyNamespacer struct {
	framework.SharedIndexInformer
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

func (i *indexerToPolicyLister) List(options kapi.ListOptions) (*authorizationapi.PolicyList, error) {
	policyList := &authorizationapi.PolicyList{}
	matcher := policyregistry.Matcher(oapi.ListOptionsToSelectors(&options))

	if i.namespace == kapi.NamespaceAll {
		returnedList := i.Indexer.List()
		for i := range returnedList {
			policy := returnedList[i].(*authorizationapi.Policy)
			if matches, err := matcher.Matches(policy); err == nil && matches {
				policyList.Items = append(policyList.Items, *policy)
			}
		}
		return policyList, nil
	}

	key := &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: i.namespace}}
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

func (i *indexerToPolicyLister) Get(name string) (*authorizationapi.Policy, error) {
	keyObj := &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: i.namespace, Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

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
