package imagesignature

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	imageapi "github.com/openshift/origin/pkg/image/api"
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
	return false
}

func (s *strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	signature := obj.(*imageapi.ImageSignature)

	signature.Conditions = nil
	signature.ImageIdentity = ""
	signature.SignedClaims = nil
	signature.Created = nil
	signature.IssuedBy = nil
	signature.IssuedTo = nil
}

func (s *strategy) GenerateName(base string) string {
	return base
}

func (s *strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	signature := obj.(*imageapi.ImageSignature)

	return validation.ValidateImageSignature(signature)
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
