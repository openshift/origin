package user

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

// userStrategy implements behavior for Users
type userStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating User
// objects via the REST API.
var Strategy = userStrategy{kapi.Scheme}

func (userStrategy) PrepareForUpdate(obj, old runtime.Object) {}

// NamespaceScoped is false for users
func (userStrategy) NamespaceScoped() bool {
	return false
}

func (userStrategy) GenerateName(base string) string {
	return base
}

func (userStrategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new user
func (userStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateUser(obj.(*api.User))
}

// AllowCreateOnUpdate is false for users
func (userStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (userStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (userStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateUserUpdate(obj.(*api.User), old.(*api.User))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		userObj, ok := obj.(*api.User)
		if !ok {
			return false, fmt.Errorf("not a user")
		}
		fields := api.UserToSelectableFields(userObj)
		return label.Matches(labels.Set(userObj.Labels)) && field.Matches(fields), nil
	})
}
