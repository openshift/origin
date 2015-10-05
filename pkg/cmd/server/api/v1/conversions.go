package v1

import (
	"k8s.io/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func init() {
	err := newer.Scheme.AddDefaultingFuncs(
		func(obj *MasterConfig) {
			if len(obj.APILevels) == 0 {
				obj.APILevels = newer.DefaultOpenShiftAPILevels
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
		},
		func(obj *KubernetesMasterConfig) {
			if obj.MasterCount == 0 {
				obj.MasterCount = 1
			}
			if len(obj.APILevels) == 0 {
				obj.APILevels = newer.DefaultKubernetesAPILevels
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
		},
		func(obj *EtcdStorageConfig) {
			if len(obj.KubernetesStorageVersion) == 0 {
				obj.KubernetesStorageVersion = "v1"
			}
			if len(obj.KubernetesStoragePrefix) == 0 {
				obj.KubernetesStoragePrefix = "kubernetes.io"
			}
			if len(obj.OpenShiftStorageVersion) == 0 {
				obj.OpenShiftStorageVersion = "v1"
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
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = newer.Scheme.AddConversionFuncs(
		func(in *NodeConfig, out *newer.NodeConfig, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *newer.NodeConfig, out *NodeConfig, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *ServingInfo, out *newer.ServingInfo, s conversion.Scope) error {
			out.BindAddress = in.BindAddress
			out.BindNetwork = in.BindNetwork
			out.ClientCA = in.ClientCA
			out.ServerCert.CertFile = in.CertFile
			out.ServerCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *newer.ServingInfo, out *ServingInfo, s conversion.Scope) error {
			out.BindAddress = in.BindAddress
			out.BindNetwork = in.BindNetwork
			out.ClientCA = in.ClientCA
			out.CertFile = in.ServerCert.CertFile
			out.KeyFile = in.ServerCert.KeyFile
			return nil
		},
		func(in *RemoteConnectionInfo, out *newer.RemoteConnectionInfo, s conversion.Scope) error {
			out.URL = in.URL
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *newer.RemoteConnectionInfo, out *RemoteConnectionInfo, s conversion.Scope) error {
			out.URL = in.URL
			out.CA = in.CA
			out.CertFile = in.ClientCert.CertFile
			out.KeyFile = in.ClientCert.KeyFile
			return nil
		},
		func(in *EtcdConnectionInfo, out *newer.EtcdConnectionInfo, s conversion.Scope) error {
			out.URLs = in.URLs
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *newer.EtcdConnectionInfo, out *EtcdConnectionInfo, s conversion.Scope) error {
			out.URLs = in.URLs
			out.CA = in.CA
			out.CertFile = in.ClientCert.CertFile
			out.KeyFile = in.ClientCert.KeyFile
			return nil
		},
		func(in *KubeletConnectionInfo, out *newer.KubeletConnectionInfo, s conversion.Scope) error {
			out.Port = in.Port
			out.CA = in.CA
			out.ClientCert.CertFile = in.CertFile
			out.ClientCert.KeyFile = in.KeyFile
			return nil
		},
		func(in *newer.KubeletConnectionInfo, out *KubeletConnectionInfo, s conversion.Scope) error {
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
