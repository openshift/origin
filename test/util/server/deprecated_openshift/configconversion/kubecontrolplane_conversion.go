package configconversion

import (
	"k8s.io/apimachinery/pkg/conversion"

	"reflect"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
)

func Convert_legacyconfigv1_MasterAuthConfig_to_kubecontrolplanev1_MasterAuthConfig(in *legacyconfigv1.MasterAuthConfig, out *kubecontrolplanev1.MasterAuthConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_AggregatorConfig_to_kubecontrolplanev1_AggregatorConfig(in *legacyconfigv1.AggregatorConfig, out *kubecontrolplanev1.AggregatorConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_KubeletConnectionInfo_to_kubecontrolplanev1_KubeletConnectionInfo(in *legacyconfigv1.KubeletConnectionInfo, out *kubecontrolplanev1.KubeletConnectionInfo, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_UserAgentMatchingConfig_to_kubecontrolplanev1_UserAgentMatchingConfig(in *legacyconfigv1.UserAgentMatchingConfig, out *kubecontrolplanev1.UserAgentMatchingConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}
