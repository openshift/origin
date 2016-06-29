package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/controller/framework"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	"github.com/openshift/origin/pkg/client"
)

type InformerToClusterPolicyBindingLister struct {
	framework.SharedIndexInformer
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (i *InformerToClusterPolicyBindingLister) LastSyncResourceVersion() string {
	return i.SharedIndexInformer.LastSyncResourceVersion()
}

func (i *InformerToClusterPolicyBindingLister) ClusterPolicyBindings() client.ClusterPolicyBindingLister {
	return i
}

func (i *InformerToClusterPolicyBindingLister) List(options kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	clusterPolicyBindingList := &authorizationapi.ClusterPolicyBindingList{}
	returnedList := i.GetIndexer().List()
	matcher := clusterpolicybindingregistry.Matcher(oapi.ListOptionsToSelectors(&options))
	for i := range returnedList {
		clusterPolicyBinding := returnedList[i].(*authorizationapi.ClusterPolicyBinding)
		if matches, err := matcher.Matches(clusterPolicyBinding); err == nil && matches {
			clusterPolicyBindingList.Items = append(clusterPolicyBindingList.Items, *clusterPolicyBinding)
		}
	}
	return clusterPolicyBindingList, nil
}

func (i *InformerToClusterPolicyBindingLister) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	keyObj := &authorizationapi.ClusterPolicyBinding{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := i.GetIndexer().GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicybinding"), name)
		return nil, existsErr
	}
	return item.(*authorizationapi.ClusterPolicyBinding), nil
}
