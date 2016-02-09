package deployconfig

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
)

// strategy implements behavior for DeploymentConfig objects
type strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating DeploymentConfig objects.
var Strategy = strategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is true for DeploymentConfig objects.
func (strategy) NamespaceScoped() bool {
	return true
}

// AllowCreateOnUpdate is false for DeploymentConfig objects.
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(obj runtime.Object) {
	dc := obj.(*api.DeploymentConfig)
	dc.Status = api.DeploymentConfigStatus{}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(obj, old runtime.Object) {
	newDc := obj.(*api.DeploymentConfig)
	oldDc := old.(*api.DeploymentConfig)
	newDc.Status = oldDc.Status
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policy.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfig(obj.(*api.DeploymentConfig))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfigUpdate(obj.(*api.DeploymentConfig), old.(*api.DeploymentConfig))
}

// CheckGracefulDelete allows a build to be gracefully deleted.
func (strategy) CheckGracefulDelete(obj runtime.Object, options *kapi.DeleteOptions) bool {
	return false
}

type statusStrategy struct {
	strategy
}

var StatusStrategy = statusStrategy{Strategy}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update of status.
func (statusStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newDc := obj.(*api.DeploymentConfig)
	oldDc := old.(*api.DeploymentConfig)
	newDc.Spec = oldDc.Spec
	newDc.Labels = oldDc.Labels
	newDc.Annotations = oldDc.Annotations
}

// ValidateUpdate is the default update validation for an end user updating status.
func (statusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateDeploymentConfigUpdate(obj.(*api.DeploymentConfig), old.(*api.DeploymentConfig))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) generic.Matcher {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			deploymentConfig, ok := obj.(*api.DeploymentConfig)
			if !ok {
				return nil, nil, fmt.Errorf("not a DeploymentConfig")
			}
			return labels.Set(deploymentConfig.ObjectMeta.Labels), api.DeploymentConfigToSelectableFields(deploymentConfig), nil
		},
	}
}
