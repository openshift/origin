package imagepolicy

import (
	"k8s.io/apiserver/pkg/admission"

	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy/imagereferencemutators"
)

func NewInitializer(imageMutators imagereferencemutators.ImageMutators, defaultRegistryFn func() (string, bool)) admission.PluginInitializer {
	return &localInitializer{
		imageMutators:     imageMutators,
		defaultRegistryFn: defaultRegistryFn,
	}
}

type WantsImageMutators interface {
	SetImageMutators(imagereferencemutators.ImageMutators)
	admission.InitializationValidator
}

type WantsDefaultRegistryFunc interface {
	SetDefaultRegistryFunc(func() (string, bool))
	admission.InitializationValidator
}

type localInitializer struct {
	imageMutators     imagereferencemutators.ImageMutators
	defaultRegistryFn func() (string, bool)
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *localInitializer) Initialize(plugin admission.Interface) {
	if wants, ok := plugin.(WantsImageMutators); ok {
		wants.SetImageMutators(i.imageMutators)
	}
	if wants, ok := plugin.(WantsDefaultRegistryFunc); ok {
		wants.SetDefaultRegistryFunc(i.defaultRegistryFn)
	}
}
