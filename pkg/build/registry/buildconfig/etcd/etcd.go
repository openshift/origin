package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/registry/buildconfig"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against BuildConfig.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &buildapi.BuildConfig{} },
		NewListFunc:       func() runtime.Object { return &buildapi.BuildConfigList{} },
		QualifiedResource: buildapi.Resource("buildconfigs"),
		PredicateFunc:     buildconfig.Matcher,

		CreateStrategy: buildconfig.GroupStrategy,
		UpdateStrategy: buildconfig.GroupStrategy,
		DeleteStrategy: buildconfig.GroupStrategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: buildconfig.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
