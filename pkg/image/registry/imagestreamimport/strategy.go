package imagestreamimport

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

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

func (s *strategy) PrepareForCreate(obj runtime.Object) {
	newIST := obj.(*api.ImageStreamImport)
	newIST.Status = api.ImageStreamImportStatus{}
}

func (s *strategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	isi := obj.(*api.ImageStreamImport)
	return validation.ValidateImageStreamImport(isi)
}
