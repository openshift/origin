package imagestreamdeletion

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// Strategy implements behavior for ImageStreamDeletions.
type imageStreamDeletionStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating
// ImageStreamDeletion objects via the REST API.
var Strategy = imageStreamDeletionStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is false for image stream deletions.
func (s imageStreamDeletionStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s imageStreamDeletionStrategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new image stream deletion.
func (s imageStreamDeletionStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	deletion := obj.(*api.ImageStreamDeletion)
	return validation.ValidateImageStreamDeletion(deletion)
}

// MatchImageStreamDeletion returns a generic matcher for a given label and field selector.
func MatchImageStreamDeletion(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		ir, ok := obj.(*api.ImageStreamDeletion)
		if !ok {
			return false, fmt.Errorf("not an ImageStreamDeletion")
		}
		fields := ImageStreamDeletionToSelectableFields(ir)
		return label.Matches(labels.Set(ir.Labels)) && field.Matches(fields), nil
	})
}

// ImageStreamDeletionToSelectableFields returns a label set that represents the object.
func ImageStreamDeletionToSelectableFields(deletion *api.ImageStreamDeletion) labels.Set {
	return labels.Set{"metadata.name": deletion.Name}
}
