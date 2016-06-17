package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for oauth clients against etcd
type REST struct {
	registry.Store
}

const EtcdPrefix = "/oauth/clients"

// NewREST returns a RESTStorage object that will work against oauth clients
func NewREST(optsGetter restoptions.Getter) (*REST, error) {

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.OAuthClient{} },
		NewListFunc: func() runtime.Object { return &api.OAuthClientList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthClient).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return oauthclient.Matcher(label, field)
		},
		QualifiedResource: api.Resource("oauthclients"),

		CreateStrategy: oauthclient.Strategy,
		UpdateStrategy: oauthclient.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, EtcdPrefix); err != nil {
		return nil, err
	}

	return &REST{*store}, nil
}
