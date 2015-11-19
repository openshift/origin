package buildconfiginstantiate

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildvalidation "github.com/openshift/origin/pkg/build/api/validation"
)

type strategy struct {
	runtime.ObjectTyper
}

var Strategy = strategy{kapi.Scheme}

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
func (strategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new role.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return buildvalidation.ValidateBuildRequest(obj.(*buildapi.BuildRequest))
}

type binaryStrategy struct {
	runtime.ObjectTyper
}

var BinaryStrategy = binaryStrategy{kapi.Scheme}

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
func (binaryStrategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new role.
func (binaryStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	// TODO: validate
	return nil
}
