package master

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/registry/generic"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/options/encryptionconfig"
	apiserverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage"
	storagefactory "k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/client-go/rest"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	kapiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	batchv1beta1 "k8s.io/kubernetes/pkg/apis/batch/v1beta1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/networking"
	"k8s.io/kubernetes/pkg/apis/policy"
	storageapi "k8s.io/kubernetes/pkg/apis/storage"
	storageapiv1beta1 "k8s.io/kubernetes/pkg/apis/storage/v1beta1"
	"k8s.io/kubernetes/pkg/kubeapiserver"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/registry/cachesize"
	"k8s.io/kubernetes/pkg/registry/core/endpoint"
	endpointsstorage "k8s.io/kubernetes/pkg/registry/core/endpoint/storage"
	kversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/api/security"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/election"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
)

var LegacyAPIGroupPrefixes = sets.NewString(apiserver.DefaultLegacyAPIPrefix, legacy.RESTPrefix)

// TODO I'm honestly not sure this is worth it. We're not likely to ever be able to launch from flags, so this just
// adds a layer of complexity that is driving me crazy.
// BuildKubeAPIserverOptions constructs the appropriate kube-apiserver run options.
// It returns an error if no KubernetesMasterConfig was defined.
func BuildKubeAPIserverOptions(masterConfig configapi.MasterConfig) (*kapiserveroptions.ServerRunOptions, error) {
	portRange, err := knet.ParsePortRange(masterConfig.KubernetesMasterConfig.ServicesNodePortRange)
	if err != nil {
		return nil, err
	}

	// Defaults are tested in TestAPIServerDefaults
	server := kapiserveroptions.NewServerRunOptions()

	// Adjust defaults
	server.EnableLogsHandler = false
	server.EventTTL = 2 * time.Hour
	server.ServiceClusterIPRange = net.IPNet(flagtypes.DefaultIPNet(masterConfig.KubernetesMasterConfig.ServicesSubnet))
	server.ServiceNodePortRange = *portRange
	server.Features.EnableProfiling = true

	server.SecureServing, err = configprocessing.ToServingOptions(masterConfig.ServingInfo)
	if err != nil {
		return nil, err
	}
	server.InsecureServing.BindPort = 0

	// disable anonymous authentication
	// NOTE: this is only to get rid of the "AnonymousAuth is not allowed with the AllowAll authorizer"
	// warning. We do not use the authenticator or authorizer created by this.
	server.Authentication.Anonymous.Allow = false
	server.Authentication.ClientCert = &apiserveroptions.ClientCertAuthenticationOptions{ClientCA: masterConfig.ServingInfo.ClientCA}
	if masterConfig.AuthConfig.RequestHeader == nil {
		server.Authentication.RequestHeader = &genericoptions.RequestHeaderAuthenticationOptions{}
	} else {
		server.Authentication.RequestHeader = &genericoptions.RequestHeaderAuthenticationOptions{
			ClientCAFile:        masterConfig.AuthConfig.RequestHeader.ClientCA,
			UsernameHeaders:     masterConfig.AuthConfig.RequestHeader.UsernameHeaders,
			GroupHeaders:        masterConfig.AuthConfig.RequestHeader.GroupHeaders,
			ExtraHeaderPrefixes: masterConfig.AuthConfig.RequestHeader.ExtraHeaderPrefixes,
			AllowedNames:        masterConfig.AuthConfig.RequestHeader.ClientCommonNames,
		}
	}

	server.Etcd, err = configprocessing.GetEtcdOptions(masterConfig.KubernetesMasterConfig.APIServerArguments, masterConfig.EtcdClientInfo, masterConfig.EtcdStorageConfig.KubernetesStoragePrefix, nil)
	if err != nil {
		return nil, err
	}

	server.GenericServerRunOptions.CorsAllowedOriginList = masterConfig.CORSAllowedOrigins
	server.GenericServerRunOptions.MaxRequestsInFlight = masterConfig.ServingInfo.MaxRequestsInFlight
	server.GenericServerRunOptions.MaxMutatingRequestsInFlight = masterConfig.ServingInfo.MaxRequestsInFlight / 2
	server.GenericServerRunOptions.MinRequestTimeout = masterConfig.ServingInfo.RequestTimeoutSeconds

	server.KubeletConfig.ReadOnlyPort = 0
	server.KubeletConfig.Port = masterConfig.KubeletClientInfo.Port
	server.KubeletConfig.PreferredAddressTypes = []string{"Hostname", "InternalIP", "ExternalIP"}
	server.KubeletConfig.EnableHttps = true
	server.KubeletConfig.CAFile = masterConfig.KubeletClientInfo.CA
	server.KubeletConfig.CertFile = masterConfig.KubeletClientInfo.ClientCert.CertFile
	server.KubeletConfig.KeyFile = masterConfig.KubeletClientInfo.ClientCert.KeyFile

	server.ProxyClientCertFile = masterConfig.AggregatorConfig.ProxyClientInfo.CertFile
	server.ProxyClientKeyFile = masterConfig.AggregatorConfig.ProxyClientInfo.KeyFile

	// resolve extended arguments
	args := map[string][]string{}
	for k, v := range masterConfig.KubernetesMasterConfig.APIServerArguments {
		args[k] = v
	}
	// fixup 'apis/' prefixed args
	for i, key := range args["runtime-config"] {
		args["runtime-config"][i] = strings.TrimPrefix(key, "apis/")
	}
	if masterConfig.AuditConfig.Enabled {
		if existing, ok := args["feature-gates"]; ok {
			args["feature-gates"] = []string{existing[0] + ",AdvancedAuditing=true"}
		} else {
			args["feature-gates"] = []string{"AdvancedAuditing=true"}
		}
	}
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(args, server.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	return server, nil
}

// BuildStorageFactory builds a storage factory based on server.Etcd.StorageConfig with overrides from masterConfig.
// This storage factory is used for kubernetes and origin registries. Compare pkg/util/restoptions/configgetter.go.
func BuildStorageFactory(server *kapiserveroptions.ServerRunOptions, enforcedStorageVersions map[schema.GroupResource]schema.GroupVersion) (*apiserverstorage.DefaultStorageFactory, error) {
	resourceEncodingConfig := apiserverstorage.NewDefaultResourceEncodingConfig(legacyscheme.Scheme)

	storageGroupsToEncodingVersion, err := server.StorageSerialization.StorageGroupsToEncodingVersion()
	if err != nil {
		return nil, err
	}
	for group, storageEncodingVersion := range storageGroupsToEncodingVersion {
		resourceEncodingConfig.SetVersionEncoding(group, storageEncodingVersion, schema.GroupVersion{Group: group, Version: runtime.APIVersionInternal})
	}
	resourceEncodingConfig.SetResourceEncoding(batch.Resource("cronjobs"), batchv1beta1.SchemeGroupVersion, batch.SchemeGroupVersion)
	resourceEncodingConfig.SetResourceEncoding(apiregistration.Resource("apiservices"), apiregistrationv1beta1.SchemeGroupVersion, apiregistration.SchemeGroupVersion)
	resourceEncodingConfig.SetResourceEncoding(storageapi.Resource("volumeattachments"), storageapiv1beta1.SchemeGroupVersion, storageapi.SchemeGroupVersion)

	for gr, storageGV := range enforcedStorageVersions {
		resourceEncodingConfig.SetResourceEncoding(gr, storageGV, schema.GroupVersion{Group: storageGV.Group, Version: runtime.APIVersionInternal})
	}

	storageFactory := apiserverstorage.NewDefaultStorageFactory(
		server.Etcd.StorageConfig,
		server.Etcd.DefaultStorageMediaType,
		legacyscheme.Codecs,
		resourceEncodingConfig,
		master.DefaultAPIResourceConfigSource(),
		kubeapiserver.SpecialDefaultResourcePrefixes,
	)
	if err != nil {
		return nil, err
	}

	// the order here is important, it defines which version will be used for storage
	// keep HPAs in the autoscaling apigroup (as in upstream 1.6), but keep extension cohabitation around until origin 3.7.
	storageFactory.AddCohabitatingResources(autoscaling.Resource("horizontalpodautoscalers"), extensions.Resource("horizontalpodautoscalers"))
	storageFactory.AddCohabitatingResources(apps.Resource("deployments"), extensions.Resource("deployments"))
	storageFactory.AddCohabitatingResources(apps.Resource("daemonsets"), extensions.Resource("daemonsets"))
	storageFactory.AddCohabitatingResources(apps.Resource("replicasets"), extensions.Resource("replicasets"))
	storageFactory.AddCohabitatingResources(networking.Resource("networkpolicies"), extensions.Resource("networkpolicies"))
	storageFactory.AddCohabitatingResources(security.Resource("securitycontextconstraints"), kapi.Resource("securitycontextconstraints"))
	// TODO: switch to prefer policy API group in 3.11
	storageFactory.AddCohabitatingResources(extensions.Resource("podsecuritypolicies"), policy.Resource("podsecuritypolicies"))

	if server.Etcd.EncryptionProviderConfigFilepath != "" {
		glog.V(4).Infof("Reading encryption configuration from %q", server.Etcd.EncryptionProviderConfigFilepath)
		transformerOverrides, err := encryptionconfig.GetTransformerOverrides(server.Etcd.EncryptionProviderConfigFilepath)
		if err != nil {
			return nil, err
		}
		for groupResource, transformer := range transformerOverrides {
			storageFactory.SetTransformer(groupResource, transformer)
		}
	}

	return storageFactory, nil
}

// buildUpstreamGenericConfig copies the apiserver.Config setup code from k8s.io/kubernetes/cmd/kube-apiserver/app/server.go.
// ONLY COMMENT OUT CODE HERE, do not modify it. Do modifications outside of this function.
func buildUpstreamGenericConfig(s *kapiserveroptions.ServerRunOptions) (*apiserver.Config, error) {
	// set defaults
	if err := s.GenericServerRunOptions.DefaultAdvertiseAddress(s.SecureServing.SecureServingOptions); err != nil {
		return nil, err
	}
	// In origin: certs should be available:
	//_, apiServerServiceIP, err := master.DefaultServiceIPRange(s.ServiceClusterIPRange)
	//if err != nil {
	//	return nil, fmt.Errorf("error determining service IP ranges: %v", err)
	//}
	//if err := s.SecureServing.MaybeDefaultWithSelfSignedCerts(s.GenericServerRunOptions.AdvertiseAddress.String(), apiServerServiceIP); err != nil {
	//	return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	//}

	s.Authentication.ApplyAuthorization(s.Authorization)

	// validate options
	if errs := s.Validate(); len(errs) != 0 {
		return nil, kerrors.NewAggregate(errs)
	}

	// create config from options
	genericConfig := apiserver.NewConfig(legacyscheme.Codecs)

	if err := s.GenericServerRunOptions.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	if err := s.SecureServing.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	if err := s.Audit.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	if err := s.Features.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	if err := s.Authentication.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	if err := s.APIEnablement.ApplyTo(genericConfig, master.DefaultAPIResourceConfigSource(), legacyscheme.Scheme); err != nil {
		return nil, err
	}
	// Do not wait for etcd because the internal etcd is launched after this and origin has an etcd test already
	// if err := utilwait.PollImmediate(etcdRetryInterval, etcdRetryLimit*etcdRetryInterval, preflight.EtcdConnection{ServerList: s.Etcd.StorageConfig.ServerList}.CheckEtcdServers); err != nil {
	// 	return nil, fmt.Errorf("error waiting for etcd connection: %v", err)
	// }

	// Use protobufs for self-communication.
	// Since not every generic apiserver has to support protobufs, we
	// cannot default to it in generic apiserver and need to explicitly
	// set it in kube-apiserver.
	genericConfig.LoopbackClientConfig.ContentConfig.ContentType = "application/vnd.kubernetes.protobuf"

	return genericConfig, nil
}

// buildUpstreamClientCARegistrationHook copies the ClientCARegistrationHook code from k8s.io/kubernetes/cmd/kube-apiserver/app/server.go.
// ONLY COMMENT OUT CODE HERE, do not modify it. Do modifications outside of this function.
func buildUpstreamClientCARegistrationHook(s *kapiserveroptions.ServerRunOptions) (master.ClientCARegistrationHook, error) {
	clientCA, err := readCAorNil(s.Authentication.ClientCert.ClientCA)
	if err != nil {
		return master.ClientCARegistrationHook{}, err
	}
	requestHeaderProxyCA, err := readCAorNil(s.Authentication.RequestHeader.ClientCAFile)
	if err != nil {
		return master.ClientCARegistrationHook{}, err
	}
	return master.ClientCARegistrationHook{
		ClientCA:                         clientCA,
		RequestHeaderUsernameHeaders:     s.Authentication.RequestHeader.UsernameHeaders,
		RequestHeaderGroupHeaders:        s.Authentication.RequestHeader.GroupHeaders,
		RequestHeaderExtraHeaderPrefixes: s.Authentication.RequestHeader.ExtraHeaderPrefixes,
		RequestHeaderCA:                  requestHeaderProxyCA,
		RequestHeaderAllowedNames:        s.Authentication.RequestHeader.AllowedNames,
	}, nil
}

func buildProxyClientCerts(masterConfig configapi.MasterConfig) ([]tls.Certificate, error) {
	var proxyClientCerts []tls.Certificate
	if len(masterConfig.KubernetesMasterConfig.ProxyClientInfo.CertFile) > 0 {
		clientCert, err := tls.LoadX509KeyPair(
			masterConfig.KubernetesMasterConfig.ProxyClientInfo.CertFile,
			masterConfig.KubernetesMasterConfig.ProxyClientInfo.KeyFile,
		)
		if err != nil {
			return nil, err
		}
		proxyClientCerts = append(proxyClientCerts, clientCert)
	}

	return proxyClientCerts, nil
}

func buildPublicAddress(masterConfig configapi.MasterConfig) (net.IP, error) {
	// Preserve previous behavior of using the first non-loopback address
	// TODO: Deprecate this behavior and just require a valid value to be passed in
	publicAddress := net.ParseIP(masterConfig.KubernetesMasterConfig.MasterIP)
	if publicAddress == nil || publicAddress.IsUnspecified() || publicAddress.IsLoopback() {
		hostIP, err := knet.ChooseHostInterface()
		if err != nil {
			return net.IP{}, fmt.Errorf("unable to find suitable network address.error='%v'. Set the masterIP directly to avoid this error.", err)
		}
		publicAddress = hostIP
		glog.Infof("Will report %v as public IP address.", publicAddress)
	}

	return publicAddress, nil
}

type incompleteKubeMasterConfig struct {
	options          *kapiserveroptions.ServerRunOptions
	incompleteConfig *apiserver.Config
	masterConfig     configapi.MasterConfig
}

func BuildKubernetesMasterConfig(masterConfig configapi.MasterConfig) (*incompleteKubeMasterConfig, error) {
	apiserverOptions, err := BuildKubeAPIserverOptions(masterConfig)
	if err != nil {
		return nil, err
	}

	genericConfig, err := buildUpstreamGenericConfig(apiserverOptions)
	if err != nil {
		return nil, err
	}

	return &incompleteKubeMasterConfig{apiserverOptions, genericConfig, masterConfig}, nil
}

func (rc *incompleteKubeMasterConfig) LoopbackConfig() *rest.Config {
	return rc.incompleteConfig.LoopbackClientConfig
}

func (rc *incompleteKubeMasterConfig) Complete(
	admissionControl admission.Interface,
	originAuthenticator authenticator.Request,
	kubeAuthorizer authorizer.Authorizer,
) (*master.Config, error) {
	genericConfig, apiserverOptions, masterConfig := rc.incompleteConfig, rc.options, rc.masterConfig

	proxyClientCerts, err := buildProxyClientCerts(masterConfig)
	if err != nil {
		return nil, err
	}

	storageFactory, err := BuildStorageFactory(apiserverOptions, map[schema.GroupResource]schema.GroupVersion{
		// SCC are actually an openshift resource we injected into the kubeapiserver pre-3.0.  We need to manage
		// their storage configuration via the kube storagefactory.
		// TODO We really should create a single one of these somewhere.
		{Group: "", Resource: "securitycontextconstraints"}: {Group: "security.openshift.io", Version: "v1"},
	})
	if err != nil {
		return nil, err
	}

	publicAddress, err := buildPublicAddress(masterConfig)
	if err != nil {
		return nil, err
	}

	clientCARegistrationHook, err := buildUpstreamClientCARegistrationHook(apiserverOptions)
	if err != nil {
		return nil, err
	}

	_, oauthMetadata, _ := oauthutil.PrepOauthMetadata(masterConfig.OAuthConfig, masterConfig.AuthConfig.OAuthMetadataFile)

	// override config values
	kubeVersion := kversion.Get()
	genericConfig.Version = &kubeVersion
	genericConfig.PublicAddress = publicAddress
	genericConfig.Authentication.Authenticator = originAuthenticator // this is used to fulfill the tokenreviews endpoint which is used by node authentication
	genericConfig.Authorization.Authorizer = kubeAuthorizer          // this is used to fulfill the kube SAR endpoints
	genericConfig.AdmissionControl = admissionControl
	genericConfig.RequestInfoResolver = configprocessing.OpenshiftRequestInfoResolver()
	genericConfig.OpenAPIConfig = configprocessing.DefaultOpenAPIConfig(oauthMetadata)
	genericConfig.SwaggerConfig = apiserver.DefaultSwaggerConfig()
	genericConfig.SwaggerConfig.PostBuildHandler = customizeSwaggerDefinition
	genericConfig.LegacyAPIGroupPrefixes = configprocessing.LegacyAPIGroupPrefixes
	genericConfig.SecureServing.MinTLSVersion = crypto.TLSVersionOrDie(masterConfig.ServingInfo.MinTLSVersion)
	genericConfig.SecureServing.CipherSuites = crypto.CipherSuitesOrDie(masterConfig.ServingInfo.CipherSuites)
	oAuthClientCertCAs, err := configapi.GetOAuthClientCertCAs(masterConfig)
	if err != nil {
		glog.Fatalf("Error setting up OAuth2 client certificates: %v", err)
	}
	for _, cert := range oAuthClientCertCAs {
		genericConfig.SecureServing.ClientCA.AddCert(cert)
	}

	url, err := url.Parse(masterConfig.MasterPublicURL)
	if err != nil {
		glog.Fatalf("Error parsing master public url %q: %v", masterConfig.MasterPublicURL, err)
	}
	genericConfig.ExternalAddress = url.Host

	genericConfig.LongRunningFunc = configprocessing.IsLongRunningRequest

	if apiserverOptions.Etcd.EnableWatchCache {
		glog.V(2).Infof("Initializing cache sizes based on %dMB limit", apiserverOptions.GenericServerRunOptions.TargetRAMMB)
		sizes := cachesize.NewHeuristicWatchCacheSizes(apiserverOptions.GenericServerRunOptions.TargetRAMMB)
		if userSpecified, err := genericoptions.ParseWatchCacheSizes(apiserverOptions.Etcd.WatchCacheSizes); err == nil {
			for resource, size := range userSpecified {
				sizes[resource] = size
			}
		}
		apiserverOptions.Etcd.WatchCacheSizes, err = genericoptions.WriteWatchCacheSizes(sizes)
		if err != nil {
			return nil, err
		}
	}

	if err := apiserverOptions.Etcd.ApplyWithStorageFactoryTo(storageFactory, genericConfig); err != nil {
		return nil, err
	}

	// we don't use legacy audit anymore
	genericConfig.LegacyAuditWriter = nil
	backend, policyChecker, err := configprocessing.GetAuditConfig(masterConfig.AuditConfig)
	if err != nil {
		return nil, err
	}
	genericConfig.AuditBackend = backend
	genericConfig.AuditPolicyChecker = policyChecker

	kubeApiserverConfig := &master.Config{
		GenericConfig: genericConfig,
		ExtraConfig: master.ExtraConfig{
			MasterCount: apiserverOptions.MasterCount,

			// Set the TLS options for proxying to pods and services
			// Proxying to nodes uses the kubeletClient TLS config (so can provide a different cert, and verify the node hostname)
			ProxyTransport: knet.SetTransportDefaults(&http.Transport{
				TLSClientConfig: &tls.Config{
					// Proxying to pods and services cannot verify hostnames, since they are contacted on randomly allocated IPs
					InsecureSkipVerify: true,
					Certificates:       proxyClientCerts,
				},
			}),

			ClientCARegistrationHook: clientCARegistrationHook,

			APIServerServicePort:      443,
			ServiceNodePortRange:      apiserverOptions.ServiceNodePortRange,
			KubernetesServiceNodePort: apiserverOptions.KubernetesServiceNodePort,
			ServiceIPRange:            apiserverOptions.ServiceClusterIPRange,

			StorageFactory:          storageFactory,
			APIResourceConfigSource: getAPIResourceConfig(masterConfig),

			EventTTL: apiserverOptions.EventTTL,

			KubeletClientConfig: apiserverOptions.KubeletConfig,

			EnableLogsSupport: false, // don't expose server logs
		},
	}

	ttl := masterConfig.KubernetesMasterConfig.MasterEndpointReconcileTTL
	interval := ttl * 2 / 3

	glog.V(2).Infof("Using the lease endpoint reconciler with TTL=%ds and interval=%ds", ttl, interval)

	config, err := kubeApiserverConfig.ExtraConfig.StorageFactory.NewConfig(kapi.Resource("apiServerIPInfo"))
	if err != nil {
		return nil, err
	}
	leaseStorage, _, err := storagefactory.Create(*config)
	if err != nil {
		return nil, err
	}
	masterLeases := newMasterLeases(leaseStorage, ttl)
	endpointConfig, err := kubeApiserverConfig.ExtraConfig.StorageFactory.NewConfig(kapi.Resource("endpoints"))
	if err != nil {
		return nil, err
	}
	endpointsStorage := endpointsstorage.NewREST(generic.RESTOptions{
		StorageConfig:           endpointConfig,
		Decorator:               generic.UndecoratedStorage,
		DeleteCollectionWorkers: 0,
		ResourcePrefix:          kubeApiserverConfig.ExtraConfig.StorageFactory.ResourcePrefix(kapi.Resource("endpoints")),
	})
	endpointRegistry := endpoint.NewRegistry(endpointsStorage)
	kubeApiserverConfig.ExtraConfig.EndpointReconcilerConfig = master.EndpointReconcilerConfig{
		Reconciler: election.NewLeaseEndpointReconciler(endpointRegistry, masterLeases),
		Interval:   time.Duration(interval) * time.Second,
	}

	if masterConfig.DNSConfig != nil {
		_, dnsPortStr, err := net.SplitHostPort(masterConfig.DNSConfig.BindAddress)
		if err != nil {
			return nil, fmt.Errorf("unable to parse DNS bind address %s: %v", masterConfig.DNSConfig.BindAddress, err)
		}
		dnsPort, err := strconv.Atoi(dnsPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DNS port: %v", err)
		}
		kubeApiserverConfig.ExtraConfig.ExtraServicePorts = append(kubeApiserverConfig.ExtraConfig.ExtraServicePorts,
			kapi.ServicePort{Name: "dns", Port: 53, Protocol: kapi.ProtocolUDP, TargetPort: intstr.FromInt(dnsPort)},
			kapi.ServicePort{Name: "dns-tcp", Port: 53, Protocol: kapi.ProtocolTCP, TargetPort: intstr.FromInt(dnsPort)},
		)
		kubeApiserverConfig.ExtraConfig.ExtraEndpointPorts = append(kubeApiserverConfig.ExtraConfig.ExtraEndpointPorts,
			kapi.EndpointPort{Name: "dns", Port: int32(dnsPort), Protocol: kapi.ProtocolUDP},
			kapi.EndpointPort{Name: "dns-tcp", Port: int32(dnsPort), Protocol: kapi.ProtocolTCP},
		)
	}

	// we do this for integration tests to be able to turn it off for better startup speed
	// TODO remove the entire option once openapi is faster
	if masterConfig.DisableOpenAPI {
		kubeApiserverConfig.GenericConfig.OpenAPIConfig = nil
	}

	return kubeApiserverConfig, nil
}

// getAPIResourceConfig builds the config for enabling resources
func getAPIResourceConfig(options configapi.MasterConfig) apiserverstorage.APIResourceConfigSource {
	resourceConfig := apiserverstorage.NewResourceConfig()

	for group := range configapi.KnownKubeAPIGroups {
		for _, version := range configapi.GetEnabledAPIVersionsForGroup(options.KubernetesMasterConfig, group) {
			gv := schema.GroupVersion{Group: group, Version: version}
			resourceConfig.EnableVersions(gv)
		}
	}

	for group := range options.KubernetesMasterConfig.DisabledAPIGroupVersions {
		for _, version := range configapi.GetDisabledAPIVersionsForGroup(options.KubernetesMasterConfig, group) {
			gv := schema.GroupVersion{Group: group, Version: version}
			resourceConfig.DisableVersions(gv)
		}
	}

	return resourceConfig
}

func readCAorNil(file string) ([]byte, error) {
	if len(file) == 0 {
		return nil, nil
	}
	return ioutil.ReadFile(file)
}

func newMasterLeases(storage storage.Interface, masterEndpointReconcileTTL int) election.Leases {
	return election.NewLeases(storage, "/masterleases/", uint64(masterEndpointReconcileTTL))
}
