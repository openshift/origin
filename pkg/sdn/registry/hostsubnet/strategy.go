package hostsubnet

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
	"github.com/openshift/origin/pkg/sdn/apis/network/validation"
)

// sdnStrategy implements behavior for HostSubnets
type sdnStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating HostSubnet
// objects via the REST API.
var Strategy = sdnStrategy{kapi.Scheme}

func (sdnStrategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (sdnStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

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

func (sdnStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Validate validates a new sdn
func (sdnStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateHostSubnet(obj.(*sdnapi.HostSubnet))
}

// AllowCreateOnUpdate is false for sdns
func (sdnStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (sdnStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for a HostSubnet
func (sdnStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateHostSubnetUpdate(obj.(*sdnapi.HostSubnet), old.(*sdnapi.HostSubnet))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, bool, error) {
	obj, ok := o.(*sdnapi.HostSubnet)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a HostSubnet")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *sdnapi.HostSubnet) fields.Set {
	return sdnapi.HostSubnetToSelectableFields(obj)
}
