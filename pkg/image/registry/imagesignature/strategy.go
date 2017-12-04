package imagesignature

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation"
)

// strategy implements behavior for ImageStreamTags.
type strategy struct {
	runtime.ObjectTyper
}

var Strategy = &strategy{
	ObjectTyper: legacyscheme.Scheme,
}

func (s *strategy) NamespaceScoped() bool {
	return false
}

func (s *strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
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

func (s *strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
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
