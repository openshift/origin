package build

import (
	"fmt"
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kstorage "k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// strategy implements behavior for Build objects
type strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
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

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, nil, fmt.Errorf("not a Build")
	}
	return labels.Set(build.ObjectMeta.Labels), api.BuildToSelectableFields(build), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

type detailsStrategy struct {
	strategy
}

// Prepares a build for update by only allowing an update to build details.
// Build details currently consists of: Spec.Revision, Status.Reason, and
// Status.Message, all of which are updated from within the build pod
func (detailsStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	newBuild := obj.(*api.Build)
	oldBuild := old.(*api.Build)

	// ignore phase updates unless the caller is updating the build to
	// a completed phase.
	phase := oldBuild.Status.Phase
	if buildutil.IsBuildComplete(newBuild) {
		phase = newBuild.Status.Phase
	}
	revision := newBuild.Spec.Revision
	message := newBuild.Status.Message
	reason := newBuild.Status.Reason
	outputTo := newBuild.Status.Output.To
	*newBuild = *oldBuild
	newBuild.Status.Phase = phase
	newBuild.Spec.Revision = revision
	newBuild.Status.Reason = reason
	newBuild.Status.Message = message
	newBuild.Status.Output.To = outputTo
}

// Validates that an update is valid by ensuring that no Revision exists and that it's not getting updated to blank
func (detailsStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	newBuild := obj.(*api.Build)
	oldBuild := old.(*api.Build)
	oldRevision := oldBuild.Spec.Revision
	newRevision := newBuild.Spec.Revision
	errors := field.ErrorList{}

	if newRevision == nil && oldRevision != nil {
		errors = append(errors, field.Invalid(field.NewPath("spec", "revision"), nil, "cannot set an empty revision in build spec"))
	}
	if !reflect.DeepEqual(oldRevision, newRevision) && oldRevision != nil {
		// If there was already a revision, then return an error
		errors = append(errors, field.Duplicate(field.NewPath("spec", "revision"), oldBuild.Spec.Revision))
	}
	return errors
}

// AllowUnconditionalUpdate returns true to allow a Build with an empty resourceVersion to update the Revision
func (detailsStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// DetailsStrategy is the strategy used to manage updates to a Build revision
var DetailsStrategy = detailsStrategy{Strategy}
