package cache

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/client"
)

type InformerToClusterPolicyLister struct {
	cache.SharedIndexInformer
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (i *InformerToClusterPolicyLister) LastSyncResourceVersion() string {
	return i.SharedIndexInformer.LastSyncResourceVersion()
}

func (i *InformerToClusterPolicyLister) ClusterPolicies() client.ClusterPolicyLister {
	return i
}

func (i *InformerToClusterPolicyLister) List(options metav1.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	clusterPolicyList := &authorizationapi.ClusterPolicyList{}
	returnedList := i.GetIndexer().List()
	labelSel, fieldSel, err := oapi.ListOptionsToSelectors(&options)
	if err != nil {
		return nil, err
	}
	matcher := clusterpolicyregistry.Matcher(labelSel, fieldSel)
	for i := range returnedList {
		clusterPolicy := returnedList[i].(*authorizationapi.ClusterPolicy)
		if matches, err := matcher.Matches(clusterPolicy); err == nil && matches {
			clusterPolicyList.Items = append(clusterPolicyList.Items, *clusterPolicy)
		}
	}
	return clusterPolicyList, nil
}

func (i *InformerToClusterPolicyLister) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicy, error) {
	keyObj := &authorizationapi.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(keyObj)

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
