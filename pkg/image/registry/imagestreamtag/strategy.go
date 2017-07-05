package imagestreamtag

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
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

func (s *strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	newIST := obj.(*imageapi.ImageStreamTag)

	newIST.Conditions = nil
	newIST.Image = imageapi.Image{}
}

func (s *strategy) GenerateName(base string) string {
	return base
}

func (s *strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	istag := obj.(*imageapi.ImageStreamTag)

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

func (s *strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	newIST := obj.(*imageapi.ImageStreamTag)
	oldIST := old.(*imageapi.ImageStreamTag)

	// for backwards compatibility, callers can't be required to set both annotation locations when
	// doing a GET and then update.
	if newIST.Tag != nil {
		newIST.Tag.Annotations = newIST.Annotations
	}
	newIST.Conditions = oldIST.Conditions
	newIST.SelfLink = oldIST.SelfLink
	newIST.Image = oldIST.Image
}

func (s *strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	newIST := obj.(*imageapi.ImageStreamTag)
	oldIST := old.(*imageapi.ImageStreamTag)

	return validation.ValidateImageStreamTagUpdate(newIST, oldIST)
}

// MatchImageStreamTag returns a generic matcher for a given label and field selector.
func MatchImageStreamTag(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, bool, error) {
			obj, ok := o.(*imageapi.ImageStreamTag)
			if !ok {
				return nil, nil, false, fmt.Errorf("not an ImageStreamTag")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *imageapi.ImageStreamTag) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
}
