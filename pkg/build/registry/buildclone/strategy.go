package buildclone

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildvalidation "github.com/openshift/origin/pkg/build/api/validation"
)

type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{kapi.Scheme}

func (s strategy) NamespaceScoped() bool {
	return true
}

func (s strategy) AllowCreateOnUpdate() bool {
	return false
}

func (s strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// Validate validates a new role.
func (s strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return buildvalidation.ValidateBuildRequest(obj.(*buildapi.BuildRequest))
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}
