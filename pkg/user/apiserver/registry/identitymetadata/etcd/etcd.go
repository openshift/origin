package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"

	"github.com/openshift/api/user"
	printersinternal "github.com/openshift/origin/pkg/printers/internalversion"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/apiserver/registry/identitymetadata"
)

type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &userapi.IdentityMetadata{} },
		NewListFunc:              func() runtime.Object { return &userapi.IdentityMetadataList{} },
		DefaultQualifiedResource: user.Resource("identitymetadatas"),

		TableConvertor: printerstorage.TableConvertor{TablePrinter: printers.NewTablePrinter().With(printersinternal.AddHandlers)},

		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			// TODO use existing once fixed upstream
			metadata := obj.(*userapi.IdentityMetadata)
			expires := uint64(metadata.ExpiresIn)
			return expires, nil
		},

		CreateStrategy: identitymetadata.Strategy,
		UpdateStrategy: identitymetadata.Strategy,
		DeleteStrategy: identitymetadata.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}
