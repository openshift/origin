package imagestreamtag

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

// strategy implements behavior for ImageStreamTags.
type strategy struct {
	runtime.ObjectTyper
}

var Strategy = &strategy{
	ObjectTyper: kapi.Scheme,
}

func (s *strategy) NamespaceScoped() bool {
	return true
}

func (s *strategy) PrepareForCreate(obj runtime.Object) {
}

func (s *strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	istag := obj.(*api.ImageStreamTag)

	return validation.ValidateImageStreamTag(istag)
}

func (s *strategy) AllowCreateOnUpdate() bool {
	return false
}

func (*strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (s *strategy) PrepareForUpdate(obj, old runtime.Object) {
	newIST := obj.(*api.ImageStreamTag)
	oldIST := old.(*api.ImageStreamTag)

	newIST.SelfLink = oldIST.SelfLink
}

func (s *strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	newIST := obj.(*api.ImageStreamTag)
	oldIST := old.(*api.ImageStreamTag)

	return validation.ValidateImageStreamTagUpdate(newIST, oldIST)
}

// MatchImageStreamTag returns a generic matcher for a given label and field selector.
func MatchImageStreamTag(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		target, ok := obj.(*api.ImageStreamTag)
		if !ok {
			return false, fmt.Errorf("not an ImageStreamTag")
		}
		fields := ImageStreamToSelectableFields(target)
		return label.Matches(labels.Set(target.Labels)) && field.Matches(fields), nil
	})
}

// ImageStreamToSelectableFields returns a label set that represents the object.
func ImageStreamToSelectableFields(target *api.ImageStreamTag) labels.Set {
	return labels.Set{
		"metadata.name": target.Name,
	}
}
