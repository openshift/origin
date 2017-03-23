package kubernetes

import (
	"crypto/tls"
	"errors"
	"fmt"
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

	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	apiserveropenapi "k8s.io/kubernetes/pkg/apiserver/openapi"
	"k8s.io/kubernetes/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/authorizer"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/genericapiserver"
	kgenericfilters "k8s.io/kubernetes/pkg/genericapiserver/filters"
	openapicommon "k8s.io/kubernetes/pkg/genericapiserver/openapi/common"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/registry/cachesize"
	"k8s.io/kubernetes/pkg/registry/core/endpoint"
	endpointsetcd "k8s.io/kubernetes/pkg/registry/core/endpoint/etcd"
	"k8s.io/kubernetes/pkg/registry/generic"
	storagefactory "k8s.io/kubernetes/pkg/storage/storagebackend/factory"
	"k8s.io/kubernetes/pkg/util/config"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/intstr"
	knet "k8s.io/kubernetes/pkg/util/net"
	"k8s.io/kubernetes/pkg/util/sets"
	kversion "k8s.io/kubernetes/pkg/version"
	scheduleroptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/election"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/controller/shared"
	openapigenerated "github.com/openshift/origin/pkg/openapi"
	"github.com/openshift/origin/pkg/version"
)

// request paths that match this regular expression will be treated as long running
// and not subjected to the default server timeout.
const originLongRunningEndpointsRE = "(/|^)buildconfigs/.*/instantiatebinary$"

var LegacyAPIGroupPrefixes = sets.NewString(genericapiserver.DefaultLegacyAPIPrefix, api.Prefix, api.LegacyPrefix)

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	Options    configapi.KubernetesMasterConfig
	KubeClient kclientset.Interface

	Master            *master.Config
	ControllerManager *cmapp.CMServer
	SchedulerServer   *scheduleroptions.SchedulerServer
	CloudProvider     cloudprovider.Interface

	Informers shared.InformerFactory
}

