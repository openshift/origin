package projectrequest

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectvalidation "github.com/openshift/origin/pkg/project/apis/project/validation"
)

// strategy implements behavior for OAuthClient objects
type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{legacyscheme.Scheme}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for projectrequest objects
func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

// Validate validates a new client
func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	projectrequest := obj.(*projectapi.ProjectRequest)
	return projectvalidation.ValidateProjectRequest(projectrequest)
}

// ValidateUpdate validates a client update
func (strategy) ValidateUpdate(ctx context.Context, obj runtime.Object, old runtime.Object) field.ErrorList {
	return nil
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate is false for OAuth objects
func (strategy) AllowCreateOnUpdate() bool {
	return false
}
