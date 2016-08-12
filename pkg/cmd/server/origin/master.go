package origin

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kubeapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	v1beta1extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/genericapiserver"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/api/v1"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildetcd "github.com/openshift/origin/pkg/build/registry/build/etcd"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	deploylogregistry "github.com/openshift/origin/pkg/deploy/registry/deploylog"
	deployrollback "github.com/openshift/origin/pkg/deploy/registry/rollback"
	"github.com/openshift/origin/pkg/dockerregistry"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	"github.com/openshift/origin/pkg/image/importer"
	imageimporter "github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagesecret"
	"github.com/openshift/origin/pkg/image/registry/imagesignature"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimport"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	projectproxy "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequeststorage "github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/route/etcd"
	clusternetworketcd "github.com/openshift/origin/pkg/sdn/registry/clusternetwork/etcd"
	egressnetworkpolicyetcd "github.com/openshift/origin/pkg/sdn/registry/egressnetworkpolicy/etcd"
	hostsubnetetcd "github.com/openshift/origin/pkg/sdn/registry/hostsubnet/etcd"
	netnamespaceetcd "github.com/openshift/origin/pkg/sdn/registry/netnamespace/etcd"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	templateetcd "github.com/openshift/origin/pkg/template/registry/etcd"
	groupetcd "github.com/openshift/origin/pkg/user/registry/group/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	"github.com/openshift/origin/pkg/version"

	"github.com/openshift/origin/pkg/build/registry/buildclone"
	"github.com/openshift/origin/pkg/build/registry/buildconfiginstantiate"

	appliedclusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/appliedclusterresourcequota"
	clusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/clusterresourcequota"

	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicystorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	clusterrolestorage "github.com/openshift/origin/pkg/authorization/registry/clusterrole/proxy"
	clusterrolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding/proxy"
	"github.com/openshift/origin/pkg/authorization/registry/localresourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/localsubjectaccessreview"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
	"github.com/openshift/origin/pkg/authorization/registry/selfsubjectrulesreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
)

const (
	LegacyOpenShiftAPIPrefix = "/osapi" // TODO: make configurable
	OpenShiftAPIPrefix       = "/oapi"  // TODO: make configurable
	KubernetesAPIPrefix      = "/api"   // TODO: make configurable
	KubernetesAPIGroupPrefix = "/apis"  // TODO: make configurable
	OpenShiftAPIV1           = "v1"
	OpenShiftAPIPrefixV1     = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1
	swaggerAPIPrefix         = "/swaggerapi/"
)

var (
	excludedV1Types = sets.NewString()

	// TODO: correctly solve identifying requests by type
	longRunningRE = regexp.MustCompile("watch|proxy|logs?|exec|portforward|attach")
)

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// InstallAPI returns an array of strings describing what was installed
	InstallAPI(*restful.Container) ([]string, error)
}

// APIInstallFunc is a function for installing APIs
type APIInstallFunc func(*restful.Container) ([]string, error)

// InstallAPI implements APIInstaller
func (fn APIInstallFunc) InstallAPI(container *restful.Container) ([]string, error) {
	return fn(container)
}

