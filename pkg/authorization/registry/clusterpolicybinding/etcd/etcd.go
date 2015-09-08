package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	"github.com/openshift/origin/pkg/util"
)

const ClusterPolicyBindingPath = "/authorization/cluster/policybindings"

type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:      func() runtime.Object { return &authorizationapi.ClusterPolicyBinding{} },
		NewListFunc:  func() runtime.Object { return &authorizationapi.ClusterPolicyBindingList{} },
		EndpointName: "clusterpolicybinding",
		KeyRootFunc: func(ctx kapi.Context) string {
			return ClusterPolicyBindingPath
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, ClusterPolicyBindingPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.ClusterPolicyBinding).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusterpolicybinding.Matcher(label, field)
		},

		CreateStrategy: clusterpolicybinding.Strategy,
		UpdateStrategy: clusterpolicybinding.Strategy,

		Storage: s,
	}

	return &REST{store}
}
