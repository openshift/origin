package v1

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/util/sets"

	internal "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func addDefaultingFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(obj *MasterConfig) {
			if len(obj.APILevels) == 0 {
				obj.APILevels = internal.DefaultOpenShiftAPILevels
			}
			if len(obj.Controllers) == 0 {
				obj.Controllers = ControllersAll
			}
			if obj.ServingInfo.RequestTimeoutSeconds == 0 {
				obj.ServingInfo.RequestTimeoutSeconds = 60 * 60
			}
			if obj.ServingInfo.MaxRequestsInFlight == 0 {
				obj.ServingInfo.MaxRequestsInFlight = 500
			}
			if len(obj.PolicyConfig.OpenShiftInfrastructureNamespace) == 0 {
				obj.PolicyConfig.OpenShiftInfrastructureNamespace = bootstrappolicy.DefaultOpenShiftInfraNamespace
			}
			if len(obj.RoutingConfig.Subdomain) == 0 {
				obj.RoutingConfig.Subdomain = "router.default.svc.cluster.local"
			}

			// Populate the new NetworkConfig.ServiceNetworkCIDR field from the KubernetesMasterConfig.ServicesSubnet field if needed
			if len(obj.NetworkConfig.ServiceNetworkCIDR) == 0 {
				if obj.KubernetesMasterConfig != nil && len(obj.KubernetesMasterConfig.ServicesSubnet) > 0 {
					// if a subnet is set in the kubernetes master config, use that
					obj.NetworkConfig.ServiceNetworkCIDR = obj.KubernetesMasterConfig.ServicesSubnet
				} else {
					// default ServiceClusterIPRange used by kubernetes if nothing is specified
					obj.NetworkConfig.ServiceNetworkCIDR = "10.0.0.0/24"
				}
			}

			// Historically, the clientCA was incorrectly used as the master's server cert CA bundle
			// If missing from the config, migrate the ClientCA into that field
			if obj.OAuthConfig != nil && obj.OAuthConfig.MasterCA == nil {
				s := obj.ServingInfo.ClientCA
				// The final value of OAuthConfig.MasterCA should never be nil
				obj.OAuthConfig.MasterCA = &s
			}
		},
		func(obj *KubernetesMasterConfig) {
			if obj.MasterCount == 0 {
				obj.MasterCount = 1
			}
			if len(obj.APILevels) == 0 {
				obj.APILevels = internal.DefaultKubernetesAPILevels
			}
			if len(obj.ServicesNodePortRange) == 0 {
				obj.ServicesNodePortRange = "30000-32767"
			}
			if len(obj.PodEvictionTimeout) == 0 {
				obj.PodEvictionTimeout = "5m"
			}
		},
		func(obj *NodeConfig) {
			// Defaults/migrations for NetworkConfig
			if len(obj.NetworkConfig.NetworkPluginName) == 0 {
				obj.NetworkConfig.NetworkPluginName = obj.DeprecatedNetworkPluginName
			}
			if obj.NetworkConfig.MTU == 0 {
				obj.NetworkConfig.MTU = 1450
			}
			if len(obj.IPTablesSyncPeriod) == 0 {
				obj.IPTablesSyncPeriod = "5s"
			}

			// Auth cache defaults
			if len(obj.AuthConfig.AuthenticationCacheTTL) == 0 {
				obj.AuthConfig.AuthenticationCacheTTL = "5m"
			}
			if obj.AuthConfig.AuthenticationCacheSize == 0 {
				obj.AuthConfig.AuthenticationCacheSize = 1000
			}
			if len(obj.AuthConfig.AuthorizationCacheTTL) == 0 {
				obj.AuthConfig.AuthorizationCacheTTL = "5m"
			}
			if obj.AuthConfig.AuthorizationCacheSize == 0 {
				obj.AuthConfig.AuthorizationCacheSize = 1000
			}
		},
		func(obj *EtcdStorageConfig) {
			if len(obj.KubernetesStorageVersion) == 0 {
				obj.KubernetesStorageVersion = "v1"
			}
			if len(obj.KubernetesStoragePrefix) == 0 {
				obj.KubernetesStoragePrefix = "kubernetes.io"
			}
			if len(obj.OpenShiftStorageVersion) == 0 {
				obj.OpenShiftStorageVersion = internal.DefaultOpenShiftStorageVersionLevel
			}
			if len(obj.OpenShiftStoragePrefix) == 0 {
				obj.OpenShiftStoragePrefix = "openshift.io"
			}
		},
		func(obj *DockerConfig) {
			if len(obj.ExecHandlerName) == 0 {
				obj.ExecHandlerName = DockerExecHandlerNative
			}
		},
		func(obj *ServingInfo) {
			if len(obj.BindNetwork) == 0 {
				obj.BindNetwork = "tcp4"
			}
		},
		func(obj *ImagePolicyConfig) {
			if obj.MaxImagesBulkImportedPerRepository == 0 {
				obj.MaxImagesBulkImportedPerRepository = 5
			}
			if obj.MaxScheduledImageImportsPerMinute == 0 {
				obj.MaxScheduledImageImportsPerMinute = 60
			}
			if obj.ScheduledImageImportMinimumIntervalSeconds == 0 {
				obj.ScheduledImageImportMinimumIntervalSeconds = 15 * 60
			}
		},
		func(obj *DNSConfig) {
			if len(obj.BindNetwork) == 0 {
				obj.BindNetwork = "tcp4"
			}
		},
		func(obj *SecurityAllocator) {
			if len(obj.UIDAllocatorRange) == 0 {
				obj.UIDAllocatorRange = "1000000000-1999999999/10000"
			}
			if len(obj.MCSAllocatorRange) == 0 {
				obj.MCSAllocatorRange = "s0:/2"
			}
			if obj.MCSLabelsPerProject == 0 {
				obj.MCSLabelsPerProject = 5
			}
		},
		func(obj *IdentityProvider) {
			if len(obj.MappingMethod) == 0 {
				// By default, only let one identity provider authenticate a particular user
				// If multiple identity providers collide, the second one in will fail to auth
				// The admin can set this to "add" if they want to allow new identities to join existing users
				obj.MappingMethod = "claim"
			}
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		func(in *NodeConfig, out *internal.NodeConfig, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *internal.NodeConfig, out *NodeConfig, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *KubernetesMasterConfig, out *internal.KubernetesMasterConfig, s conversion.Scope) error {
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
		},
		func(in *internal.KubernetesMasterConfig, out *KubernetesMasterConfig, s conversion.Scope) error {
			// internal doesn't have all fields: APILevels
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *ServingInfo, out *internal.ServingInfo, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}
			out.ServerCert.CertFile = in.CertFile
			out.ServerCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *internal.ServingInfo, out *ServingInfo, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}
			out.CertFile = in.ServerCert.CertFile
			out.KeyFile = in.ServerCert.KeyFile
			return nil
		},
		func(in *RemoteConnectionInfo, out *internal.RemoteConnectionInfo, s conversion.Scope) error {
			out.URL = in.URL
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *internal.RemoteConnectionInfo, out *RemoteConnectionInfo, s conversion.Scope) error {
			out.URL = in.URL
			out.CA = in.CA
			out.CertFile = in.ClientCert.CertFile
			out.KeyFile = in.ClientCert.KeyFile
			return nil
		},
		func(in *EtcdConnectionInfo, out *internal.EtcdConnectionInfo, s conversion.Scope) error {
			out.URLs = in.URLs
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *internal.EtcdConnectionInfo, out *EtcdConnectionInfo, s conversion.Scope) error {
			out.URLs = in.URLs
			out.CA = in.CA
			out.CertFile = in.ClientCert.CertFile
			out.KeyFile = in.ClientCert.KeyFile
			return nil
		},
		func(in *KubeletConnectionInfo, out *internal.KubeletConnectionInfo, s conversion.Scope) error {
			out.Port = in.Port
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *internal.KubeletConnectionInfo, out *KubeletConnectionInfo, s conversion.Scope) error {
			out.Port = in.Port
			out.CA = in.CA
			out.CertFile = in.ClientCert.CertFile
			out.KeyFile = in.ClientCert.KeyFile
			return nil
		},
		func(in *internal.IdentityProvider, out *IdentityProvider, s conversion.Scope) error {
			if err := convert_runtime_Object_To_runtime_RawExtension(in.Provider, &out.Provider, s); err != nil {
				return err
			}
			out.Name = in.Name
			out.UseAsChallenger = in.UseAsChallenger
			out.UseAsLogin = in.UseAsLogin
			out.MappingMethod = in.MappingMethod
			return nil
		},
		func(in *IdentityProvider, out *internal.IdentityProvider, s conversion.Scope) error {
			if err := convert_runtime_RawExtension_To_runtime_Object(&in.Provider, out.Provider, s); err != nil {
				return err
			}
			if in.Provider.Object != nil {
				var err error
				out.Provider, err = internal.Scheme.ConvertToVersion(in.Provider.Object, internal.SchemeGroupVersion.String())
				if err != nil {
					return err
				}
			}
			out.Name = in.Name
			out.UseAsChallenger = in.UseAsChallenger
			out.UseAsLogin = in.UseAsLogin
			out.MappingMethod = in.MappingMethod
			return nil
		},
		func(in *internal.AdmissionPluginConfig, out *AdmissionPluginConfig, s conversion.Scope) error {
			if err := convert_runtime_Object_To_runtime_RawExtension(in.Configuration, &out.Configuration, s); err != nil {
				return err
			}
			out.Location = in.Location
			return nil
		},
		func(in *AdmissionPluginConfig, out *internal.AdmissionPluginConfig, s conversion.Scope) error {
			if err := convert_runtime_RawExtension_To_runtime_Object(&in.Configuration, out.Configuration, s); err != nil {
				return err
			}
			if in.Configuration.Object != nil {
				var err error
				out.Configuration, err = internal.Scheme.ConvertToVersion(in.Configuration.Object, internal.SchemeGroupVersion.String())
				if err != nil {
					return err
				}
			}
			out.Location = in.Location
			return nil
		},
		func(in *MasterVolumeConfig, out *internal.MasterVolumeConfig, s conversion.Scope) error {
			out.DynamicProvisioningEnabled = (in.DynamicProvisioningEnabled == nil) || (*in.DynamicProvisioningEnabled)
			return nil
		},
		func(in *internal.MasterVolumeConfig, out *MasterVolumeConfig, s conversion.Scope) error {
			enabled := in.DynamicProvisioningEnabled
			out.DynamicProvisioningEnabled = &enabled
			return nil
		},
		api.Convert_resource_Quantity_To_resource_Quantity,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

