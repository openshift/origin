package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for images against etcd.
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a new REST.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &imageapi.Image{} },
		NewListFunc:              func() runtime.Object { return &imageapi.ImageList{} },
		DefaultQualifiedResource: imageapi.Resource("images"),

		CreateStrategy: image.Strategy,
		UpdateStrategy: image.Strategy,
		DeleteStrategy: image.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
