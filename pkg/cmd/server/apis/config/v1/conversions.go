package v1

import (
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	internal "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func Convert_v1_AuditConfig_To_config_AuditConfig(in *legacyconfigv1.AuditConfig, out *internal.AuditConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if len(in.AuditFilePath) > 0 {
		out.InternalAuditFilePath = in.AuditFilePath
	}
	return nil
}

func Convert_config_AuditConfig_To_v1_AuditConfig(in *internal.AuditConfig, out *legacyconfigv1.AuditConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}
func Convert_v1_EtcdConnectionInfo_To_config_EtcdConnectionInfo(in *legacyconfigv1.EtcdConnectionInfo, out *internal.EtcdConnectionInfo, s conversion.Scope) error {
	out.URLs = in.URLs
	out.CA = in.CA
	out.ClientCert.CertFile = in.CertFile
	out.ClientCert.KeyFile = in.KeyFile
	return nil
}

func Convert_config_EtcdConnectionInfo_To_v1_EtcdConnectionInfo(in *internal.EtcdConnectionInfo, out *legacyconfigv1.EtcdConnectionInfo, s conversion.Scope) error {
	out.URLs = in.URLs
	out.CA = in.CA
	out.CertFile = in.ClientCert.CertFile
	out.KeyFile = in.ClientCert.KeyFile
	return nil
}

func Convert_v1_KubeletConnectionInfo_To_config_KubeletConnectionInfo(in *legacyconfigv1.KubeletConnectionInfo, out *internal.KubeletConnectionInfo, s conversion.Scope) error {
	out.Port = in.Port
	out.CA = in.CA
	out.ClientCert.CertFile = in.CertFile
	out.ClientCert.KeyFile = in.KeyFile
	return nil
}

func Convert_config_KubeletConnectionInfo_To_v1_KubeletConnectionInfo(in *internal.KubeletConnectionInfo, out *legacyconfigv1.KubeletConnectionInfo, s conversion.Scope) error {
	out.Port = in.Port
	out.CA = in.CA
	out.CertFile = in.ClientCert.CertFile
	out.KeyFile = in.ClientCert.KeyFile
	return nil
}

func Convert_v1_KubernetesMasterConfig_To_config_KubernetesMasterConfig(in *legacyconfigv1.KubernetesMasterConfig, out *internal.KubernetesMasterConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	if out.DisabledAPIGroupVersions == nil {
		out.DisabledAPIGroupVersions = map[string][]string{}
	}

	// the APILevels (whitelist) needs to be converted into an internal blacklist
	if len(in.APILevels) == 0 {
		out.DisabledAPIGroupVersions[internal.APIGroupKube] = []string{"*"}

	} else {
		availableLevels := internal.KubeAPIGroupsToAllowedVersions[internal.APIGroupKube]
		whitelistedLevels := sets.NewString(in.APILevels...)
		blacklistedLevels := []string{}

		for _, curr := range availableLevels {
			if !whitelistedLevels.Has(curr) {
				blacklistedLevels = append(blacklistedLevels, curr)
			}
		}

		if len(blacklistedLevels) > 0 {
			out.DisabledAPIGroupVersions[internal.APIGroupKube] = blacklistedLevels
		}
	}

	return nil
}

func Convert_config_KubernetesMasterConfig_To_v1_KubernetesMasterConfig(in *internal.KubernetesMasterConfig, out *legacyconfigv1.KubernetesMasterConfig, s conversion.Scope) error {
	// internal doesn't have all fields: APILevels
	return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
}

func Convert_v1_NodeConfig_To_config_NodeConfig(in *legacyconfigv1.NodeConfig, out *internal.NodeConfig, s conversion.Scope) error {
	return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
}

func Convert_config_NodeConfig_To_v1_NodeConfig(in *internal.NodeConfig, out *legacyconfigv1.NodeConfig, s conversion.Scope) error {
	return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
}

