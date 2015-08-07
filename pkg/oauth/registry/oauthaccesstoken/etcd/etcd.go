package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/util"
)

// rest implements a RESTStorage for access tokens against etcd
type REST struct {
	// Cannot inline because we don't want the Update function
	store *etcdgeneric.Etcd
}

const EtcdPrefix = "/oauth/accesstokens"

// NewREST returns a RESTStorage object that will work against access tokens
func NewREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.OAuthAccessToken{} },
		NewListFunc: func() runtime.Object { return &api.OAuthAccessTokenList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthAccessToken).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return oauthaccesstoken.Matcher(label, field)
		},
		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*api.OAuthAccessToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},

		EndpointName: "oauthaccesstokens",

		Storage: s,
	}

	store.CreateStrategy = oauthaccesstoken.Strategy

	return &REST{store}
}

func (r *REST) New() runtime.Object {
	return r.store.NewFunc()
}

func (r *REST) NewList() runtime.Object {
	return r.store.NewListFunc()
}

func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return r.store.List(ctx, label, field)
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return r.store.Create(ctx, obj)
}

func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	return r.store.Delete(ctx, name, options)
}