// Run launches the OpenShift master. It takes optional installers that may install additional endpoints into the server.
// All endpoints get configured CORS behavior
// Protected installers' endpoints are protected by API authentication and authorization.
// Unprotected installers' endpoints do not have any additional protection added.
func (c *MasterConfig) Run(protected []APIInstaller, unprotected []APIInstaller) {
	var extra []string

	safe := genericapiserver.NewHandlerContainer(http.NewServeMux(), kapi.Codecs)
	open := genericapiserver.NewHandlerContainer(http.NewServeMux(), kapi.Codecs)

	// enforce authentication on protected endpoints
	protected = append(protected, APIInstallFunc(c.InstallProtectedAPI))
	for _, i := range protected {
		msgs, err := i.InstallAPI(safe)
		if err != nil {
			glog.Fatalf("error installing api %v", err)
		}
		extra = append(extra, msgs...)
	}
	handler := c.versionSkewFilter(safe)
	handler = c.authorizationFilter(handler)
	handler = c.impersonationFilter(handler)
	// audit handler must comes before the impersonationFilter to read the original user
	handler = c.auditHandler(handler)
	handler = authenticationHandlerFilter(handler, c.Authenticator, c.getRequestContextMapper())
	handler = namespacingFilter(handler, c.getRequestContextMapper())
	handler = cacheControlFilter(handler, "no-store") // protected endpoints should not be cached

	// unprotected resources
	unprotected = append(unprotected, APIInstallFunc(c.InstallUnprotectedAPI))
	for _, i := range unprotected {
		msgs, err := i.InstallAPI(open)
		if err != nil {
			glog.Fatalf("error installing api %v", err)
		}
		extra = append(extra, msgs...)
	}

	var kubeAPILevels []string
	if c.Options.KubernetesMasterConfig != nil {
		kubeAPILevels = configapi.GetEnabledAPIVersionsForGroup(*c.Options.KubernetesMasterConfig, kapi.GroupName)
	}

	handler = indexAPIPaths(c.Options.APILevels, kubeAPILevels, handler)

	open.Handle("/", handler)

	// install swagger
	swaggerConfig := swagger.Config{
		WebServicesUrl:   c.Options.MasterPublicURL,
		WebServices:      append(safe.RegisteredWebServices(), open.RegisteredWebServices()...),
		ApiPath:          swaggerAPIPrefix,
		PostBuildHandler: customizeSwaggerDefinition,
	}
	// log nothing from swagger
	swagger.LogInfo = func(format string, v ...interface{}) {}
	swagger.RegisterSwaggerService(swaggerConfig, open)
	extra = append(extra, fmt.Sprintf("Started Swagger Schema API at %%s%s", swaggerAPIPrefix))

	handler = open

	// add CORS support
	if origins := c.ensureCORSAllowedOrigins(); len(origins) != 0 {
		handler = apiserver.CORS(handler, origins, nil, nil, "true")
	}

	if c.WebConsoleEnabled() {
		handler = assetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
	}

	// Make the outermost filter the requestContextMapper to ensure all components share the same context
	if contextHandler, err := kapi.NewRequestContextFilter(c.getRequestContextMapper(), handler); err != nil {
		glog.Fatalf("Error setting up request context filter: %v", err)
	} else {
		handler = contextHandler
	}

	longRunningRequestCheck := apiserver.BasicLongRunningRequestCheck(longRunningRE, map[string]string{"watch": "true"})
	// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
	// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
	if c.Options.ServingInfo.MaxRequestsInFlight > 0 {
		sem := make(chan bool, c.Options.ServingInfo.MaxRequestsInFlight)
		handler = apiserver.MaxInFlightLimit(sem, longRunningRequestCheck, handler)
	}

	c.serve(handler, extra)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) RunHealth() {
	ws := &restful.WebService{}
	mux := http.NewServeMux()
	hc := genericapiserver.NewHandlerContainer(mux, kapi.Codecs)
	hc.Add(ws)

	initHealthCheckRoute(ws, "/healthz")
	initReadinessCheckRoute(ws, "/healthz/ready", func() bool { return true })
	initMetricsRoute(ws, "/metrics")

	c.serve(hc, []string{"Started health checks at %s"})
}

