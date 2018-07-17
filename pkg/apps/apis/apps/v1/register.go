package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"
	extensionsv1beta1conversions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"

	"github.com/openshift/api/apps/v1"
	"github.com/openshift/origin/pkg/apps/apis/apps"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		apps.Install,
		v1.Install,
		corev1conversions.AddToScheme,
		extensionsv1beta1conversions.AddToScheme,
		AddConversionFuncs,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
