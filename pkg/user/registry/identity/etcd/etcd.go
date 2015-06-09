package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/identity"
)

// REST implements a RESTStorage for identites against etcd
type REST struct {
	etcdgeneric.Etcd
}

const EtcdPrefix = "/useridentities"

// NewREST returns a RESTStorage object that will work against identites
func NewREST(h tools.EtcdHelper) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.Identity{} },
		NewListFunc: func() runtime.Object { return &api.IdentityList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			// TODO: JTL: switch to NoNamespaceKeyRootFunc after rebase
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Identity).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return identity.MatchIdentity(label, field)
		},
		EndpointName: "identities",

		Helper: h,
	}

	store.CreateStrategy = identity.Strategy
	store.UpdateStrategy = identity.Strategy

	return &REST{*store}
}