// serve starts serving the provided http.Handler using security settings derived from the MasterConfig
func (c *MasterConfig) serve(handler http.Handler, extra []string) {
	timeout := c.Options.ServingInfo.RequestTimeoutSeconds
	if timeout == -1 {
		timeout = 0
	}

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    time.Duration(timeout) * time.Second,
		WriteTimeout:   time.Duration(timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go utilwait.Forever(func() {
		for _, s := range extra {
			glog.Infof(s, c.Options.ServingInfo.BindAddress)
		}
		if c.TLS {
			extraCerts, err := configapi.GetNamedCertificateMap(c.Options.ServingInfo.NamedCertificates)
			if err != nil {
				glog.Fatal(err)
			}
			server.TLSConfig = crypto.SecureTLSConfig(&tls.Config{
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  c.ClientCAs,
				// Set SNI certificate func
				GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
			})
			glog.Fatal(cmdutil.ListenAndServeTLS(server, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)
}

// InitializeObjects ensures objects in Kubernetes and etcd are properly populated.
// Requires a Kube client to be established and that etcd be started.
func (c *MasterConfig) InitializeObjects() {
	// Create required policy rules if needed
	c.ensureComponentAuthorizationRules()
	// Ensure the default SCCs are created
	c.ensureDefaultSecurityContextConstraints()
	// Bind default roles for service accounts in the default namespace if needed
	c.ensureDefaultNamespaceServiceAccountRoles()
	// Create the infra namespace
	c.ensureOpenShiftInfraNamespace()
	// Create the shared resource namespace
	c.ensureOpenShiftSharedResourcesNamespace()
}

func (c *MasterConfig) InstallProtectedAPI(container *restful.Container) ([]string, error) {
	// initialize OpenShift API
	storage := c.GetRestStorage()

	messages := []string{}
	legacyAPIVersions := []string{}
	currentAPIVersions := []string{}

	if configapi.HasOpenShiftAPILevel(c.Options, OpenShiftAPIV1) {
		if err := c.apiLegacyV1(storage).InstallREST(container); err != nil {
			glog.Fatalf("Unable to initialize v1 API: %v", err)
		}
		messages = append(messages, fmt.Sprintf("Started Origin API at %%s%s", OpenShiftAPIPrefixV1))
		currentAPIVersions = append(currentAPIVersions, OpenShiftAPIV1)
	}

	var root *restful.WebService
	for _, service := range container.RegisteredWebServices() {
		switch service.RootPath() {
		case "/":
			root = service
		case OpenShiftAPIPrefixV1:
			service.Doc("OpenShift REST API, version v1").ApiVersion("v1")
		}
	}

	if root == nil {
		root = new(restful.WebService)
		container.Add(root)
	}

	// The old API prefix must continue to return 200 (with an empty versions
	// list) for backwards compatibility, even though we won't service any other
	// requests through the route. Take care when considering whether to delete
	// this route.
	initAPIVersionRoute(root, LegacyOpenShiftAPIPrefix, legacyAPIVersions...)
	initAPIVersionRoute(root, OpenShiftAPIPrefix, currentAPIVersions...)

	initControllerRoutes(root, "/controllers", c.Options.Controllers != configapi.ControllersDisabled, c.ControllerPlug)
	initHealthCheckRoute(root, "/healthz")
	initReadinessCheckRoute(root, "/healthz/ready", c.ProjectAuthorizationCache.ReadyForAccess)
	initVersionRoute(container, "/version/openshift")

	return messages, nil
}

// initReadinessCheckRoute initializes an HTTP endpoint for readiness checking
func initVersionRoute(container *restful.Container, path string) {
	// Set up a service to return the git code version.
	versionWS := new(restful.WebService)
	versionWS.Path(path)
	versionWS.Doc("git code version from which this is built")
	versionWS.Route(
		versionWS.GET("/").To(handleVersion).
			Doc("get the code version").
			Operation("getCodeVersion").
			Produces(restful.MIME_JSON).
			Consumes(restful.MIME_JSON))

	container.Add(versionWS)
}

// handleVersion writes the server's version information.
func handleVersion(req *restful.Request, resp *restful.Response) {
	output, err := json.MarshalIndent(version.Get(), "", "  ")
	if err != nil {
		http.Error(resp.ResponseWriter, err.Error(), http.StatusInternalServerError)
		return
	}
	resp.ResponseWriter.Header().Set("Content-Type", "application/json")
	resp.ResponseWriter.WriteHeader(http.StatusOK)
	resp.ResponseWriter.Write(output)
}

func (c *MasterConfig) GetRestStorage() map[string]rest.Storage {
	kubeletClient, err := kubeletclient.NewStaticKubeletClient(c.KubeletClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	// TODO: allow the system CAs and the local CAs to be joined together.
	importTransport, err := restclient.TransportFor(&restclient.Config{})
	if err != nil {
		glog.Fatalf("Unable to configure a default transport for importing: %v", err)
	}
	insecureImportTransport, err := restclient.TransportFor(&restclient.Config{Insecure: true})
	if err != nil {
		glog.Fatalf("Unable to configure a default transport for importing: %v", err)
	}

	buildStorage, buildDetailsStorage, err := buildetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	buildRegistry := buildregistry.NewRegistry(buildStorage)

	buildConfigStorage, err := buildconfigetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	buildConfigRegistry := buildconfigregistry.NewRegistry(buildConfigStorage)

	deployConfigStorage, deployConfigStatusStorage, deployConfigScaleStorage, err := deployconfigetcd.NewREST(c.RESTOptionsGetter, c.DeploymentConfigScaleClient())
	checkStorageErr(err)
	deployConfigRegistry := deployconfigregistry.NewRegistry(deployConfigStorage)

	routeAllocator := c.RouteAllocator()

	routeStorage, routeStatusStorage, err := routeetcd.NewREST(c.RESTOptionsGetter, routeAllocator)
	checkStorageErr(err)

	hostSubnetStorage, err := hostsubnetetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	netNamespaceStorage, err := netnamespaceetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	clusterNetworkStorage, err := clusternetworketcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	egressNetworkPolicyStorage, err := egressnetworkpolicyetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)

	userStorage, err := useretcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage, err := identityetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	identityRegistry := identityregistry.NewRegistry(identityStorage)
	userIdentityMappingStorage := useridentitymapping.NewREST(userRegistry, identityRegistry)
	groupStorage, err := groupetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)

	policyStorage, err := policyetcd.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	policyRegistry := policyregistry.NewRegistry(policyStorage)
	policyBindingStorage, err := policybindingetcd.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	policyBindingRegistry := policybindingregistry.NewRegistry(policyBindingStorage)

	clusterPolicyStorage, err := clusterpolicystorage.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterPolicyStorage)
	clusterPolicyBindingStorage, err := clusterpolicybindingstorage.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterPolicyBindingStorage)

	selfSubjectRulesReviewStorage := selfsubjectrulesreview.NewREST(c.RuleResolver, c.Informers.ClusterPolicies().Lister().ClusterPolicies())

	roleStorage := rolestorage.NewVirtualStorage(policyRegistry, c.RuleResolver)
	roleBindingStorage := rolebindingstorage.NewVirtualStorage(policyBindingRegistry, c.RuleResolver)
	clusterRoleStorage := clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry)
	clusterRoleBindingStorage := clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry)

	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)
	localSubjectAccessReviewStorage := localsubjectaccessreview.NewREST(subjectAccessReviewRegistry)
	resourceAccessReviewStorage := resourceaccessreview.NewREST(c.Authorizer)
	resourceAccessReviewRegistry := resourceaccessreview.NewRegistry(resourceAccessReviewStorage)
	localResourceAccessReviewStorage := localresourceaccessreview.NewREST(resourceAccessReviewRegistry)

	imageStorage, err := imageetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	imageRegistry := image.NewRegistry(imageStorage)
	imageSignatureStorage := imagesignature.NewREST(c.PrivilegedLoopbackOpenShiftClient.Images())
	imageStreamLimitVerifier := imageadmission.NewLimitVerifier(c.KubeClient())
	imageStreamSecretsStorage := imagesecret.NewREST(c.ImageStreamSecretClient())
	imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage, err := imagestreametcd.NewREST(c.RESTOptionsGetter, c.RegistryNameFn, subjectAccessReviewRegistry, imageStreamLimitVerifier)
	checkStorageErr(err)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry, c.RegistryNameFn)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	importerFn := func(r importer.RepositoryRetriever) imageimporter.Interface {
		return imageimporter.NewImageStreamImporter(r, c.Options.ImagePolicyConfig.MaxImagesBulkImportedPerRepository, flowcontrol.NewTokenBucketRateLimiter(2.0, 3))
	}
	importerDockerClientFn := func() dockerregistry.Client {
		return dockerregistry.NewClient(20*time.Second, false)
	}
	imageStreamImportStorage := imagestreamimport.NewREST(importerFn, imageStreamRegistry, internalImageStreamStorage, imageStorage, c.ImageStreamImportSecretClient(), importTransport, insecureImportTransport, importerDockerClientFn)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:      buildConfigRegistry.GetBuildConfig,
			UpdateBuildConfigFunc:   buildConfigRegistry.UpdateBuildConfig,
			GetBuildFunc:            buildRegistry.GetBuild,
			CreateBuildFunc:         buildRegistry.CreateBuild,
			GetImageStreamFunc:      imageStreamRegistry.GetImageStream,
			GetImageStreamImageFunc: imageStreamImageRegistry.GetImageStreamImage,
			GetImageStreamTagFunc:   imageStreamTagRegistry.GetImageStreamTag,
		},
		ServiceAccounts: c.KubeClient(),
		Secrets:         c.KubeClient(),
	}

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployConfigRegistry.GetDeploymentConfig,
			ISFn:   imageStreamRegistry.GetImageStream,
			LISFn2: imageStreamRegistry.ListImageStreams,
		},
	}
	configClient, kclient := c.DeploymentConfigClients()
	deployRollbackClient := deployrollback.Client{
		DCFn: deployConfigRegistry.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{kclient}.GetDeployment,
		GRFn: deployrollback.NewRollbackGenerator().GenerateRollback,
	}
	deployConfigRollbackStorage := deployrollback.NewREST(configClient, kclient, c.EtcdHelper.Codec())

	projectStorage := projectproxy.NewREST(kclient.Namespaces(), c.ProjectAuthorizationCache, c.ProjectAuthorizationCache, c.ProjectCache)

	namespace, templateName, err := configapi.ParseNamespaceAndName(c.Options.ProjectConfig.ProjectRequestTemplate)
	if err != nil {
		glog.Errorf("Error parsing project request template value: %v", err)
		// we can continue on, the storage that gets created will be valid, it simply won't work properly.  There's no reason to kill the master
	}
	projectRequestStorage := projectrequeststorage.NewREST(c.Options.ProjectConfig.ProjectRequestMessage, namespace, templateName, c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient, c.Informers.PolicyBindings().Lister())

	bcClient := c.BuildConfigWebHookClient()
	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildConfigRegistry,
		buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		},
	)

	clientStorage, err := clientetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	clientRegistry := clientregistry.NewRegistry(clientStorage)

	// If OAuth is disabled, set the strategy to Deny
	saAccountGrantMethod := oauthapi.GrantHandlerDeny
	if c.Options.OAuthConfig != nil {
		// Otherwise, take the value provided in master-config.yaml
		saAccountGrantMethod = oauthapi.GrantHandlerType(c.Options.OAuthConfig.GrantConfig.ServiceAccountMethod)
	}

	combinedOAuthClientGetter := saoauth.NewServiceAccountOAuthClientGetter(c.KubeClient(), c.KubeClient(), clientRegistry, saAccountGrantMethod)
	authorizeTokenStorage, err := authorizetokenetcd.NewREST(c.RESTOptionsGetter, combinedOAuthClientGetter)
	checkStorageErr(err)
	accessTokenStorage, err := accesstokenetcd.NewREST(c.RESTOptionsGetter, combinedOAuthClientGetter)
	checkStorageErr(err)
	clientAuthorizationStorage, err := clientauthetcd.NewREST(c.RESTOptionsGetter, combinedOAuthClientGetter)
	checkStorageErr(err)

	templateStorage, err := templateetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)

	storage := map[string]rest.Storage{
		"images":               imageStorage,
		"imagesignatures":      imageSignatureStorage,
		"imageStreams/secrets": imageStreamSecretsStorage,
		"imageStreams":         imageStreamStorage,
		"imageStreams/status":  imageStreamStatusStorage,
		"imageStreamImports":   imageStreamImportStorage,
		"imageStreamImages":    imageStreamImageStorage,
		"imageStreamMappings":  imageStreamMappingStorage,
		"imageStreamTags":      imageStreamTagStorage,

		"deploymentConfigs":          deployConfigStorage,
		"deploymentConfigs/scale":    deployConfigScaleStorage,
		"deploymentConfigs/status":   deployConfigStatusStorage,
		"deploymentConfigs/rollback": deployConfigRollbackStorage,
		"deploymentConfigs/log":      deploylogregistry.NewREST(configClient, kclient, c.DeploymentLogClient(), kubeletClient),

		// TODO: Deprecate these
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, c.EtcdHelper.Codec()),
		"deploymentConfigRollbacks": deployrollback.NewDeprecatedREST(deployRollbackClient, c.EtcdHelper.Codec()),

		"processedTemplates": templateregistry.NewREST(),
		"templates":          templateStorage,

		"routes":        routeStorage,
		"routes/status": routeStatusStorage,

		"projects":        projectStorage,
		"projectRequests": projectRequestStorage,

		"hostSubnets":           hostSubnetStorage,
		"netNamespaces":         netNamespaceStorage,
		"clusterNetworks":       clusterNetworkStorage,
		"egressNetworkPolicies": egressNetworkPolicyStorage,

		"users":                userStorage,
		"groups":               groupStorage,
		"identities":           identityStorage,
		"userIdentityMappings": userIdentityMappingStorage,

		"oAuthAuthorizeTokens":      authorizeTokenStorage,
		"oAuthAccessTokens":         accessTokenStorage,
		"oAuthClients":              clientStorage,
		"oAuthClientAuthorizations": clientAuthorizationStorage,

		"resourceAccessReviews":      resourceAccessReviewStorage,
		"subjectAccessReviews":       subjectAccessReviewStorage,
		"localSubjectAccessReviews":  localSubjectAccessReviewStorage,
		"localResourceAccessReviews": localResourceAccessReviewStorage,
		"selfSubjectRulesReviews":    selfSubjectRulesReviewStorage,

		"policies":       policyStorage,
		"policyBindings": policyBindingStorage,
		"roles":          roleStorage,
		"roleBindings":   roleBindingStorage,

		"clusterPolicies":       clusterPolicyStorage,
		"clusterPolicyBindings": clusterPolicyBindingStorage,
		"clusterRoleBindings":   clusterRoleBindingStorage,
		"clusterRoles":          clusterRoleStorage,

		"clusterResourceQuotas":        restInPeace(clusterresourcequotaregistry.NewStorage(c.RESTOptionsGetter)),
		"clusterResourceQuotas/status": updateInPeace(clusterresourcequotaregistry.NewStatusStorage(c.RESTOptionsGetter)),
		"appliedClusterResourceQuotas": appliedclusterresourcequotaregistry.NewREST(
			c.ClusterQuotaMappingController.GetClusterQuotaMapper(), c.Informers.ClusterResourceQuotas().Lister(), c.Informers.Namespaces().Lister()),
	}

	if configapi.IsBuildEnabled(&c.Options) {
		storage["builds"] = buildStorage
		storage["buildConfigs"] = buildConfigStorage
		storage["buildConfigs/webhooks"] = buildConfigWebHooks
		storage["builds/clone"] = buildclone.NewStorage(buildGenerator)
		storage["buildConfigs/instantiate"] = buildconfiginstantiate.NewStorage(buildGenerator)
		storage["buildConfigs/instantiatebinary"] = buildconfiginstantiate.NewBinaryStorage(buildGenerator, buildStorage, c.BuildLogClient(), kubeletClient)
		storage["builds/log"] = buildlogregistry.NewREST(buildStorage, buildStorage, c.BuildLogClient(), kubeletClient)
		storage["builds/details"] = buildDetailsStorage
	}

	return storage
}

