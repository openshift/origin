package user

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

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

// MatchUser returns a generic matcher for a given label and field selector.
func MatchUser(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		userObj, ok := obj.(*api.User)
		if !ok {
			return false, fmt.Errorf("not a user")
		}
		fields := UserToSelectableFields(userObj)
		return label.Matches(labels.Set(userObj.Labels)) && field.Matches(fields), nil
	})
}

// UserToSelectableFields returns a label set that represents the object
func UserToSelectableFields(user *api.User) labels.Set {
	return labels.Set{
		"name": user.Name,
	}
}