func Convert_v1_RemoteConnectionInfo_To_config_RemoteConnectionInfo(in *legacyconfigv1.RemoteConnectionInfo, out *internal.RemoteConnectionInfo, s conversion.Scope) error {
	out.URL = in.URL
	out.CA = in.CA
	out.ClientCert.CertFile = in.CertFile
	out.ClientCert.KeyFile = in.KeyFile
	return nil
}

func Convert_config_RemoteConnectionInfo_To_v1_RemoteConnectionInfo(in *internal.RemoteConnectionInfo, out *legacyconfigv1.RemoteConnectionInfo, s conversion.Scope) error {
	out.URL = in.URL
	out.CA = in.CA
	out.CertFile = in.ClientCert.CertFile
	out.KeyFile = in.ClientCert.KeyFile
	return nil
}

func Convert_v1_ServingInfo_To_config_ServingInfo(in *legacyconfigv1.ServingInfo, out *internal.ServingInfo, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	out.ServerCert.CertFile = in.CertFile
	out.ServerCert.KeyFile = in.KeyFile
	return nil
}

func Convert_config_ServingInfo_To_v1_ServingInfo(in *internal.ServingInfo, out *legacyconfigv1.ServingInfo, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	out.CertFile = in.ServerCert.CertFile
	out.KeyFile = in.ServerCert.KeyFile
	return nil
}

func Convert_v1_MasterVolumeConfig_To_config_MasterVolumeConfig(in *legacyconfigv1.MasterVolumeConfig, out *internal.MasterVolumeConfig, s conversion.Scope) error {
	out.DynamicProvisioningEnabled = (in.DynamicProvisioningEnabled == nil) || (*in.DynamicProvisioningEnabled)
	return nil
}

func Convert_config_MasterVolumeConfig_To_v1_MasterVolumeConfig(in *internal.MasterVolumeConfig, out *legacyconfigv1.MasterVolumeConfig, s conversion.Scope) error {
	enabled := in.DynamicProvisioningEnabled
	out.DynamicProvisioningEnabled = &enabled
	return nil
}

func Convert_v1_MasterNetworkConfig_To_config_MasterNetworkConfig(in *legacyconfigv1.MasterNetworkConfig, out *internal.MasterNetworkConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	if len(out.ClusterNetworks) == 0 {
		out.ClusterNetworks = []internal.ClusterNetworkEntry{
			{
				CIDR:             in.DeprecatedClusterNetworkCIDR,
				HostSubnetLength: in.DeprecatedHostSubnetLength,
			},
		}
	}

	if out.VXLANPort == 0 {
		out.VXLANPort = 4789
	}
	return nil
}

func Convert_v1_AdmissionPluginConfig_To_config_AdmissionPluginConfig(in *legacyconfigv1.AdmissionPluginConfig, out *internal.AdmissionPluginConfig, s conversion.Scope) error {
	if err := autoConvert_v1_AdmissionPluginConfig_To_config_AdmissionPluginConfig(in, out, s); err != nil {
		return err
	}

	if len(in.Configuration.Raw) == 0 && (in.Configuration.Object == nil) {
		out.Configuration = nil
	} else {
		if err := convert_runtime_RawExtension_To_runtime_Object(&in.Configuration, &out.Configuration, s); err != nil {
			return nil
		}
	}

	return nil
}

func Convert_config_AdmissionPluginConfig_To_v1_AdmissionPluginConfig(in *internal.AdmissionPluginConfig, out *legacyconfigv1.AdmissionPluginConfig, s conversion.Scope) error {
	if err := autoConvert_config_AdmissionPluginConfig_To_v1_AdmissionPluginConfig(in, out, s); err != nil {
		return err
	}

	if in.Configuration == nil {
		out.Configuration.Object = nil
		out.Configuration.Raw = nil
	} else {
		if err := convert_runtime_Object_To_runtime_RawExtension(&in.Configuration, &out.Configuration, s); err != nil {
			return nil
		}
	}

	return nil
}

