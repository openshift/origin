package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/network/v1"
	"github.com/openshift/origin/pkg/network/apis/network"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		network.Install,
		v1.Install,
		RegisterDefaults,
	)
	Install = localSchemeBuilder.AddToScheme
)
