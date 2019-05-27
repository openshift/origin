package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	"github.com/openshift/api/image/v1"
	"github.com/openshift/origin/pkg/image/apis/image"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		image.Install,
		v1.Install,
		corev1conversions.AddToScheme,
		docker10.AddToScheme,
		dockerpre012.AddToScheme,

		addFieldSelectorKeyConversions,
		AddConversionFuncs,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
