package instantiate

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
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

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate is a no-op for the instantiate endpoint.
func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
}

// PrepareForUpdate clears fields that are not allowed to be set by the instantiate endpoint.
func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	newDc := obj.(*api.DeploymentConfig)
	oldDc := old.(*api.DeploymentConfig)

	// Allow the status fields that need to be updated in every instantiation.
	oldStatus := oldDc.Status
	oldStatus.LatestVersion = newDc.Status.LatestVersion
	oldStatus.Details = newDc.Status.Details
	newDc.Status = oldStatus

	if !reflect.DeepEqual(oldDc.Spec, newDc.Spec) || newDc.Status.LatestVersion != oldDc.Status.LatestVersion {
		newDc.Generation = oldDc.Generation + 1
	}
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// CheckGracefulDelete allows a deployment config to be gracefully deleted.
func (strategy) CheckGracefulDelete(obj runtime.Object, options *kapi.DeleteOptions) bool {
	return false
}

// Validate is a no-op for the instantiate endpoint.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfig(obj.(*api.DeploymentConfig))
}

// ValidateUpdate is the default update validation for the instantiate endpoint.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfigUpdate(obj.(*api.DeploymentConfig), old.(*api.DeploymentConfig))
}