func checkStorageErr(err error) {
	if err != nil {
		glog.Fatalf("Error building REST storage: %v", err)
	}
}

func (c *MasterConfig) InstallUnprotectedAPI(container *restful.Container) ([]string, error) {
	return []string{}, nil
}

// initAPIVersionRoute initializes the osapi endpoint to behave similar to the upstream api endpoint
func initAPIVersionRoute(root *restful.WebService, prefix string, versions ...string) {
	versionHandler := apiserver.APIVersionHandler(kapi.Codecs, func(req *restful.Request) *unversioned.APIVersions {
		apiVersionsForDiscovery := unversioned.APIVersions{
			// TODO: ServerAddressByClientCIDRs: s.getServerAddressByClientCIDRs(req.Request),
			Versions: versions,
		}
		return &apiVersionsForDiscovery
	})
	root.Route(root.GET(prefix).To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON))
}

// initHealthCheckRoute initializes an HTTP endpoint for health checking.
// OpenShift is deemed healthy if the API server can respond with an OK messages
func initHealthCheckRoute(root *restful.WebService, path string) {
	root.Route(root.GET(path).To(func(req *restful.Request, resp *restful.Response) {
		resp.ResponseWriter.WriteHeader(http.StatusOK)
		resp.ResponseWriter.Write([]byte("ok"))
	}).Doc("return the health state of the master").
		Returns(http.StatusOK, "if master is healthy", nil).
		Produces(restful.MIME_JSON))
}

