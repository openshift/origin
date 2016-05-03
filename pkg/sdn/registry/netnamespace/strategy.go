package netnamespace

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/api/validation"
)

// sdnStrategy implements behavior for NetNamespaces
type sdnStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating NetNamespace
// objects via the REST API.
var Strategy = sdnStrategy{kapi.Scheme}

// Canonicalize normalizes the object after validation.
func (sdnStrategy) Canonicalize(obj runtime.Object) {
}

// NamespaceScoped is false for sdns
func (sdnStrategy) NamespaceScoped() bool {
	return false
}

func (sdnStrategy) GenerateName(base string) string {
	return base
}

func (sdnStrategy) PrepareForCreate(obj runtime.Object) {
}

func (sdnStrategy) PrepareForUpdate(obj, old runtime.Object) {
}

// AllowCreateOnUpdate is false for NetNamespace
func (sdnStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (sdnStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Validate validates a new NetNamespace
func (sdnStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	newObj := obj.(*api.NetNamespace)
	oldObj := old.(*api.NetNamespace)
	errList := validation.ValidateNetNamespace(newObj)
	return append(errList, validation.ValidateNetNamespaceUpdate(newObj, oldObj)...)
}
