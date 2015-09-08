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
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/util"
)

const ClusterPolicyPath = "/authorization/cluster/policies"

type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:      func() runtime.Object { return &authorizationapi.ClusterPolicy{} },
		NewListFunc:  func() runtime.Object { return &authorizationapi.ClusterPolicyList{} },
		EndpointName: "clusterpolicy",
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

		Storage: s,
	}

	return &REST{store}
}
