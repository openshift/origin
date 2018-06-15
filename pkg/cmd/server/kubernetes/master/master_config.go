package master

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	restful "github.com/emicklei/go-restful"
	"github.com/go-openapi/spec"
	"github.com/golang/glog"
	"gopkg.in/natefinch/lumberjack.v2"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	"k8s.io/apiserver/pkg/audit"
	auditpolicy "k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apiserverendpointsopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	apiserver "k8s.io/apiserver/pkg/server"
	kgenericfilters "k8s.io/apiserver/pkg/server/filters"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/options/encryptionconfig"
	apiserverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage"
	storagefactory "k8s.io/apiserver/pkg/storage/storagebackend/factory"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	auditlog "k8s.io/apiserver/plugin/pkg/audit/log"
	auditwebhook "k8s.io/apiserver/plugin/pkg/audit/webhook"
	pluginwebhook "k8s.io/apiserver/plugin/pkg/audit/webhook"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	openapicommon "k8s.io/kube-openapi/pkg/common"
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
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"
	kversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/api"
	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/election"
	nodeclient "github.com/openshift/origin/pkg/cmd/server/kubernetes/node/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	openapigenerated "github.com/openshift/origin/pkg/openapi"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"github.com/openshift/origin/pkg/version"

	// TODO fix this install, it is required for TestPreferredGroupVersions to pass
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
)

// request paths that match this regular expression will be treated as long running
// and not subjected to the default server timeout.
const originLongRunningEndpointsRE = "(/|^)(buildconfigs/.*/instantiatebinary|imagestreamimports)$"

var LegacyAPIGroupPrefixes = sets.NewString(apiserver.DefaultLegacyAPIPrefix, api.Prefix)

