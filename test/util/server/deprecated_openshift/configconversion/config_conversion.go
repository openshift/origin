package configconversion

import (
	"k8s.io/apimachinery/pkg/conversion"

	"reflect"

	configv1 "github.com/openshift/api/config/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
)

func Convert_legacyconfigv1_HTTPServingInfo_to_configv1_HTTPServingInfo(in *legacyconfigv1.HTTPServingInfo, out *configv1.HTTPServingInfo, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_AuditConfig_to_configv1_AuditConfig(in *legacyconfigv1.AuditConfig, out *configv1.AuditConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_EtcdConnectionInfo_to_configv1_EtcdConnectionInfo(in *legacyconfigv1.EtcdConnectionInfo, out *configv1.EtcdConnectionInfo, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_AdmissionPluginConfig_to_configv1_AdmissionPluginConfig(in *legacyconfigv1.AdmissionPluginConfig, out *configv1.AdmissionPluginConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_legacyconfigv1_MasterClients_to_configv1_KubeClientConfig(in *legacyconfigv1.MasterClients, out *configv1.KubeClientConfig, s conversion.Scope) error {
	out.KubeConfig = in.OpenShiftLoopbackKubeConfig
	if in.OpenShiftLoopbackClientConnectionOverrides == nil {
		return nil
	}

	return Convert_legacyconfigv1_ClientConnectionOverrides_to_configv1_ClientConnectionOverrides(in.OpenShiftLoopbackClientConnectionOverrides, &out.ConnectionOverrides, s)
}

func Convert_legacyconfigv1_ClientConnectionOverrides_to_configv1_ClientConnectionOverrides(in *legacyconfigv1.ClientConnectionOverrides, out *configv1.ClientConnectionOverrides, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}
