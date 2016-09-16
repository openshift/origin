package buildconfig

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
)

// strategy implements behavior for BuildConfig objects
type strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating BuildConfig objects.
var Strategy = strategy{kapi.Scheme, kapi.SimpleNameGenerator}

func (strategy) NamespaceScoped() bool {
	return true
}

// AllowCreateOnUpdate is false for BuildConfig objects.
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	bc := obj.(*api.BuildConfig)
	dropUnknownTriggers(bc)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	newBC := obj.(*api.BuildConfig)
	oldBC := old.(*api.BuildConfig)
	dropUnknownTriggers(newBC)
	// Do not allow the build version to go backwards or we'll
	// get conflicts with existing builds.
	if newBC.Status.LastVersion < oldBC.Status.LastVersion {
		newBC.Status.LastVersion = oldBC.Status.LastVersion
	}
}

// Validate validates a new policy.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateBuildConfig(obj.(*api.BuildConfig))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBuildConfigUpdate(obj.(*api.BuildConfig), old.(*api.BuildConfig))
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			buildConfig, ok := obj.(*api.BuildConfig)
			if !ok {
				return nil, nil, fmt.Errorf("not a BuildConfig")
			}
			return labels.Set(buildConfig.ObjectMeta.Labels), api.BuildConfigToSelectableFields(buildConfig), nil
		},
	}
}

// CheckGracefulDelete allows a build config to be gracefully deleted.
func (strategy) CheckGracefulDelete(obj runtime.Object, options *kapi.DeleteOptions) bool {
	return false
}

// dropUnknownTriggers drops any triggers that are of an unknown type
func dropUnknownTriggers(bc *api.BuildConfig) {
	triggers := []api.BuildTriggerPolicy{}
	for _, t := range bc.Spec.Triggers {
		if api.KnownTriggerTypes.Has(string(t.Type)) {
			triggers = append(triggers, t)
		}
	}
	bc.Spec.Triggers = triggers
}
