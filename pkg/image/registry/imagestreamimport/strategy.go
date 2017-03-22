package imagestreamimport

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	serverapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// strategy implements behavior for ImageStreamImports.
type strategy struct {
	runtime.ObjectTyper
	allowedRegistries *serverapi.AllowedRegistries
	registryFn        api.DefaultRegistryFunc
}

func NewStrategy(registries *serverapi.AllowedRegistries, registryFn api.DefaultRegistryFunc) *strategy {
	return &strategy{
		ObjectTyper:       kapi.Scheme,
		allowedRegistries: registries,
		registryFn:        registryFn,
	}
}

func (s *strategy) NamespaceScoped() bool {
	return true
}

func (s *strategy) GenerateName(string) string {
	return ""
}

func (s *strategy) Canonicalize(runtime.Object) {
}

func (s *strategy) ValidateAllowedRegistries(isi *api.ImageStreamImport) field.ErrorList {
	errs := field.ErrorList{}
	if s.allowedRegistries == nil {
		return errs
	}
	allowedRegistries := *s.allowedRegistries
	// FIXME: The registryFn won't return the registry location until the registry service
	// is created. This should be switched to use registry DNS instead of lazy-loading.
	if localRegistry, ok := s.registryFn(); ok {
		allowedRegistries = append([]configapi.RegistryLocation{{DomainName: localRegistry}}, allowedRegistries...)
	}
	validate := func(path *field.Path, name string, insecure bool) field.ErrorList {
		ref, _ := api.ParseDockerImageReference(name)
		registryHost, registryPort := ref.RegistryHostPort(insecure)
		return validation.ValidateRegistryAllowedForImport(path.Child("from", "name"), ref.Name, registryHost, registryPort, &allowedRegistries)
	}
	if spec := isi.Spec.Repository; spec != nil && spec.From.Kind == "DockerImage" {
		errs = append(errs, validate(field.NewPath("spec").Child("repository"), spec.From.Name, spec.ImportPolicy.Insecure)...)
	}
	if len(isi.Spec.Images) > 0 {
		for i, image := range isi.Spec.Images {
			errs = append(errs, validate(field.NewPath("spec").Child("images").Index(i), image.From.Name, image.ImportPolicy.Insecure)...)
		}
	}
	return errs
}

func (s *strategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
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

func (s *strategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	isi := obj.(*api.ImageStreamImport)
	return validation.ValidateImageStreamImport(isi)
}