// TODO I'm honestly not sure this is worth it. We're not likely to ever be able to launch from flags, so this just
// adds a layer of complexity that is driving me crazy.
// BuildKubeAPIserverOptions constructs the appropriate kube-apiserver run options.
// It returns an error if no KubernetesMasterConfig was defined.
func BuildKubeAPIserverOptions(masterConfig configapi.MasterConfig) (*kapiserveroptions.ServerRunOptions, error) {
	host, portString, err := net.SplitHostPort(masterConfig.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

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

	server.SecureServing.BindAddress = net.ParseIP(host)
	server.SecureServing.BindPort = port
	server.SecureServing.BindNetwork = masterConfig.ServingInfo.BindNetwork
	server.SecureServing.ServerCert.CertKey.CertFile = masterConfig.ServingInfo.ServerCert.CertFile
	server.SecureServing.ServerCert.CertKey.KeyFile = masterConfig.ServingInfo.ServerCert.KeyFile
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

	server.Etcd.EnableGarbageCollection = true
	server.Etcd.StorageConfig.Type = "etcd3"
	server.Etcd.DefaultStorageMediaType = "application/json" // TODO(post-1.6.1-rebase): enable protobuf with etcd3 as upstream
	server.Etcd.StorageConfig.Quorum = true
	server.Etcd.StorageConfig.Prefix = masterConfig.EtcdStorageConfig.KubernetesStoragePrefix
	server.Etcd.StorageConfig.ServerList = masterConfig.EtcdClientInfo.URLs
	server.Etcd.StorageConfig.KeyFile = masterConfig.EtcdClientInfo.ClientCert.KeyFile
	server.Etcd.StorageConfig.CertFile = masterConfig.EtcdClientInfo.ClientCert.CertFile
	server.Etcd.StorageConfig.CAFile = masterConfig.EtcdClientInfo.CA
	server.Etcd.DefaultWatchCacheSize = 0

	server.GenericServerRunOptions.CorsAllowedOriginList = masterConfig.CORSAllowedOrigins
	server.GenericServerRunOptions.MaxRequestsInFlight = masterConfig.ServingInfo.MaxRequestsInFlight
	server.GenericServerRunOptions.MaxMutatingRequestsInFlight = masterConfig.ServingInfo.MaxRequestsInFlight / 2
	server.GenericServerRunOptions.MinRequestTimeout = masterConfig.ServingInfo.RequestTimeoutSeconds
	for _, nc := range masterConfig.ServingInfo.NamedCertificates {
		sniCert := utilflag.NamedCertKey{
			CertFile: nc.CertFile,
			KeyFile:  nc.KeyFile,
			Names:    nc.Names,
		}
		server.SecureServing.SNICertKeys = append(server.SecureServing.SNICertKeys, sniCert)
	}

	server.KubeletConfig.ReadOnlyPort = 0

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
	resourceEncodingConfig := apiserverstorage.NewDefaultResourceEncodingConfig(legacyscheme.Registry)

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
	storageFactory.AddCohabitatingResources(securityapi.Resource("securitycontextconstraints"), kapi.Resource("securitycontextconstraints"))
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
	if err := s.APIEnablement.ApplyTo(genericConfig, master.DefaultAPIResourceConfigSource(), legacyscheme.Registry); err != nil {
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

func buildKubeApiserverConfig(
	masterConfig configapi.MasterConfig,
	admissionControl admission.Interface,
	originAuthenticator authenticator.Request,
	kubeAuthorizer authorizer.Authorizer,
) (*master.Config, error) {
	apiserverOptions, err := BuildKubeAPIserverOptions(masterConfig)
	if err != nil {
		return nil, err
	}

	genericConfig, err := buildUpstreamGenericConfig(apiserverOptions)
	if err != nil {
		return nil, err
	}

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

	// override config values
	kubeVersion := kversion.Get()
	genericConfig.Version = &kubeVersion
	genericConfig.PublicAddress = publicAddress
	genericConfig.Authentication.Authenticator = originAuthenticator // this is used to fulfill the tokenreviews endpoint which is used by node authentication
	genericConfig.Authorization.Authorizer = kubeAuthorizer          // this is used to fulfill the kube SAR endpoints
	genericConfig.DisabledPostStartHooks.Insert(rbacrest.PostStartHookName)
	// This disables the ThirdPartyController which removes handlers from our go-restful containers.  The remove functionality is broken and destroys the serve mux.
	genericConfig.DisabledPostStartHooks.Insert("extensions/third-party-resources")
	genericConfig.AdmissionControl = admissionControl
	genericConfig.RequestInfoResolver = openshiftRequestInfoResolver()
	genericConfig.OpenAPIConfig = defaultOpenAPIConfig(masterConfig)
	genericConfig.SwaggerConfig = apiserver.DefaultSwaggerConfig()
	genericConfig.SwaggerConfig.PostBuildHandler = customizeSwaggerDefinition
	genericConfig.LegacyAPIGroupPrefixes = LegacyAPIGroupPrefixes
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

	originLongRunningRequestRE := regexp.MustCompile(originLongRunningEndpointsRE)
	kubeLongRunningFunc := kgenericfilters.BasicLongRunningRequestCheck(
		sets.NewString("watch", "proxy"),
		sets.NewString("attach", "exec", "proxy", "log", "portforward"),
	)
	genericConfig.LongRunningFunc = func(r *http.Request, requestInfo *apirequest.RequestInfo) bool {
		return originLongRunningRequestRE.MatchString(r.URL.Path) || kubeLongRunningFunc(r, requestInfo)
	}

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
	backend, policyChecker, err := GetAuditConfig(masterConfig.AuditConfig)
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

			KubeletClientConfig: *nodeclient.GetKubeletClientConfig(masterConfig),

			EnableLogsSupport:     false, // don't expose server logs
			EnableCoreControllers: true,
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

	return kubeApiserverConfig, nil
}

// TODO this function's parameters need to be refactored
func BuildKubernetesMasterConfig(
	masterConfig configapi.MasterConfig,
	admissionControl admission.Interface,
	originAuthenticator authenticator.Request,
	kubeAuthorizer authorizer.Authorizer,
) (*master.Config, error) {
	apiserverConfig, err := buildKubeApiserverConfig(
		masterConfig,
		admissionControl,
		originAuthenticator,
		kubeAuthorizer,
	)
	if err != nil {
		return nil, err
	}

	// we do this for integration tests to be able to turn it off for better startup speed
	// TODO remove the entire option once openapi is faster
	if masterConfig.DisableOpenAPI {
		apiserverConfig.GenericConfig.OpenAPIConfig = nil
	}

	return apiserverConfig, nil
}

func defaultOpenAPIConfig(config configapi.MasterConfig) *openapicommon.Config {
	securityDefinitions := spec.SecurityDefinitions{}
	if len(config.ServiceAccountConfig.PublicKeyFiles) > 0 || len(config.AuthConfig.WebhookTokenAuthenticators) > 0 {
		securityDefinitions["BearerToken"] = &spec.SecurityScheme{
			SecuritySchemeProps: spec.SecuritySchemeProps{
				Type:        "apiKey",
				Name:        "authorization",
				In:          "header",
				Description: "Bearer Token authentication",
			},
		}
	}
	if _, oauthMetadata, _ := oauthutil.PrepOauthMetadata(config); oauthMetadata != nil {
		securityDefinitions["Oauth2Implicit"] = &spec.SecurityScheme{
			SecuritySchemeProps: spec.SecuritySchemeProps{
				Type:             "oauth2",
				Flow:             "implicit",
				AuthorizationURL: oauthMetadata.AuthorizationEndpoint,
				Scopes:           scope.DescribeScopes(oauthMetadata.ScopesSupported),
			},
		}
		securityDefinitions["Oauth2AccessToken"] = &spec.SecurityScheme{
			SecuritySchemeProps: spec.SecuritySchemeProps{
				Type:             "oauth2",
				Flow:             "accessCode",
				AuthorizationURL: oauthMetadata.AuthorizationEndpoint,
				TokenURL:         oauthMetadata.TokenEndpoint,
				Scopes:           scope.DescribeScopes(oauthMetadata.ScopesSupported),
			},
		}
	}
	defNamer := apiserverendpointsopenapi.NewDefinitionNamer(legacyscheme.Scheme)
	return &openapicommon.Config{
		ProtocolList:      []string{"https"},
		GetDefinitions:    openapigenerated.GetOpenAPIDefinitions,
		IgnorePrefixes:    []string{"/swaggerapi", "/healthz", "/controllers", "/metrics", "/version/openshift", "/brokers"},
		GetDefinitionName: defNamer.GetDefinitionName,
		GetOperationIDAndTags: func(r *restful.Route) (string, []string, error) {
			op := r.Operation
			path := r.Path
			// DEPRECATED: These endpoints are going to be removed in 1.8 or 1.9 release.
			if strings.HasPrefix(path, "/oapi/v1/namespaces/{namespace}/processedtemplates") {
				op = "createNamespacedProcessedTemplate"
			} else if strings.HasPrefix(path, "/apis/template.openshift.io/v1/namespaces/{namespace}/processedtemplates") {
				op = "createNamespacedProcessedTemplateV1"
			} else if strings.HasPrefix(path, "/oapi/v1/processedtemplates") {
				op = "createProcessedTemplateForAllNamespacesV1"
			} else if strings.HasPrefix(path, "/apis/template.openshift.io/v1/processedtemplates") {
				op = "createProcessedTemplateForAllNamespaces"
			}
			if op != r.Operation {
				return op, []string{}, nil
			}
			return apiserverendpointsopenapi.GetOperationIDAndTags(r)
		},
		Info: &spec.Info{
			InfoProps: spec.InfoProps{
				Title:   "OpenShift API (with Kubernetes)",
				Version: version.Get().String(),
				License: &spec.License{
					Name: "Apache 2.0 (ASL2.0)",
					URL:  "http://www.apache.org/licenses/LICENSE-2.0",
				},
				Description: heredoc.Doc(`
					OpenShift provides builds, application lifecycle, image content management,
					and administrative policy on top of Kubernetes. The API allows consistent
					management of those objects.

					All API operations are authenticated via an Authorization	bearer token that
					is provided for service accounts as a generated secret (in JWT form) or via
					the native OAuth endpoint located at /oauth/authorize. Core infrastructure
					components may use client certificates that require no authentication.

					All API operations return a 'resourceVersion' string that represents the
					version of the object in the underlying storage. The standard LIST operation
					performs a snapshot read of the underlying objects, returning a resourceVersion
					representing a consistent version of the listed objects. The WATCH operation
					allows all updates to a set of objects after the provided resourceVersion to
					be observed by a client. By listing and beginning a watch from the returned
					resourceVersion, clients may observe a consistent view of the state of one
					or more objects. Note that WATCH always returns the update after the provided
					resourceVersion. Watch may be extended a limited time in the past - using
					etcd 2 the watch window is 1000 events (which on a large cluster may only
					be a few tens of seconds) so clients must explicitly handle the "watch
					to old error" by re-listing.

					Objects are divided into two rough categories - those that have a lifecycle
					and must reflect the state of the cluster, and those that have no state.
					Objects with lifecycle typically have three main sections:

					* 'metadata' common to all objects
					* a 'spec' that represents the desired state
					* a 'status' that represents how much of the desired state is reflected on
					  the cluster at the current time

					Objects that have no state have 'metadata' but may lack a 'spec' or 'status'
					section.

					Objects are divided into those that are namespace scoped (only exist inside
					of a namespace) and those that are cluster scoped (exist outside of
					a namespace). A namespace scoped resource will be deleted when the namespace
					is deleted and cannot be created if the namespace has not yet been created
					or is in the process of deletion. Cluster scoped resources are typically
					only accessible to admins - resources like nodes, persistent volumes, and
					cluster policy.

					All objects have a schema that is a combination of the 'kind' and
					'apiVersion' fields. This schema is additive only for any given version -
					no backwards incompatible changes are allowed without incrementing the
					apiVersion. The server will return and accept a number of standard
					responses that share a common schema - for instance, the common
					error type is 'metav1.Status' (described below) and will be returned
					on any error from the API server.

					The API is available in multiple serialization formats - the default is
					JSON (Accept: application/json and Content-Type: application/json) but
					clients may also use YAML (application/yaml) or the native Protobuf
					schema (application/vnd.kubernetes.protobuf). Note that the format
					of the WATCH API call is slightly different - for JSON it returns newline
					delimited objects while for Protobuf it returns length-delimited frames
					(4 bytes in network-order) that contain a 'versioned.Watch' Protobuf
					object.

					See the OpenShift documentation at https://docs.openshift.org for more
					information.
				`),
			},
		},
		DefaultResponse: &spec.Response{
			ResponseProps: spec.ResponseProps{
				Description: "Default Response.",
			},
		},
		SecurityDefinitions: &securityDefinitions,
	}
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

func openshiftRequestInfoResolver() apirequest.RequestInfoResolver {
	// Default API request info factory
	requestInfoFactory := &apirequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "osapi", "oapi", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi"),
	}
	personalSARRequestInfoResolver := oauthorizer.NewPersonalSARRequestInfoResolver(requestInfoFactory)
	projectRequestInfoResolver := oauthorizer.NewProjectRequestInfoResolver(personalSARRequestInfoResolver)
	return projectRequestInfoResolver
}

func GetAuditConfig(auditConfig configapi.AuditConfig) (audit.Backend, auditpolicy.Checker, error) {
	if !auditConfig.Enabled {
		return nil, nil, nil
	}
	var (
		backend       audit.Backend
		policyChecker auditpolicy.Checker
		writer        io.Writer
	)
	if len(auditConfig.AuditFilePath) > 0 {
		writer = &lumberjack.Logger{
			Filename:   auditConfig.AuditFilePath,
			MaxAge:     auditConfig.MaximumFileRetentionDays,
			MaxBackups: auditConfig.MaximumRetainedFiles,
			MaxSize:    auditConfig.MaximumFileSizeMegabytes,
		}
	} else {
		// backwards compatible writer to regular log
		writer = cmdutil.NewGLogWriterV(0)
	}
	backend = auditlog.NewBackend(writer, auditlog.FormatJson, auditv1beta1.SchemeGroupVersion)
	policyChecker = auditpolicy.NewChecker(&auditinternal.Policy{
		// This is for backwards compatibility maintaining the old visibility, ie. just
		// raw overview of the requests comming in.
		Rules: []auditinternal.PolicyRule{{Level: auditinternal.LevelMetadata}},
	})

	// when a policy file is defined we enable the advanced auditing
	if auditConfig.PolicyConfiguration != nil || len(auditConfig.PolicyFile) > 0 {
		// policy configuration
		if auditConfig.PolicyConfiguration != nil {
			p := auditConfig.PolicyConfiguration.(*auditinternal.Policy)
			policyChecker = auditpolicy.NewChecker(p)
		} else if len(auditConfig.PolicyFile) > 0 {
			p, _ := auditpolicy.LoadPolicyFromFile(auditConfig.PolicyFile)
			policyChecker = auditpolicy.NewChecker(p)
		}

		// log configuration, only when file path was provided
		if len(auditConfig.AuditFilePath) > 0 {
			backend = auditlog.NewBackend(writer, string(auditConfig.LogFormat), auditv1beta1.SchemeGroupVersion)
		}

		// webhook configuration, only when config file was provided
		if len(auditConfig.WebHookKubeConfig) > 0 {
			webhook, err := auditwebhook.NewBackend(auditConfig.WebHookKubeConfig, auditv1beta1.SchemeGroupVersion, pluginwebhook.DefaultInitialBackoff)
			if err != nil {
				glog.Fatalf("Audit webhook initialization failed: %v", err)
			}
			backend = audit.Union(backend, webhook)
		}
	}

	return backend, policyChecker, nil
}
