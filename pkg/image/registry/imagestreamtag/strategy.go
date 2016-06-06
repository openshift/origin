package imagestreamtag

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
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
	newIST := obj.(*api.ImageStreamTag)

	newIST.Conditions = nil
	newIST.Image = api.Image{}
}

func (s *strategy) GenerateName(base string) string {
	return base
}

func (s *strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	istag := obj.(*api.ImageStreamTag)

	return validation.ValidateImageStreamTag(istag)
}

func (s *strategy) AllowCreateOnUpdate() bool {
	return false
}

func (*strategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

func (s *strategy) PrepareForUpdate(obj, old runtime.Object) {
	newIST := obj.(*api.ImageStreamTag)
	oldIST := old.(*api.ImageStreamTag)

	// for backwards compatibility, callers can't be required to set both annotation locations when
	// doing a GET and then update.
	if newIST.Tag != nil {
		newIST.Tag.Annotations = newIST.Annotations
	}
	newIST.Conditions = oldIST.Conditions
	newIST.SelfLink = oldIST.SelfLink
	newIST.Image = oldIST.Image
}

func (s *strategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	newIST := obj.(*api.ImageStreamTag)
	oldIST := old.(*api.ImageStreamTag)

	return validation.ValidateImageStreamTagUpdate(newIST, oldIST)
}

// Export prepares the object for exporting.
func (s *strategy) Export(obj runtime.Object, exact bool) error {
	ist, ok := obj.(*api.ImageStreamTag)
	if !ok {
		return fmt.Errorf("unexpected object: %v", obj)
	}
	oapi.ExportObjectMeta(&ist.ObjectMeta, exact)
	oapi.ExportObjectMeta(&ist.Image.ObjectMeta, exact)
	return nil
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
