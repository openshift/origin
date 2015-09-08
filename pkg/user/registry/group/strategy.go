package group

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

// groupStrategy implements behavior for Groups
type groupStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating Group
// objects via the REST API.
var Strategy = groupStrategy{kapi.Scheme}

func (groupStrategy) PrepareForUpdate(obj, old runtime.Object) {}

// NamespaceScoped is false for groups
func (groupStrategy) NamespaceScoped() bool {
	return false
}

func (groupStrategy) GenerateName(base string) string {
	return base
}

func (groupStrategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new group
func (groupStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateGroup(obj.(*api.Group))
}

// AllowCreateOnUpdate is false for groups
func (groupStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (groupStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end group.
func (groupStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateGroupUpdate(obj.(*api.Group), old.(*api.Group))
}

// MatchGroup returns a generic matcher for a given label and field selector.
func MatchGroup(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		groupObj, ok := obj.(*api.Group)
		if !ok {
			return false, fmt.Errorf("not a group")
		}
		fields := GroupToSelectableFields(groupObj)
		return label.Matches(labels.Set(groupObj.Labels)) && field.Matches(fields), nil
	})
}

// GroupToSelectableFields returns a label set that represents the object
func GroupToSelectableFields(group *api.Group) labels.Set {
	return labels.Set{
		"name": group.Name,
	}
}
