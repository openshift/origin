package cache

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	"github.com/openshift/origin/pkg/client"
)

type InformerToClusterPolicyBindingLister struct {
	cache.SharedIndexInformer
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (i *InformerToClusterPolicyBindingLister) LastSyncResourceVersion() string {
	return i.SharedIndexInformer.LastSyncResourceVersion()
}

func (i *InformerToClusterPolicyBindingLister) ClusterPolicyBindings() client.ClusterPolicyBindingLister {
	return i
}

func (i *InformerToClusterPolicyBindingLister) List(options metav1.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	clusterPolicyBindingList := &authorizationapi.ClusterPolicyBindingList{}
	returnedList := i.GetIndexer().List()
	labelSel, fieldSel, err := oapi.ListOptionsToSelectors(&options)
	if err != nil {
		return nil, err
	}
	matcher := clusterpolicybindingregistry.Matcher(labelSel, fieldSel)
	for i := range returnedList {
		clusterPolicyBinding := returnedList[i].(*authorizationapi.ClusterPolicyBinding)
		if matches, err := matcher.Matches(clusterPolicyBinding); err == nil && matches {
			clusterPolicyBindingList.Items = append(clusterPolicyBindingList.Items, *clusterPolicyBinding)
		}
	}
	return clusterPolicyBindingList, nil
}

func (i *InformerToClusterPolicyBindingLister) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicyBinding, error) {
	keyObj := &authorizationapi.ClusterPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: name}}
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

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
