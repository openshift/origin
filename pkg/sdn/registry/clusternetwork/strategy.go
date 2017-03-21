package clusternetwork

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/api/validation"
)

// sdnStrategy implements behavior for ClusterNetworks
type sdnStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating ClusterNetwork
// objects via the REST API.
var Strategy = sdnStrategy{kapi.Scheme}

func (sdnStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for sdns
func (sdnStrategy) NamespaceScoped() bool {
	return false
}

func (sdnStrategy) GenerateName(base string) string {
	return base
}

func (sdnStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (sdnStrategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new sdn
func (sdnStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateClusterNetwork(obj.(*api.ClusterNetwork))
}

// AllowCreateOnUpdate is false for sdn
func (sdnStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (sdnStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for a ClusterNetwork
func (sdnStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateClusterNetworkUpdate(obj.(*api.ClusterNetwork), old.(*api.ClusterNetwork))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, error) {
	obj, ok := o.(*api.ClusterNetwork)
	if !ok {
		return nil, nil, fmt.Errorf("not a ClusterNetwork")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), nil
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
func SelectableFields(obj *api.ClusterNetwork) fields.Set {
	return api.ClusterNetworkToSelectableFields(obj)
}
