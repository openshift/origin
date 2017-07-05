package group

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/apis/user/validation"
)

// groupStrategy implements behavior for Groups
type groupStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating Group
// objects via the REST API.
var Strategy = groupStrategy{kapi.Scheme}

func (groupStrategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (groupStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for groups
func (groupStrategy) NamespaceScoped() bool {
	return false
}

func (groupStrategy) GenerateName(base string) string {
	return base
}

func (groupStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Validate validates a new group
func (groupStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateGroup(obj.(*userapi.Group))
}

// AllowCreateOnUpdate is false for groups
func (groupStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (groupStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (groupStrategy) Canonicalize(obj runtime.Object) {
}

// ValidateUpdate is the default update validation for an end group.
func (groupStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateGroupUpdate(obj.(*userapi.Group), old.(*userapi.Group))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, bool, error) {
	obj, ok := o.(*userapi.Group)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a Group")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *userapi.Group) fields.Set {
	return userapi.GroupToSelectableFields(obj)
}
