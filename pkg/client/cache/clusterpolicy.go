package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/controller/framework"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/client"
)

type InformerToClusterPolicyLister struct {
	framework.SharedIndexInformer
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (i *InformerToClusterPolicyLister) LastSyncResourceVersion() string {
	return i.SharedIndexInformer.LastSyncResourceVersion()
}

func (i *InformerToClusterPolicyLister) ClusterPolicies() client.ClusterPolicyLister {
	return i
}

func (i *InformerToClusterPolicyLister) List(options kapi.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	clusterPolicyList := &authorizationapi.ClusterPolicyList{}
	returnedList := i.GetIndexer().List()
	matcher := clusterpolicyregistry.Matcher(oapi.ListOptionsToSelectors(&options))
	for i := range returnedList {
		clusterPolicy := returnedList[i].(*authorizationapi.ClusterPolicy)
		if matches, err := matcher.Matches(clusterPolicy); err == nil && matches {
			clusterPolicyList.Items = append(clusterPolicyList.Items, *clusterPolicy)
		}
	}
	return clusterPolicyList, nil
}

func (i *InformerToClusterPolicyLister) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	keyObj := &authorizationapi.ClusterPolicy{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := framework.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

	item, exists, getErr := i.GetIndexer().GetByKey(key)
	if getErr != nil {
		return nil, getErr
	}
	if !exists {
		existsErr := kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicy"), name)
		return nil, existsErr
	}
	return item.(*authorizationapi.ClusterPolicy), nil
}
