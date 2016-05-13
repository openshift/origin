package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	"github.com/openshift/origin/pkg/util"
)

const ClusterPolicyBindingPath = "/authorization/cluster/policybindings"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &authorizationapi.ClusterPolicyBindingList{} }

	storageInterface := opts.Decorator(opts.Storage, 100, &authorizationapi.ClusterPolicyBindingList{}, ClusterPolicyBindingPath, clusterpolicybinding.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.ClusterPolicyBinding{} },
		NewListFunc:       newListFunc,
		QualifiedResource: authorizationapi.Resource("clusterpolicybinding"),
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

		Storage: storageInterface,
	}

	return &REST{store}
}
