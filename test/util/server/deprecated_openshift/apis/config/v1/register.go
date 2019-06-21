package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coreinternalconversions "k8s.io/kubernetes/pkg/apis/core"

	buildv1 "github.com/openshift/api/build/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	buildinternalconversions "github.com/openshift/openshift-apiserver/pkg/build/apis/build/v1"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

var (
	// Legacy is the 'v1' apiVersion of config
	LegacyGroupName          = ""
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}
	legacySchemeBuilder      = runtime.NewSchemeBuilder(
		legacyconfigv1.InstallLegacy,
		config.InstallLegacy,
		coreinternalconversions.AddToScheme,
		buildinternalconversions.Install,

		RegisterConversions,
		addConversionFuncs,
		addDefaultingFuncs,
	)
	InstallLegacy = legacySchemeBuilder.AddToScheme

	externalLegacySchemeBuilder = runtime.NewSchemeBuilder(
		legacyconfigv1.InstallLegacy,
		buildv1.Install,
	)
	InstallLegacyExternal = externalLegacySchemeBuilder.AddToScheme

	// this only exists to make the generator happy.
	localSchemeBuilder = runtime.NewSchemeBuilder()
)