// Convert_v1_IdentityProvider_To_config_IdentityProvider is an autogenerated conversion function.
func Convert_v1_IdentityProvider_To_config_IdentityProvider(in *legacyconfigv1.IdentityProvider, out *internal.IdentityProvider, s conversion.Scope) error {
	if err := autoConvert_v1_IdentityProvider_To_config_IdentityProvider(in, out, s); err != nil {
		return err
	}

	if len(in.Provider.Raw) == 0 && (in.Provider.Object == nil) {
		out.Provider = nil
	} else {
		if err := convert_runtime_RawExtension_To_runtime_Object(&in.Provider, &out.Provider, s); err != nil {
			return nil
		}
	}

	return nil
}

func Convert_config_IdentityProvider_To_v1_IdentityProvider(in *internal.IdentityProvider, out *legacyconfigv1.IdentityProvider, s conversion.Scope) error {
	if err := autoConvert_config_IdentityProvider_To_v1_IdentityProvider(in, out, s); err != nil {
		return err
	}

	if in.Provider == nil {
		out.Provider.Object = nil
		out.Provider.Raw = nil
	} else {
		if err := convert_runtime_Object_To_runtime_RawExtension(&in.Provider, &out.Provider, s); err != nil {
			return nil
		}
	}

	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddConversionFuncs(
	//convert_runtime_Object_To_runtime_RawExtension,
	//convert_runtime_RawExtension_To_runtime_Object,
	//Convert_v1_AuditConfig_To_config_AuditConfig,
	//Convert_config_AuditConfig_To_v1_AuditConfig,
	//Convert_v1_EtcdConnectionInfo_To_config_EtcdConnectionInfo,
	//Convert_config_EtcdConnectionInfo_To_v1_EtcdConnectionInfo,
	//Convert_v1_KubeletConnectionInfo_To_config_KubeletConnectionInfo,
	//Convert_v1_KubernetesMasterConfig_To_config_KubernetesMasterConfig,
	//Convert_config_KubernetesMasterConfig_To_v1_KubernetesMasterConfig,
	//Convert_v1_NodeConfig_To_config_NodeConfig,
	//Convert_config_NodeConfig_To_v1_NodeConfig,
	//Convert_v1_RemoteConnectionInfo_To_config_RemoteConnectionInfo,
	//Convert_config_RemoteConnectionInfo_To_v1_RemoteConnectionInfo,
	//Convert_v1_ServingInfo_To_config_ServingInfo,
	//Convert_config_ServingInfo_To_v1_ServingInfo,
	//Convert_v1_MasterVolumeConfig_To_config_MasterVolumeConfig,
	//Convert_config_MasterVolumeConfig_To_v1_MasterVolumeConfig,
	//Convert_v1_MasterNetworkConfig_To_config_MasterNetworkConfig,
	//metav1.Convert_resource_Quantity_To_resource_Quantity,
	//metav1.Convert_bool_To_Pointer_bool,
	//metav1.Convert_Pointer_bool_To_bool,
	)
}

// convert_runtime_Object_To_runtime_RawExtension attempts to convert runtime.Objects to the appropriate target.
func convert_runtime_Object_To_runtime_RawExtension(in *runtime.Object, out *runtime.RawExtension, s conversion.Scope) error {
	return apihelpers.Convert_runtime_Object_To_runtime_RawExtension(internal.Scheme, in, out, s)
}

// convert_runtime_RawExtension_To_runtime_Object attempts to convert an incoming object into the
// appropriate output type.
func convert_runtime_RawExtension_To_runtime_Object(in *runtime.RawExtension, out *runtime.Object, s conversion.Scope) error {
	return apihelpers.Convert_runtime_RawExtension_To_runtime_Object(internal.Scheme, in, out, s)
}
