package instantiate

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/apps/apis/apps/validation"
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

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (strategy) GenerateName(base string) string {
	return base
}

// PrepareForCreate is a no-op for the instantiate endpoint.
func (strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// PrepareForUpdate clears fields that are not allowed to be set by the instantiate endpoint.
func (strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	newDc := obj.(*appsapi.DeploymentConfig)
	oldDc := old.(*appsapi.DeploymentConfig)

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
func (strategy) CheckGracefulDelete(obj runtime.Object, options *metav1.DeleteOptions) bool {
	return false
}

// Validate is a no-op for the instantiate endpoint.
func (strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfig(obj.(*appsapi.DeploymentConfig))
}

// ValidateUpdate is the default update validation for the instantiate endpoint.
func (strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfigUpdate(obj.(*appsapi.DeploymentConfig), old.(*appsapi.DeploymentConfig))
}
