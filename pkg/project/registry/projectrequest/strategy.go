package projectrequest

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	projectapi "github.com/openshift/origin/pkg/project/api"
	projectvalidation "github.com/openshift/origin/pkg/project/api/validation"
)

// strategy implements behavior for OAuthClient objects
type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{kapi.Scheme}

func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for projectrequest objects
func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// Validate validates a new client
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	projectrequest := obj.(*projectapi.ProjectRequest)
	return projectvalidation.ValidateProjectRequest(projectrequest)
}

// ValidateUpdate validates a client update
func (strategy) ValidateUpdate(ctx kapi.Context, obj runtime.Object, old runtime.Object) field.ErrorList {
	return nil
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate is false for OAuth objects
func (strategy) AllowCreateOnUpdate() bool {
	return false
}
