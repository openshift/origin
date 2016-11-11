package origin

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	restful "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/go-openapi/spec"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/natefinch/lumberjack.v2"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kubeapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	v1beta1extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/apiserver/audit"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	"k8s.io/kubernetes/pkg/genericapiserver"
	"k8s.io/kubernetes/pkg/genericapiserver/openapi"
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
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	deploylogregistry "github.com/openshift/origin/pkg/deploy/registry/deploylog"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/registry/generator"
	deployconfiginstantiate "github.com/openshift/origin/pkg/deploy/registry/instantiate"
	deployrollback "github.com/openshift/origin/pkg/deploy/registry/rollback"
	"github.com/openshift/origin/pkg/dockerregistry"
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
	"github.com/openshift/origin/pkg/oauth/discovery"
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

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
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
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyselfsubjectreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview"
	oscc "github.com/openshift/origin/pkg/security/scc"
)

const (
	LegacyOpenShiftAPIPrefix = "/osapi" // TODO: make configurable
	OpenShiftAPIPrefix       = "/oapi"  // TODO: make configurable
	KubernetesAPIPrefix      = "/api"   // TODO: make configurable
	KubernetesAPIGroupPrefix = "/apis"  // TODO: make configurable
	OpenShiftAPIV1           = "v1"
	OpenShiftAPIPrefixV1     = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1
	swaggerAPIPrefix         = "/swaggerapi/"
	// Discovery endpoint for OAuth 2.0 Authorization Server Metadata
	// See IETF Draft:
	// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	oauthMetadataEndpoint = "/.well-known/oauth-authorization-server"
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
	if c.Options.AuditConfig.Enabled {
		attributeGetter := apiserver.NewRequestAttributeGetter(c.getRequestContextMapper(), c.getRequestInfoResolver())
		var writer io.Writer
		if len(c.Options.AuditConfig.AuditFilePath) > 0 {
			writer = &lumberjack.Logger{
				Filename:   c.Options.AuditConfig.AuditFilePath,
				MaxAge:     c.Options.AuditConfig.MaximumFileRetentionDays,
				MaxBackups: c.Options.AuditConfig.MaximumRetainedFiles,
				MaxSize:    c.Options.AuditConfig.MaximumFileSizeMegabytes,
			}
		} else {
			// backwards compatible writer to regular log
			writer = cmdutil.NewGLogWriterV(0)
		}
		handler = audit.WithAudit(handler, attributeGetter, writer)
	}
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

	openAPIConfig := openapi.Config{
		SwaggerConfig:  &swaggerConfig,
		IgnorePrefixes: []string{"/swaggerapi"},
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
	err := openapi.RegisterOpenAPIService(&openAPIConfig, open)
	if err != nil {
		glog.Fatalf("Failed to generate open api spec: %v", err)
	}
	extra = append(extra, fmt.Sprintf("Started OpenAPI Schema at %%s%s", openapi.OpenAPIServePath))

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

	// Set up OAuth metadata only if we are configured to use OAuth
	if c.Options.OAuthConfig != nil {
		initOAuthAuthorizationServerMetadataRoute(container, oauthMetadataEndpoint, c.Options.OAuthConfig.MasterPublicURL)
	}

	return messages, nil
}

// initVersionRoute initializes an HTTP endpoint for the server's version information.
func initVersionRoute(container *restful.Container, path string) {
	// Build version info once
	versionInfo, err := json.MarshalIndent(version.Get(), "", "  ")
	if err != nil {
		glog.Errorf("Unable to initialize version route: %v", err)
		return
	}

	// Set up a service to return the git code version.
	versionWS := new(restful.WebService)
	versionWS.Path(path)
	versionWS.Doc("git code version from which this is built")
	versionWS.Route(
		versionWS.GET("/").To(func(_ *restful.Request, resp *restful.Response) {
			writeJSON(resp, versionInfo)
		}).
			Doc("get the code version").
			Operation("getCodeVersion").
			Produces(restful.MIME_JSON))

	container.Add(versionWS)
}

func writeJSON(resp *restful.Response, json []byte) {
	resp.ResponseWriter.Header().Set("Content-Type", "application/json")
	resp.ResponseWriter.WriteHeader(http.StatusOK)
	resp.ResponseWriter.Write(json)
}

// initOAuthAuthorizationServerMetadataRoute initializes an HTTP endpoint for OAuth 2.0 Authorization Server Metadata discovery
// https://tools.ietf.org/id/draft-ietf-oauth-discovery-04.html#rfc.section.2
// masterPublicURL should be internally and externally routable to allow all users to discover this information
func initOAuthAuthorizationServerMetadataRoute(container *restful.Container, path, masterPublicURL string) {
	// Build OAuth metadata once
	metadata, err := json.MarshalIndent(discovery.Get(masterPublicURL, OpenShiftOAuthAuthorizeURL(masterPublicURL), OpenShiftOAuthTokenURL(masterPublicURL)), "", "  ")
	if err != nil {
		glog.Errorf("Unable to initialize OAuth authorization server metadata route: %v", err)
		return
	}

	// Set up a service to return the OAuth metadata.
	oauthWS := new(restful.WebService)
	oauthWS.Path(path)
	oauthWS.Doc("OAuth 2.0 Authorization Server Metadata")
	oauthWS.Route(
		oauthWS.GET("/").To(func(_ *restful.Request, resp *restful.Response) {
			writeJSON(resp, metadata)
		}).
			Doc("get the server's OAuth 2.0 Authorization Server Metadata").
			Operation("getOAuthAuthorizationServerMetadata").
			Produces(restful.MIME_JSON))

	container.Add(oauthWS)
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

	deployConfigStorage, deployConfigStatusStorage, deployConfigScaleStorage, err := deployconfigetcd.NewREST(c.RESTOptionsGetter)

	dcInstantiateOriginClient, dcInstantiateKubeClient := c.DeploymentConfigInstantiateClients()
	dcInstantiateStorage := deployconfiginstantiate.NewREST(
		*deployConfigStorage.Store,
		dcInstantiateOriginClient,
		dcInstantiateKubeClient,
		c.ExternalVersionCodec,
		c.AdmissionControl,
	)

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
	subjectRulesReviewStorage := subjectrulesreview.NewREST(c.RuleResolver, c.Informers.ClusterPolicies().Lister().ClusterPolicies())

	roleStorage := rolestorage.NewVirtualStorage(policyRegistry, c.RuleResolver, nil, authorizationapi.Resource("role"))
	roleBindingStorage := rolebindingstorage.NewVirtualStorage(policyBindingRegistry, c.RuleResolver, nil, authorizationapi.Resource("rolebinding"))
	clusterRoleStorage := clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry, c.RuleResolver)
	clusterRoleBindingStorage := clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry, c.RuleResolver)

	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)
	localSubjectAccessReviewStorage := localsubjectaccessreview.NewREST(subjectAccessReviewRegistry)
	resourceAccessReviewStorage := resourceaccessreview.NewREST(c.Authorizer)
	resourceAccessReviewRegistry := resourceaccessreview.NewRegistry(resourceAccessReviewStorage)
	localResourceAccessReviewStorage := localresourceaccessreview.NewREST(resourceAccessReviewRegistry)

	podSecurityPolicyReviewStorage := podsecuritypolicyreview.NewREST(oscc.NewDefaultSCCMatcher(c.Informers.SecurityContextConstraints().Lister()), c.Informers.ServiceAccounts().Lister(), clientadapter.FromUnversionedClient(c.PrivilegedLoopbackKubernetesClient))
	podSecurityPolicySubjectStorage := podsecuritypolicysubjectreview.NewREST(oscc.NewDefaultSCCMatcher(c.Informers.SecurityContextConstraints().Lister()), clientadapter.FromUnversionedClient(c.PrivilegedLoopbackKubernetesClient))
	podSecurityPolicySelfSubjectReviewStorage := podsecuritypolicyselfsubjectreview.NewREST(oscc.NewDefaultSCCMatcher(c.Informers.SecurityContextConstraints().Lister()), clientadapter.FromUnversionedClient(c.PrivilegedLoopbackKubernetesClient))

	imageStorage, err := imageetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	imageRegistry := image.NewRegistry(imageStorage)
	imageSignatureStorage := imagesignature.NewREST(c.PrivilegedLoopbackOpenShiftClient.Images())
	imageStreamSecretsStorage := imagesecret.NewREST(c.ImageStreamSecretClient())
	imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage, err := imagestreametcd.NewREST(c.RESTOptionsGetter, c.RegistryNameFn, subjectAccessReviewRegistry, c.LimitVerifier)
	checkStorageErr(err)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry, c.RegistryNameFn)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	importerCache, err := imageimporter.NewImageStreamLayerCache(imageimporter.DefaultImageStreamLayerCacheSize)
	checkStorageErr(err)
	importerFn := func(r importer.RepositoryRetriever) imageimporter.Interface {
		return imageimporter.NewImageStreamImporter(r, c.Options.ImagePolicyConfig.MaxImagesBulkImportedPerRepository, flowcontrol.NewTokenBucketRateLimiter(2.0, 3), &importerCache)
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
	deployConfigRollbackStorage := deployrollback.NewREST(configClient, kclient, c.ExternalVersionCodec)

	projectStorage := projectproxy.NewREST(c.PrivilegedLoopbackKubernetesClient.Namespaces(), c.ProjectAuthorizationCache, c.ProjectAuthorizationCache, c.ProjectCache)

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

	osClient, kubeClient := c.OAuthServerClients()
	combinedOAuthClientGetter := saoauth.NewServiceAccountOAuthClientGetter(kubeClient, kubeClient, osClient, clientRegistry, saAccountGrantMethod)
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

		"deploymentConfigs":             deployConfigStorage,
		"deploymentConfigs/scale":       deployConfigScaleStorage,
		"deploymentConfigs/status":      deployConfigStatusStorage,
		"deploymentConfigs/rollback":    deployConfigRollbackStorage,
		"deploymentConfigs/log":         deploylogregistry.NewREST(configClient, kclient, c.DeploymentLogClient(), kubeletClient),
		"deploymentConfigs/instantiate": dcInstantiateStorage,

		// TODO: Deprecate these
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, c.ExternalVersionCodec),
		"deploymentConfigRollbacks": deployrollback.NewDeprecatedREST(deployRollbackClient, c.ExternalVersionCodec),

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
		"subjectRulesReviews":        subjectRulesReviewStorage,

		"podSecurityPolicyReviews":            podSecurityPolicyReviewStorage,
		"podSecurityPolicySubjectReviews":     podSecurityPolicySubjectStorage,
		"podSecurityPolicySelfSubjectReviews": podSecurityPolicySelfSubjectReviewStorage,

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
		storage["builds/clone"] = buildclone.NewStorage(buildGenerator)
		storage["builds/log"] = buildlogregistry.NewREST(buildStorage, buildStorage, c.BuildLogClient(), kubeletClient)
		storage["builds/details"] = buildDetailsStorage

		storage["buildConfigs"] = buildConfigStorage
		storage["buildConfigs/webhooks"] = buildConfigWebHooks
		storage["buildConfigs/instantiate"] = buildconfiginstantiate.NewStorage(buildGenerator)
		storage["buildConfigs/instantiatebinary"] = buildconfiginstantiate.NewBinaryStorage(buildGenerator, buildStorage, c.BuildLogClient(), kubeletClient)
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

// initMetricsRoute initializes an HTTP endpoint for metrics.
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

// getRequestInfoResolver returns a request resolver.
func (c *MasterConfig) getRequestInfoResolver() *apiserver.RequestInfoResolver {
	if c.RequestInfoResolver == nil {
		c.RequestInfoResolver = &apiserver.RequestInfoResolver{
			APIPrefixes: sets.NewString(strings.Trim(LegacyOpenShiftAPIPrefix, "/"),
				strings.Trim(OpenShiftAPIPrefix, "/"),
				strings.Trim(KubernetesAPIPrefix, "/"),
				strings.Trim(KubernetesAPIGroupPrefix, "/")), // all possible API prefixes
			GrouplessAPIPrefixes: sets.NewString(strings.Trim(LegacyOpenShiftAPIPrefix, "/"),
				strings.Trim(OpenShiftAPIPrefix, "/"),
				strings.Trim(KubernetesAPIPrefix, "/"),
			), // APIPrefixes that won't have groups (legacy)
		}
	}
	return c.RequestInfoResolver
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
