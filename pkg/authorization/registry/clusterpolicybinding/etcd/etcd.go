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
	"github.com/openshift/origin/pkg/util/restoptions"
)

const ClusterPolicyBindingPath = "/authorization/cluster/policybindings"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.ClusterPolicyBinding{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.ClusterPolicyBindingList{} },
		QualifiedResource: authorizationapi.Resource("clusterpolicybindings"),
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
	}

	if err := restoptions.ApplyOptions(optsGetter, store, ClusterPolicyBindingPath); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
