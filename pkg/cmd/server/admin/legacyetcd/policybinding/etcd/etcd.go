package etcd

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/policybinding"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against PolicyBinding objects.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:                   kapi.Scheme,
		NewFunc:                  func() runtime.Object { return &authorizationapi.PolicyBinding{} },
		NewListFunc:              func() runtime.Object { return &authorizationapi.PolicyBindingList{} },
		DefaultQualifiedResource: authorizationapi.Resource("policybindings"),

		CreateStrategy: policybinding.Strategy,
		UpdateStrategy: policybinding.Strategy,
		DeleteStrategy: policybinding.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter,
		AttrFunc: storage.AttrFunc(storage.DefaultNamespaceScopedAttr).WithFieldMutation(FieldSetMutator)}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}

func FieldSetMutator(obj runtime.Object, fieldSet fields.Set) error {
	policyBinding, ok := obj.(*authorizationapi.PolicyBinding)
	if !ok {
		return fmt.Errorf("%T not a PolicyBinding", obj)
	}
	fieldSet["policyRef.namespace"] = policyBinding.PolicyRef.Namespace
	return nil
}
