package configconversion

import (
	"k8s.io/apimachinery/pkg/conversion"

	"reflect"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
)

func Convert_legacyconfigv1_JenkinsPipelineConfig_to_kubecontrolplanev1_JenkinsPipelineConfig(in *legacyconfigv1.JenkinsPipelineConfig, out *openshiftcontrolplanev1.JenkinsPipelineConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_RegistryLocation_to_kubecontrolplanev1_RegistryLocation(in *legacyconfigv1.RegistryLocation, out *openshiftcontrolplanev1.RegistryLocation, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}
