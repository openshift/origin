package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for oauth clients against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against oauth clients
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.OAuthClient{} },
		NewListFunc:       func() runtime.Object { return &api.OAuthClientList{} },
		PredicateFunc:     oauthclient.Matcher,
		QualifiedResource: api.Resource("oauthclients"),

		CreateStrategy: oauthclient.Strategy,
		UpdateStrategy: oauthclient.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: oauthclient.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