// initReadinessCheckRoute initializes an HTTP endpoint for readiness checking
func initReadinessCheckRoute(root *restful.WebService, path string, readyFunc func() bool) {
	root.Route(root.GET(path).To(func(req *restful.Request, resp *restful.Response) {
		if readyFunc() {
			resp.ResponseWriter.WriteHeader(http.StatusOK)
			resp.ResponseWriter.Write([]byte("ok"))

		} else {
			resp.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
		}
	}).Doc("return the readiness state of the master").
		Returns(http.StatusOK, "if the master is ready", nil).
		Returns(http.StatusServiceUnavailable, "if the master is not ready", nil).
		Produces(restful.MIME_JSON))
}

// initHealthCheckRoute initializes an HTTP endpoint for health checking.
// OpenShift is deemed healthy if the API server can respond with an OK messages
func initMetricsRoute(root *restful.WebService, path string) {
	h := prometheus.Handler()
	root.Route(root.GET(path).To(func(req *restful.Request, resp *restful.Response) {
		h.ServeHTTP(resp.ResponseWriter, req.Request)
	}).Doc("return metrics for this process").
		Returns(http.StatusOK, "if metrics are available", nil).
		Produces("text/plain"))
}

func (c *MasterConfig) defaultAPIGroupVersion() *apiserver.APIGroupVersion {
	var restMapper meta.MultiRESTMapper
	seenGroups := sets.String{}
	for _, gv := range registered.EnabledVersions() {
		if seenGroups.Has(gv.Group) {
			continue
		}
		seenGroups.Insert(gv.Group)

		groupMeta, err := registered.Group(gv.Group)
		if err != nil {
			continue
		}
		restMapper = meta.MultiRESTMapper(append(restMapper, groupMeta.RESTMapper))
	}

	statusMapper := meta.NewDefaultRESTMapper([]unversioned.GroupVersion{kubeapiv1.SchemeGroupVersion}, registered.GroupOrDie(kapi.GroupName).InterfacesFor)
	statusMapper.Add(kubeapiv1.SchemeGroupVersion.WithKind("Status"), meta.RESTScopeRoot)
	restMapper = meta.MultiRESTMapper(append(restMapper, statusMapper))

	return &apiserver.APIGroupVersion{
		Root: OpenShiftAPIPrefix,

		Mapper: restMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Copier:    kapi.Scheme,
		Linker:    registered.GroupOrDie("").SelfLinker,

		Admit:                       c.AdmissionControl,
		Context:                     c.getRequestContextMapper(),
		SubresourceGroupVersionKind: map[string]unversioned.GroupVersionKind{},
	}
}

