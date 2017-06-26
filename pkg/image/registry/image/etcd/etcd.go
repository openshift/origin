package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for images against etcd.
type REST struct {
	*registry.Store
}

// NewREST returns a new REST.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &imageapi.Image{} },
		NewListFunc:       func() runtime.Object { return &imageapi.ImageList{} },
		PredicateFunc:     image.Matcher,
		QualifiedResource: imageapi.Resource("images"),

		CreateStrategy: image.Strategy,
		UpdateStrategy: image.Strategy,
		DeleteStrategy: image.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: image.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
