package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/network/v1"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		v1.Install,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