// apiLegacyV1 returns the resources and codec for API version v1.
func (c *MasterConfig) apiLegacyV1(all map[string]rest.Storage) *apiserver.APIGroupVersion {
	storage := make(map[string]rest.Storage)
	for k, v := range all {
		if excludedV1Types.Has(k) {
			continue
		}
		storage[strings.ToLower(k)] = v
	}
	version := c.defaultAPIGroupVersion()
	version.Storage = storage
	version.GroupVersion = v1.SchemeGroupVersion
	version.Serializer = kapi.Codecs
	version.ParameterCodec = runtime.NewParameterCodec(kapi.Scheme)
	version.SubresourceGroupVersionKind["deploymentconfigs/scale"] = v1beta1extensions.SchemeGroupVersion.WithKind("Scale")
	return version
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() kapi.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = kapi.NewRequestContextMapper()
	}
	return c.RequestContextMapper
}

// RouteAllocator returns a route allocation controller.
func (c *MasterConfig) RouteAllocator() *routeallocationcontroller.RouteAllocationController {
	osclient, kclient := c.RouteAllocatorClients()
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		OSClient:   osclient,
		KubeClient: kclient,
	}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(c.Options.RoutingConfig.Subdomain)
	if err != nil {
		glog.Fatalf("Route plugin initialization failed: %v", err)
	}

	return factory.Create(plugin)
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

// GetDeployment returns the deployment with the provided context and name
func (c clientDeploymentInterface) GetDeployment(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(kapi.NamespaceValue(ctx)).Get(name)
}
