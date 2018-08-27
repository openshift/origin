package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"

	"github.com/openshift/api/image"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageregistry "github.com/openshift/origin/pkg/image/apiserver/registry/image"
	printersinternal "github.com/openshift/origin/pkg/printers/internalversion"
)

// REST implements a RESTStorage for images against etcd.
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a new REST.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &imageapi.Image{} },
		NewListFunc:              func() runtime.Object { return &imageapi.ImageList{} },
		DefaultQualifiedResource: image.Resource("images"),

		TableConvertor: printerstorage.TableConvertor{TablePrinter: printers.NewTablePrinter().With(printersinternal.AddHandlers)},

		CreateStrategy: imageregistry.Strategy,
		UpdateStrategy: imageregistry.Strategy,
		DeleteStrategy: imageregistry.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
