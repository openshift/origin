package deployconfig

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

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

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(obj runtime.Object) {
	_ = obj.(*api.DeploymentConfig)
	// TODO: need to ensure status.latestVersion is not set out of order
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(obj, old runtime.Object) {
	_ = obj.(*api.DeploymentConfig)
	// TODO: need to ensure status.latestVersion is not set out of order
}

// Validate validates a new policy.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateDeploymentConfig(obj.(*api.DeploymentConfig))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateDeploymentConfigUpdate(obj.(*api.DeploymentConfig), old.(*api.DeploymentConfig))
}

// CheckGracefulDelete allows a build to be gracefully deleted.
func (strategy) CheckGracefulDelete(obj runtime.Object, options *kapi.DeleteOptions) bool {
	return false
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
			return labels.Set(deploymentConfig.ObjectMeta.Labels), SelectableFields(deploymentConfig), nil
		},
	}
}

// SelectableFields returns a label set that represents the object
func SelectableFields(deploymentConfig *api.DeploymentConfig) fields.Set {
	return fields.Set{
		"metadata.name": deploymentConfig.Name,
	}
}
