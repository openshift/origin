package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
)

// rest implements a RESTStorage for oauth clients against etcd
type REST struct {
	etcdgeneric.Etcd
}

const EtcdPrefix = "/registry/oauth/clients"

// NewREST returns a RESTStorage object that will work against oauth clients
func NewREST(h tools.EtcdHelper) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.OAuthClient{} },
		NewListFunc: func() runtime.Object { return &api.OAuthClientList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			// TODO: JTL: switch to NoNamespaceKeyRootFunc after rebase
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthClient).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return oauthclient.Matcher(label, field)
		},
		EndpointName: "oauthclients",

		Helper: h,
	}

	store.CreateStrategy = oauthclient.Strategy
	store.UpdateStrategy = oauthclient.Strategy

	return &REST{*store}
}