// BuildDefaultAPIServer constructs the appropriate APIServer and StorageFactory for the kubernetes server.
// It returns an error if no KubernetesMasterConfig was defined.
func BuildDefaultAPIServer(options configapi.MasterConfig) (*apiserveroptions.ServerRunOptions, genericapiserver.StorageFactory, error) {
	if options.KubernetesMasterConfig == nil {
		return nil, nil, fmt.Errorf("no kubernetesMasterConfig defined, unable to load settings")
	}
	_, portString, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, nil, err
	}

	portRange, err := knet.ParsePortRange(options.KubernetesMasterConfig.ServicesNodePortRange)
	if err != nil {
		return nil, nil, err
	}

	// Defaults are tested in TestAPIServerDefaults
	server := apiserveroptions.NewServerRunOptions()

	// Adjust defaults
	server.EventTTL = 2 * time.Hour
	server.GenericServerRunOptions.AnonymousAuth = true
	server.GenericServerRunOptions.ServiceClusterIPRange = net.IPNet(flagtypes.DefaultIPNet(options.KubernetesMasterConfig.ServicesSubnet))
	server.GenericServerRunOptions.ServiceNodePortRange = *portRange
	server.GenericServerRunOptions.EnableProfiling = true
	server.GenericServerRunOptions.MasterCount = options.KubernetesMasterConfig.MasterCount

	server.GenericServerRunOptions.SecurePort = port
	server.GenericServerRunOptions.InsecurePort = 0
	server.GenericServerRunOptions.TLSCertFile = options.ServingInfo.ServerCert.CertFile
	server.GenericServerRunOptions.TLSPrivateKeyFile = options.ServingInfo.ServerCert.KeyFile
	server.GenericServerRunOptions.ClientCAFile = options.ServingInfo.ClientCA

	server.GenericServerRunOptions.MaxRequestsInFlight = options.ServingInfo.MaxRequestsInFlight
	server.GenericServerRunOptions.MinRequestTimeout = options.ServingInfo.RequestTimeoutSeconds
	for _, nc := range options.ServingInfo.NamedCertificates {
		sniCert := config.NamedCertKey{
			CertFile: nc.CertFile,
			KeyFile:  nc.KeyFile,
			Names:    nc.Names,
		}
		server.GenericServerRunOptions.SNICertKeys = append(server.GenericServerRunOptions.SNICertKeys, sniCert)
	}

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.APIServerArguments, server.AddFlags); len(err) > 0 {
		return nil, nil, kerrors.NewAggregate(err)
	}

	resourceEncodingConfig := genericapiserver.NewDefaultResourceEncodingConfig()
	resourceEncodingConfig.SetVersionEncoding(
		kapi.GroupName,
		unversioned.GroupVersion{Group: kapi.GroupName, Version: options.EtcdStorageConfig.KubernetesStorageVersion},
		kapi.SchemeGroupVersion,
	)

	resourceEncodingConfig.SetVersionEncoding(
		extensions.GroupName,
		unversioned.GroupVersion{Group: extensions.GroupName, Version: "v1beta1"},
		extensions.SchemeGroupVersion,
	)

	resourceEncodingConfig.SetVersionEncoding(
		batch.GroupName,
		unversioned.GroupVersion{Group: batch.GroupName, Version: "v1"},
		batch.SchemeGroupVersion,
	)

	resourceEncodingConfig.SetVersionEncoding(
		autoscaling.GroupName,
		unversioned.GroupVersion{Group: autoscaling.GroupName, Version: "v1"},
		autoscaling.SchemeGroupVersion,
	)

	storageGroupsToEncodingVersion, err := server.GenericServerRunOptions.StorageGroupsToEncodingVersion()
	if err != nil {
		return nil, nil, err
	}

	// use the stock storage config based on args, but override bits from our config where appropriate
	etcdConfig := server.GenericServerRunOptions.StorageConfig
	etcdConfig.Prefix = options.EtcdStorageConfig.KubernetesStoragePrefix
	etcdConfig.ServerList = options.EtcdClientInfo.URLs
	etcdConfig.KeyFile = options.EtcdClientInfo.ClientCert.KeyFile
	etcdConfig.CertFile = options.EtcdClientInfo.ClientCert.CertFile
	etcdConfig.CAFile = options.EtcdClientInfo.CA

	storageFactory, err := genericapiserver.BuildDefaultStorageFactory(
		etcdConfig,
		server.GenericServerRunOptions.DefaultStorageMediaType,
		kapi.Codecs,
		genericapiserver.NewDefaultResourceEncodingConfig(),
		storageGroupsToEncodingVersion,
		// FIXME: this GroupVersionResource override should be configurable
		[]unversioned.GroupVersionResource{batch.Resource("cronjobs").WithVersion("v2alpha1")},
		master.DefaultAPIResourceConfigSource(), server.GenericServerRunOptions.RuntimeConfig,
	)
	if err != nil {
		return nil, nil, err
	}

	/*storageFactory := genericapiserver.NewDefaultStorageFactory(
		etcdConfig,
		server.DefaultStorageMediaType,
		kapi.Codecs,
		resourceEncodingConfig,
		master.DefaultAPIResourceConfigSource(),
	)*/
	// the order here is important, it defines which version will be used for storage
	storageFactory.AddCohabitatingResources(batch.Resource("jobs"), extensions.Resource("jobs"))
	storageFactory.AddCohabitatingResources(extensions.Resource("horizontalpodautoscalers"), autoscaling.Resource("horizontalpodautoscalers"))

	return server, storageFactory, nil
}

