package rangeallocations

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"

	"github.com/openshift/api/security"
	printersinternal "github.com/openshift/origin/pkg/printers/internalversion"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

type REST struct {
	*genericregistry.Store
}

var _ rest.StandardStorage = &REST{}

func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &securityapi.RangeAllocation{} },
		NewListFunc:              func() runtime.Object { return &securityapi.RangeAllocationList{} },
		DefaultQualifiedResource: security.Resource("rangeallocations"),

		TableConvertor: printerstorage.TableConvertor{TablePrinter: printers.NewTablePrinter().With(printersinternal.AddHandlers)},

		CreateStrategy: strategyInstance,
		UpdateStrategy: strategyInstance,
		DeleteStrategy: strategyInstance,
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}
	return &REST{store}

}