var codec = serializer.NewCodecFactory(internal.Scheme).LegacyCodec(SchemeGroupVersion)

// Convert_runtime_Object_To_runtime_RawExtension is conversion function that assumes that the runtime.Object you've embedded is in
// the same GroupVersion that your containing type is in.  This is signficantly better than simply breaking.
// Given an ordered list of preferred external versions for a given encode or conversion call, the behavior of this function could be
// made generic, predictable, and controllable.
func convert_runtime_Object_To_runtime_RawExtension(in runtime.Object, out *runtime.RawExtension, s conversion.Scope) error {
	if in == nil {
		return nil
	}

	externalObject, err := internal.Scheme.ConvertToVersion(in, s.Meta().DestVersion)
	if runtime.IsNotRegisteredError(err) {
		switch cast := in.(type) {
		case *runtime.Unknown:
			out.RawJSON = cast.RawJSON
			return nil
		case *runtime.Unstructured:
			bytes, err := runtime.Encode(runtime.UnstructuredJSONScheme, externalObject)
			if err != nil {
				return err
			}
			out.RawJSON = bytes
			return nil
		}
	}
	if err != nil {
		return err
	}

	bytes, err := runtime.Encode(codec, externalObject)
	if err != nil {
		return err
	}

	out.RawJSON = bytes
	out.Object = externalObject

	return nil
}

// Convert_runtime_RawExtension_To_runtime_Object well, this is the reason why there was runtime.Embedded.  The `out` here is hopeless.
// The caller doesn't know the type ahead of time and that means this method can't communicate the return value.  This sucks really badly.
// I'm going to set the `in.Object` field can have callers to this function do magic to pull it back out.  I'm also going to bitch about it.
func convert_runtime_RawExtension_To_runtime_Object(in *runtime.RawExtension, out runtime.Object, s conversion.Scope) error {
	if in == nil || len(in.RawJSON) == 0 || in.Object != nil {
		return nil
	}

	decodedObject, err := runtime.Decode(codec, in.RawJSON)
	if err != nil {
		in.Object = &runtime.Unknown{RawJSON: in.RawJSON}
		return nil
	}

	internalObject, err := internal.Scheme.ConvertToVersion(decodedObject, s.Meta().DestVersion)
	if err != nil {
		return err
	}

	in.Object = internalObject

	return nil
}