// TODO this function's parameters need to be refactored
func BuildKubernetesMasterConfig(
	options configapi.MasterConfig,
	requestContextMapper kapi.RequestContextMapper,
	kubeClient kclientset.Interface,
	informers shared.InformerFactory,
	admissionControl admission.Interface,
	originAuthenticator authenticator.Request,
	kubeAuthorizer authorizer.Authorizer,
) (*MasterConfig, error) {
	if options.KubernetesMasterConfig == nil {
		return nil, errors.New("insufficient information to build KubernetesMasterConfig")
	}

	// in-order list of plug-ins that should intercept admission decisions
	// TODO: Push node environment support to upstream in future

	podEvictionTimeout, err := time.ParseDuration(options.KubernetesMasterConfig.PodEvictionTimeout)
	if err != nil {
		return nil, fmt.Errorf("unable to parse PodEvictionTimeout: %v", err)
	}

	// Defaults are tested in TestCMServerDefaults
	cmserver := cmapp.NewCMServer()
	// Adjust defaults
	cmserver.ClusterSigningCertFile = ""
	cmserver.ClusterSigningKeyFile = ""
	cmserver.Address = ""                   // no healthz endpoint
	cmserver.Port = 0                       // no healthz endpoint
	cmserver.EnableGarbageCollector = false // disabled until we add the controller
	cmserver.PodEvictionTimeout = unversioned.Duration{Duration: podEvictionTimeout}
	cmserver.VolumeConfiguration.EnableDynamicProvisioning = options.VolumeConfig.DynamicProvisioningEnabled

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.ControllerArguments, cmserver.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	schedulerserver := scheduleroptions.NewSchedulerServer()
	schedulerserver.PolicyConfigFile = options.KubernetesMasterConfig.SchedulerConfigFile
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.SchedulerArguments, schedulerserver.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	cloud, err := cloudprovider.InitCloudProvider(cmserver.CloudProvider, cmserver.CloudConfigFile)
	if err != nil {
		return nil, err
	}
	if cloud != nil {
		glog.V(2).Infof("Successfully initialized cloud provider: %q from the config file: %q\n", cmserver.CloudProvider, cmserver.CloudConfigFile)
	}

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

	server, storageFactory, err := BuildDefaultAPIServer(options)
	if err != nil {
		return nil, err
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

	genericConfig := genericapiserver.NewConfig().ApplyOptions(server.GenericServerRunOptions)

	kubeVersion := kversion.Get()
	genericConfig.Version = &kubeVersion

	// MUST be in synced with the value in CMServer
	genericConfig.EnableGarbageCollection = false // disabled until we add the controller

	genericConfig.PublicAddress = publicAddress
	genericConfig.Authenticator = originAuthenticator // this is used to fulfill the tokenreviews endpoint which is used by node authentication
	genericConfig.Authorizer = kubeAuthorizer         // this is used to fulfill the kube SAR endpoints
	genericConfig.AdmissionControl = admissionControl
	genericConfig.RequestContextMapper = requestContextMapper
	genericConfig.APIResourceConfigSource = getAPIResourceConfig(options)
	genericConfig.OpenAPIConfig = DefaultOpenAPIConfig()
	genericConfig.SwaggerConfig = genericapiserver.DefaultSwaggerConfig()
	genericConfig.SwaggerConfig.PostBuildHandler = customizeSwaggerDefinition
	_, loopbackClientConfig, err := configapi.GetKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}
	genericConfig.LoopbackClientConfig = loopbackClientConfig
	genericConfig.LegacyAPIGroupPrefixes = LegacyAPIGroupPrefixes
	genericConfig.SecureServingInfo.BindAddress = options.ServingInfo.BindAddress
	genericConfig.SecureServingInfo.BindNetwork = options.ServingInfo.BindNetwork
	genericConfig.SecureServingInfo.MinTLSVersion = crypto.TLSVersionOrDie(options.ServingInfo.MinTLSVersion)
	genericConfig.SecureServingInfo.CipherSuites = crypto.CipherSuitesOrDie(options.ServingInfo.CipherSuites)
	oAuthClientCertCAs, err := configapi.GetOAuthClientCertCAs(options)
	if err != nil {
		glog.Fatalf("Error setting up OAuth2 client certificates: %v", err)
	}
	for _, cert := range oAuthClientCertCAs {
		genericConfig.SecureServingInfo.ClientCA.AddCert(cert)
	}
	requestHeaderCACerts, err := configapi.GetRequestHeaderClientCertCAs(options)
	if err != nil {
		glog.Fatalf("Error setting up request header client certificates: %v", err)
	}
	for _, cert := range requestHeaderCACerts {
		genericConfig.SecureServingInfo.ClientCA.AddCert(cert)
	}

	url, err := url.Parse(options.MasterPublicURL)
	if err != nil {
		glog.Fatalf("Error parsing master public url %q: %v", options.MasterPublicURL, err)
	}
	genericConfig.ExternalAddress = url.Host

	originLongRunningRequestRE := regexp.MustCompile(originLongRunningEndpointsRE)
	originLongRunningFunc := kgenericfilters.BasicLongRunningRequestCheck(originLongRunningRequestRE, nil)
	kubeLongRunningFunc := genericConfig.LongRunningFunc
	genericConfig.LongRunningFunc = func(r *http.Request) bool {
		return originLongRunningFunc(r) || kubeLongRunningFunc(r)
	}

	serviceIPRange, apiServerServiceIP, err := genericapiserver.DefaultServiceIPRange(server.GenericServerRunOptions.ServiceClusterIPRange)
	if err != nil {
		glog.Fatalf("Error determining service IP ranges: %v", err)
	}

	m := &master.Config{
		GenericConfig: genericConfig,
		MasterCount:   server.GenericServerRunOptions.MasterCount,

		// Set the TLS options for proxying to pods and services
		// Proxying to nodes uses the kubeletClient TLS config (so can provide a different cert, and verify the node hostname)
		ProxyTransport: knet.SetTransportDefaults(&http.Transport{
			TLSClientConfig: &tls.Config{
				// Proxying to pods and services cannot verify hostnames, since they are contacted on randomly allocated IPs
				InsecureSkipVerify: true,
				Certificates:       proxyClientCerts,
			},
		}),

		EnableWatchCache:          server.GenericServerRunOptions.EnableWatchCache,
		ServiceIPRange:            serviceIPRange,
		APIServerServiceIP:        apiServerServiceIP,
		APIServerServicePort:      443,
		ServiceNodePortRange:      server.GenericServerRunOptions.ServiceNodePortRange,
		KubernetesServiceNodePort: server.GenericServerRunOptions.KubernetesServiceNodePort,

		StorageFactory: storageFactory,

		EventTTL: server.EventTTL,

		KubeletClientConfig: *configapi.GetKubeletClientConfig(options),

		EnableLogsSupport:     false, // don't expose server logs
		EnableCoreControllers: true,

		DeleteCollectionWorkers: server.GenericServerRunOptions.DeleteCollectionWorkers,
	}

	if m.EnableWatchCache {
		cachesize.SetWatchCacheSizes(server.GenericServerRunOptions.WatchCacheSizes)
	}

	if m.EnableCoreControllers {
		glog.V(2).Info("Using the lease endpoint reconciler")
		config, err := m.StorageFactory.NewConfig(kapi.Resource("apiServerIPInfo"))
		if err != nil {
			return nil, err
		}
		leaseStorage, _, err := storagefactory.Create(*config)
		if err != nil {
			return nil, err
		}
		masterLeases := newMasterLeases(leaseStorage)

		endpointConfig, err := m.StorageFactory.NewConfig(kapi.Resource("endpoints"))
		if err != nil {
			return nil, err
		}
		endpointsStorage := endpointsetcd.NewREST(generic.RESTOptions{
			StorageConfig:           endpointConfig,
			Decorator:               generic.UndecoratedStorage,
			DeleteCollectionWorkers: 0,
			ResourcePrefix:          m.StorageFactory.ResourcePrefix(kapi.Resource("endpoints")),
		})

		endpointRegistry := endpoint.NewRegistry(endpointsStorage)

		m.EndpointReconcilerConfig = master.EndpointReconcilerConfig{
			Reconciler: election.NewLeaseEndpointReconciler(endpointRegistry, masterLeases),
			Interval:   master.DefaultEndpointReconcilerInterval,
		}
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
			kapi.EndpointPort{Name: "dns", Port: int32(dnsPort), Protocol: kapi.ProtocolUDP},
			kapi.EndpointPort{Name: "dns-tcp", Port: int32(dnsPort), Protocol: kapi.ProtocolTCP},
		)
	}

	kmaster := &MasterConfig{
		Options:    *options.KubernetesMasterConfig,
		KubeClient: kubeClient,

		Master:            m,
		ControllerManager: cmserver,
		CloudProvider:     cloud,
		SchedulerServer:   schedulerserver,
		Informers:         informers,
	}

	return kmaster, nil
}

