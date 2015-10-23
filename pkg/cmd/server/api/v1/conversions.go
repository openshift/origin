package v1

import (
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/util/sets"

	internal "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func init() {
	err := internal.Scheme.AddDefaultingFuncs(
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
	err = internal.Scheme.AddConversionFuncs(
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
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
