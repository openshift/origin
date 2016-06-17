package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/restoptions"
)

const ClusterPolicyPath = "/authorization/cluster/policies"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.ClusterPolicy{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.ClusterPolicyList{} },
		QualifiedResource: authorizationapi.Resource("clusterpolicies"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return ClusterPolicyPath
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, ClusterPolicyPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.ClusterPolicy).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusterpolicy.Matcher(label, field)
		},

		CreateStrategy: clusterpolicy.Strategy,
		UpdateStrategy: clusterpolicy.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, ClusterPolicyPath); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