func DefaultOpenAPIConfig() *openapicommon.Config {
	return &openapicommon.Config{
		Definitions:    openapigenerated.OpenAPIDefinitions,
		IgnorePrefixes: []string{"/swaggerapi", "/healthz", "/controllers", "/metrics", "/version/openshift", "/brokers"},
		GetOperationIDAndTags: func(servePath string, r *restful.Route) (string, []string, error) {
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
			} else if strings.HasPrefix(path, "/oapi/v1/namespaces/{namespace}/generatedeploymentconfigs") {
				op = "generateNamespacedDeploymentConfig"
			}
			if op != r.Operation {
				return op, []string{}, nil
			}
			return apiserveropenapi.GetOperationIDAndTags(servePath, r)
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
					error type is 'unversioned.Status' (described below) and will be returned
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
	}
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

// TODO remove this func in 1.6 when we get rid of the hack above
func concatenateFiles(prefix, separator string, files ...string) (string, error) {
	data := []byte{}
	for _, file := range files {
		fileBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return "", err
		}
		data = append(data, fileBytes...)
		data = append(data, []byte(separator)...)
	}
	tmpFile, err := ioutil.TempFile("", prefix)
	if err != nil {
		return "", err
	}
	if _, err := tmpFile.Write(data); err != nil {
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}
