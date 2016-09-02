package build

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
)

// strategy implements behavior for Build objects
type strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Decorator is used to compute duration of a build since its not stored in etcd yet
var Decorator = func(obj runtime.Object) error {
	build, ok := obj.(*api.Build)
	if !ok {
		return errors.NewBadRequest(fmt.Sprintf("not a build: %v", build))
	}
	if build.Status.StartTimestamp == nil {
		build.Status.Duration = 0
	} else {
		completionTimestamp := build.Status.CompletionTimestamp
		if completionTimestamp == nil {
			dummy := unversioned.Now()
			completionTimestamp = &dummy
		}
		build.Status.Duration = completionTimestamp.Rfc3339Copy().Time.Sub(build.Status.StartTimestamp.Rfc3339Copy().Time)
	}
	return nil
}

// Strategy is the default logic that applies when creating and updating Build objects.
var Strategy = strategy{kapi.Scheme, kapi.SimpleNameGenerator}

func (strategy) NamespaceScoped() bool {
	return true
}

// AllowCreateOnUpdate is false for Build objects.
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	build := obj.(*api.Build)
	if len(build.Status.Phase) == 0 {
		build.Status.Phase = api.BuildPhaseNew
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	_ = obj.(*api.Build)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policy.
func (strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateBuild(obj.(*api.Build))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBuildUpdate(obj.(*api.Build), old.(*api.Build))
}

// CheckGracefulDelete allows a build to be gracefully deleted.
func (strategy) CheckGracefulDelete(obj runtime.Object, options *kapi.DeleteOptions) bool {
	return false
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			build, ok := obj.(*api.Build)
			if !ok {
				return nil, nil, fmt.Errorf("not a build")
			}
			return labels.Set(build.ObjectMeta.Labels), api.BuildToSelectableFields(build), nil
		},
	}
}

type detailsStrategy struct {
	strategy
}

// Prepares a build for update by only allowing an update to build details.
// For now, this is the Spec.Revision field
func (detailsStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	newBuild := obj.(*api.Build)
	oldBuild := old.(*api.Build)
	revision := newBuild.Spec.Revision
	*newBuild = *oldBuild
	newBuild.Spec.Revision = revision
}

// Validates that an update is valid by ensuring that no Revision exists and that it's not getting updated to blank
func (detailsStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	newBuild := obj.(*api.Build)
	oldBuild := old.(*api.Build)
	errors := field.ErrorList{}
	if oldBuild.Spec.Revision != nil {
		// If there was already a revision, then return an error
		errors = append(errors, field.Duplicate(field.NewPath("status", "revision"), oldBuild.Spec.Revision))
	}
	if newBuild.Spec.Revision == nil {
		errors = append(errors, field.Invalid(field.NewPath("status", "revision"), nil, "cannot set an empty revision in build status"))
	}
	return errors
}

// AllowUnconditionalUpdate returns true to allow a Build with an empty resourceVersion to update the Revision
func (detailsStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// DetailsStrategy is the strategy used to manage updates to a Build revision
var DetailsStrategy = detailsStrategy{Strategy}
