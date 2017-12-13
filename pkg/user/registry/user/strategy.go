package user

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/apis/user/validation"
)

// userStrategy implements behavior for Users
type userStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating User
// objects via the REST API.
var Strategy = userStrategy{legacyscheme.Scheme}

var _ rest.GarbageCollectionDeleteStrategy = userStrategy{}

func (userStrategy) DefaultGarbageCollectionPolicy(ctx apirequest.Context) rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (userStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for users
func (userStrategy) NamespaceScoped() bool {
	return false
}

func (userStrategy) GenerateName(base string) string {
	return base
}

func (userStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Validate validates a new user
func (userStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateUser(obj.(*userapi.User))
}

// AllowCreateOnUpdate is false for users
func (userStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (userStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (userStrategy) Canonicalize(obj runtime.Object) {
}

// ValidateUpdate is the default update validation for an end user.
func (userStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateUserUpdate(obj.(*userapi.User), old.(*userapi.User))
}
