package imagestreamimport

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// strategy implements behavior for ImageStreamImports.
type strategy struct {
	runtime.ObjectTyper
}

var Strategy = &strategy{kapi.Scheme}

func (s *strategy) NamespaceScoped() bool {
	return true
}

func (s *strategy) GenerateName(string) string {
	return ""
}

func (s *strategy) Canonicalize(runtime.Object) {
}

func (s *strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	newIST := obj.(*api.ImageStreamImport)
	newIST.Status = api.ImageStreamImportStatus{}
}

func (s *strategy) PrepareImageForCreate(obj runtime.Object) {
	image := obj.(*api.Image)

	// signatures can be added using "images" or "imagesignatures" resources
	image.Signatures = nil
}

func (s *strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	isi := obj.(*api.ImageStreamImport)
	return validation.ValidateImageStreamImport(isi)
}
