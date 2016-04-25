package kubernetes

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	newetcdclient "github.com/coreos/etcd/client"

	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apiserver"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/genericapiserver"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/registry/cachesize"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/intstr"
	knet "k8s.io/kubernetes/pkg/util/net"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	saadmit "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
)

// AdmissionPlugins is the full list of admission control plugins to enable in the order they must run
var AdmissionPlugins = []string{"RunOnceDuration", lifecycle.PluginName, "PodNodeConstraints", "OriginPodNodeEnvironment", overrideapi.PluginName, serviceadmit.ExternalIPPluginName, "LimitRanger", "ServiceAccount", "SecurityContextConstraint", "BuildDefaults", "BuildOverrides", "ResourceQuota", "SCCExecRestrictions"}

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	Options    configapi.KubernetesMasterConfig
	KubeClient *kclient.Client

	Master            *master.Config
	ControllerManager *cmapp.CMServer
	CloudProvider     cloudprovider.Interface
}

func BuildKubernetesMasterConfig(options configapi.MasterConfig, requestContextMapper kapi.RequestContextMapper, kubeClient *kclient.Client, pluginInitializer oadmission.PluginInitializer) (*MasterConfig, error) {
	if options.KubernetesMasterConfig == nil {
		return nil, errors.New("insufficient information to build KubernetesMasterConfig")
	}

	// Connect and setup etcd interfaces
	etcdClient, err := etcd.MakeNewEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)
	kubeletClient, err := kubeletclient.NewStaticKubeletClient(kubeletClientConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to configure Kubelet client: %v", err)
	}

	// in-order list of plug-ins that should intercept admission decisions
	// TODO: Push node environment support to upstream in future

	_, portString, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

	portRange, err := knet.ParsePortRange(options.KubernetesMasterConfig.ServicesNodePortRange)
	if err != nil {
		return nil, err
	}

	podEvictionTimeout, err := time.ParseDuration(options.KubernetesMasterConfig.PodEvictionTimeout)
	if err != nil {
		return nil, fmt.Errorf("unable to parse PodEvictionTimeout: %v", err)
	}

	// Defaults are tested in TestAPIServerDefaults
	server := apiserveroptions.NewAPIServer()
	// Adjust defaults
	server.EventTTL = 2 * time.Hour
	server.ServiceClusterIPRange = net.IPNet(flagtypes.DefaultIPNet(options.KubernetesMasterConfig.ServicesSubnet))
	server.ServiceNodePortRange = *portRange
	server.AdmissionControl = strings.Join(AdmissionPlugins, ",")
	server.EnableLogsSupport = false // don't expose server logs
	server.EnableProfiling = false
	server.APIPrefix = KubeAPIPrefix
	server.APIGroupPrefix = KubeAPIGroupPrefix
	server.SecurePort = port
	server.MasterCount = options.KubernetesMasterConfig.MasterCount

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.APIServerArguments, server.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	if len(options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride) > 0 {
		server.AdmissionControl = strings.Join(options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride, ",")
	}

	// Defaults are tested in TestCMServerDefaults
	cmserver := cmapp.NewCMServer()
	// Adjust defaults
	cmserver.Address = "" // no healthz endpoint
	cmserver.Port = 0     // no healthz endpoint
	cmserver.PodEvictionTimeout = unversioned.Duration{Duration: podEvictionTimeout}

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.ControllerArguments, cmserver.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	cloud, err := cloudprovider.InitCloudProvider(cmserver.CloudProvider, cmserver.CloudConfigFile)
	if err != nil {
		return nil, err
	}
	if cloud != nil {
		glog.V(2).Infof("Successfully initialized cloud provider: %q from the config file: %q\n", server.CloudProvider, server.CloudConfigFile)
	}

	plugins := []admission.Interface{}
	for _, pluginName := range strings.Split(server.AdmissionControl, ",") {
		switch pluginName {
		case lifecycle.PluginName:
			// We need to include our infrastructure and shared resource namespaces in the immortal namespaces list
			immortalNamespaces := sets.NewString(kapi.NamespaceDefault)
			if len(options.PolicyConfig.OpenShiftSharedResourcesNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftSharedResourcesNamespace)
			}
			if len(options.PolicyConfig.OpenShiftInfrastructureNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftInfrastructureNamespace)
			}
			plugins = append(plugins, lifecycle.NewLifecycle(clientadapter.FromUnversionedClient(kubeClient), immortalNamespaces))

		case serviceadmit.ExternalIPPluginName:
			// this needs to be moved upstream to be part of core config
			reject, admit, err := serviceadmit.ParseCIDRRules(options.NetworkConfig.ExternalIPNetworkCIDRs)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			plugins = append(plugins, serviceadmit.NewExternalIPRanger(reject, admit))
		case saadmit.PluginName:
			// we need to set some custom parameters on the service account admission controller, so create that one by hand
			saAdmitter := saadmit.NewServiceAccount(clientadapter.FromUnversionedClient(kubeClient))
			saAdmitter.LimitSecretReferences = options.ServiceAccountConfig.LimitSecretReferences
			saAdmitter.Run()
			plugins = append(plugins, saAdmitter)

		default:
			configFile, err := pluginconfig.GetPluginConfigFile(options.KubernetesMasterConfig.AdmissionConfig.PluginConfig, pluginName, server.AdmissionControlConfigFile)
			if err != nil {
				return nil, err
			}
			plugin := admission.InitPlugin(pluginName, clientadapter.FromUnversionedClient(kubeClient), configFile)
			if plugin != nil {
				plugins = append(plugins, plugin)
			}

		}
	}
	pluginInitializer.Initialize(plugins)
	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}
	admissionController := admission.NewChainHandler(plugins...)

	var proxyClientCerts []tls.Certificate
	if len(options.KubernetesMasterConfig.ProxyClientInfo.CertFile) > 0 {
		clientCert, err := tls.LoadX509KeyPair(
			options.KubernetesMasterConfig.ProxyClientInfo.CertFile,
			options.KubernetesMasterConfig.ProxyClientInfo.KeyFile,
		)
		if err != nil {
			return nil, err
		}
		proxyClientCerts = append(proxyClientCerts, clientCert)
	}

	// TODO you have to know every APIGroup you're enabling or upstream will panic.  Its alternative to panicing is Fataling
	// It needs a refactor to return errors
	storageDestinations := genericapiserver.NewStorageDestinations()
	// storageVersions is a map from API group to allowed versions that must be a version exposed by the REST API or it breaks.
	// We need to fix the upstream to stop using the storage version as a preferred api version.
	storageVersions := map[string]string{}

	enabledKubeVersions := configapi.GetEnabledAPIVersionsForGroup(*options.KubernetesMasterConfig, configapi.APIGroupKube)
	if len(enabledKubeVersions) > 0 {
		kubeStorageVersion := unversioned.GroupVersion{Group: configapi.APIGroupKube, Version: options.EtcdStorageConfig.KubernetesStorageVersion}
		databaseStorage, err := NewEtcdStorage(etcdClient, kubeStorageVersion, options.EtcdStorageConfig.KubernetesStoragePrefix)
		if err != nil {
			return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
		}
		storageDestinations.AddAPIGroup(configapi.APIGroupKube, databaseStorage)
		storageVersions[configapi.APIGroupKube] = options.EtcdStorageConfig.KubernetesStorageVersion
	}

	// enable this if extensions API is enabled (or batch or autoscaling, since they persist to extensions/v1beta1 for now)
	// TODO: replace this with a loop over configured storage versions
	extensionsEnabled := len(configapi.GetEnabledAPIVersionsForGroup(*options.KubernetesMasterConfig, configapi.APIGroupExtensions)) > 0
	batchEnabled := len(configapi.GetEnabledAPIVersionsForGroup(*options.KubernetesMasterConfig, configapi.APIGroupBatch)) > 0
	autoscalingEnabled := len(configapi.GetEnabledAPIVersionsForGroup(*options.KubernetesMasterConfig, configapi.APIGroupAutoscaling)) > 0
	if extensionsEnabled || autoscalingEnabled || batchEnabled {
		// TODO: replace this with a configured storage version for extensions once configuration exposes this
		extensionsStorageVersion := unversioned.GroupVersion{Group: extensions.GroupName, Version: "v1beta1"}
		databaseStorage, err := NewEtcdStorage(etcdClient, extensionsStorageVersion, options.EtcdStorageConfig.KubernetesStoragePrefix)
		if err != nil {
			return nil, fmt.Errorf("Error setting up Kubernetes extensions server storage: %v", err)
		}
		storageDestinations.AddAPIGroup(configapi.APIGroupExtensions, databaseStorage)
		storageVersions[configapi.APIGroupExtensions] = extensionsStorageVersion.String()
	}

	// Preserve previous behavior of using the first non-loopback address
	// TODO: Deprecate this behavior and just require a valid value to be passed in
	publicAddress := net.ParseIP(options.KubernetesMasterConfig.MasterIP)
	if publicAddress == nil || publicAddress.IsUnspecified() || publicAddress.IsLoopback() {
		hostIP, err := knet.ChooseHostInterface()
		if err != nil {
			glog.Fatalf("Unable to find suitable network address.error='%v'. Set the masterIP directly to avoid this error.", err)
		}
		publicAddress = hostIP
		glog.Infof("Will report %v as public IP address.", publicAddress)
	}

	m := &master.Config{
		Config: &genericapiserver.Config{

			PublicAddress: publicAddress,
			ReadWritePort: port,

			Authorizer:       apiserver.NewAlwaysAllowAuthorizer(),
			AdmissionControl: admissionController,

			StorageDestinations: storageDestinations,
			StorageVersions:     storageVersions,

			ServiceClusterIPRange: (*net.IPNet)(&server.ServiceClusterIPRange),
			ServiceNodePortRange:  server.ServiceNodePortRange,

			RequestContextMapper: requestContextMapper,

			APIResourceConfigSource: getAPIResourceConfig(options),
			APIPrefix:               server.APIPrefix,
			APIGroupPrefix:          server.APIGroupPrefix,

			MasterCount: server.MasterCount,

			// Set the TLS options for proxying to pods and services
			// Proxying to nodes uses the kubeletClient TLS config (so can provide a different cert, and verify the node hostname)
			ProxyTLSClientConfig: &tls.Config{
				// Proxying to pods and services cannot verify hostnames, since they are contacted on randomly allocated IPs
				InsecureSkipVerify: true,
				Certificates:       proxyClientCerts,
			},

			Serializer: kapi.Codecs,

			EnableLogsSupport:         server.EnableLogsSupport,
			EnableProfiling:           server.EnableProfiling,
			EnableWatchCache:          server.EnableWatchCache,
			MasterServiceNamespace:    server.MasterServiceNamespace,
			ExternalHost:              server.ExternalHost,
			MinRequestTimeout:         server.MinRequestTimeout,
			KubernetesServiceNodePort: server.KubernetesServiceNodePort,
		},

		EventTTL: server.EventTTL,

		KubeletClient: kubeletClient,

		EnableCoreControllers: true,

		DeleteCollectionWorkers: server.DeleteCollectionWorkers,
	}

	if server.EnableWatchCache {
		cachesize.SetWatchCacheSizes(server.WatchCacheSizes)
	}

	if options.DNSConfig != nil {
		_, dnsPortStr, err := net.SplitHostPort(options.DNSConfig.BindAddress)
		if err != nil {
			return nil, fmt.Errorf("unable to parse DNS bind address %s: %v", options.DNSConfig.BindAddress, err)
		}
		dnsPort, err := strconv.Atoi(dnsPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DNS port: %v", err)
		}
		m.ExtraServicePorts = append(m.ExtraServicePorts,
			kapi.ServicePort{Name: "dns", Port: 53, Protocol: kapi.ProtocolUDP, TargetPort: intstr.FromInt(dnsPort)},
			kapi.ServicePort{Name: "dns-tcp", Port: 53, Protocol: kapi.ProtocolTCP, TargetPort: intstr.FromInt(dnsPort)},
		)
		m.ExtraEndpointPorts = append(m.ExtraEndpointPorts,
			kapi.EndpointPort{Name: "dns", Port: dnsPort, Protocol: kapi.ProtocolUDP},
			kapi.EndpointPort{Name: "dns-tcp", Port: dnsPort, Protocol: kapi.ProtocolTCP},
		)
	}

	kmaster := &MasterConfig{
		Options:    *options.KubernetesMasterConfig,
		KubeClient: kubeClient,

		Master:            m,
		ControllerManager: cmserver,
		CloudProvider:     cloud,
	}

	return kmaster, nil
}

// getAPIResourceConfig builds the config for enabling resources
func getAPIResourceConfig(options configapi.MasterConfig) genericapiserver.APIResourceConfigSource {
	resourceConfig := genericapiserver.NewResourceConfig()

	for group := range configapi.KnownKubeAPIGroups {
		for _, version := range configapi.GetEnabledAPIVersionsForGroup(*options.KubernetesMasterConfig, group) {
			gv := unversioned.GroupVersion{Group: group, Version: version}
			resourceConfig.EnableVersions(gv)
		}
	}

	for group := range options.KubernetesMasterConfig.DisabledAPIGroupVersions {
		for _, version := range configapi.GetDisabledAPIVersionsForGroup(*options.KubernetesMasterConfig, group) {
			gv := unversioned.GroupVersion{Group: group, Version: version}
			resourceConfig.DisableVersions(gv)
		}
	}

	return resourceConfig
}

// NewEtcdStorage returns a storage interface for the provided storage version.
func NewEtcdStorage(client newetcdclient.Client, version unversioned.GroupVersion, prefix string) (helper storage.Interface, err error) {
	return etcdstorage.NewEtcdStorage(client, kapi.Codecs.LegacyCodec(version), prefix, false), nil
}
