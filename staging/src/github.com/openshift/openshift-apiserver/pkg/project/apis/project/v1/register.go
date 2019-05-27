package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	"github.com/openshift/api/project/v1"
	"github.com/openshift/origin/pkg/project/apis/project"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		project.Install,
		v1.Install,
		corev1conversions.AddToScheme,

		addFieldSelectorKeyConversions,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
