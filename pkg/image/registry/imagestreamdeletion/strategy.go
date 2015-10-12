package imagestreamdeletion

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
)

type ResourceGetter interface {
	Get(kapi.Context, string) (runtime.Object, error)
}

// Strategy implements behavior for ImageStreamDeletions.
type Strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
	ImageStreamDeletionGetter ResourceGetter
}

// NewStrategy is the default logic that applies when creating
// ImageStreamDeletion objects via the REST API.
func NewStrategy(subjectAccessReviewClient subjectaccessreview.Registry) Strategy {
	return Strategy{
		ObjectTyper:   kapi.Scheme,
		NameGenerator: kapi.SimpleNameGenerator,
	}
}

// NamespaceScoped is true for image streams.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation,
// and verifies the current user is authorized to access any image streams newly referenced
// in spec.tags.
func (s Strategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new image stream deletion.
func (s Strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	stream := obj.(*api.ImageStreamDeletion)
	_, ok := kapi.UserFrom(ctx)
	if !ok {
		return fielderrors.ValidationErrorList{kerrors.NewForbidden("imageStreamDeletion", stream.Name, fmt.Errorf("unable to update an ImageStreamDeletion without a user on the context"))}
	}
	return nil
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
func ImageStreamDeletionToSelectableFields(ir *api.ImageStreamDeletion) labels.Set {
	return labels.Set{"metadata.name": ir.Name}
}
