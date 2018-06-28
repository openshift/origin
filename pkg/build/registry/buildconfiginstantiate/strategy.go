package buildconfiginstantiate

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildvalidation "github.com/openshift/origin/pkg/build/apis/build/validation"
)

type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{legacyscheme.Scheme}

func (strategy) NamespaceScoped() bool {
	return true
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new role.
func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return buildvalidation.ValidateBuildRequest(obj.(*buildapi.BuildRequest))
}

type binaryStrategy struct {
	runtime.ObjectTyper
}

var BinaryStrategy = binaryStrategy{legacyscheme.Scheme}

func (binaryStrategy) NamespaceScoped() bool {
	return true
}

func (binaryStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (binaryStrategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (binaryStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (binaryStrategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new role.
func (binaryStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	// TODO: validate
	return nil
}
