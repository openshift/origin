package imagestreamimport

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

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

func (s *strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	newIST := obj.(*api.ImageStreamImport)
	newIST.Status = api.ImageStreamImportStatus{}
}

func (s *strategy) PrepareImageForCreate(obj runtime.Object) {
	image := obj.(*api.Image)

	// signatures can be added using "images" or "imagesignatures" resources
	image.Signatures = nil

	// Remove the raw manifest as it's very big and this leads to a large memory consumption in etcd.
	image.DockerImageManifest = ""
	image.DockerImageConfig = ""
}

func (s *strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	isi := obj.(*api.ImageStreamImport)
	return validation.ValidateImageStreamImport(isi)
}
