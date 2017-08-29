package origin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	kapi "k8s.io/kubernetes/pkg/api"
	v1beta1extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/client/retry"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	buildapiserver "github.com/openshift/origin/pkg/build/apiserver"
	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	oappsapiv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	oappsapiserver "github.com/openshift/origin/pkg/deploy/apiserver"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/security/legacyclient"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	templateapiserver "github.com/openshift/origin/pkg/template/apiserver"
	userapiserver "github.com/openshift/origin/pkg/user/apiserver"
	"github.com/openshift/origin/pkg/version"

	authzapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	projectapiv1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	quotaapiv1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	networkapiv1 "github.com/openshift/origin/pkg/sdn/apis/network/v1"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	userapiv1 "github.com/openshift/origin/pkg/user/apis/user/v1"
)

type OpenshiftAPIConfig struct {
	GenericConfig *genericapiserver.Config

	KubeClientExternal    kclientsetexternal.Interface
	KubeClientInternal    kclientsetinternal.Interface
	KubeletClientConfig   *kubeletclient.KubeletClientConfig
	KubeInternalInformers kinternalinformers.SharedInformerFactory

	AuthorizationInformers authorizationinformer.SharedInformerFactory
	QuotaInformers         quotainformer.SharedInformerFactory
	SecurityInformers      securityinformer.SharedInformerFactory

	// DeprecatedInformers is a shared factory for getting old style openshift informers
	DeprecatedOpenshiftClient *osclient.Client

	// these are all required to build our storage
	RuleResolver   rbacregistryvalidation.AuthorizationRuleResolver
	SubjectLocator authorizer.SubjectLocator
	LimitVerifier  imageadmission.LimitVerifier
	// RegistryNameFn retrieves the name of the integrated registry, or false if no such registry
	// is available.
	RegistryNameFn                     imageapi.DefaultRegistryFunc
	AllowedRegistriesForImport         *configapi.AllowedRegistries
	MaxImagesBulkImportedPerRepository int

	RouteAllocator *routeallocationcontroller.RouteAllocationController

	ProjectAuthorizationCache *projectauth.AuthorizationCache
	ProjectCache              *projectcache.ProjectCache
	ProjectRequestTemplate    string
	ProjectRequestMessage     string

	EnableBuilds bool

	ServiceAccountMethod configapi.GrantHandlerType

	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController

	// SCCStorage is actually created with a kubernetes restmapper options to have the correct prefix,
	// so we have to have it special cased here to point to the right spot.
	SCCStorage *sccstorage.REST
}

// Validate helps ensure that we build this config correctly, because there are lots of bits to remember for now
func (c *OpenshiftAPIConfig) Validate() error {
	ret := []error{}

	if c.KubeClientExternal == nil {
		ret = append(ret, fmt.Errorf("KubeClientExternal is required"))
	}
	if c.KubeClientInternal == nil {
		ret = append(ret, fmt.Errorf("KubeClientInternal is required"))
	}
	if c.KubeletClientConfig == nil {
		ret = append(ret, fmt.Errorf("KubeletClientConfig is required"))
	}
	if c.KubeInternalInformers == nil {
		ret = append(ret, fmt.Errorf("KubeInternalInformers is required"))
	}
	if c.AuthorizationInformers == nil {
		ret = append(ret, fmt.Errorf("AuthorizationInformers is required"))
	}
	if c.QuotaInformers == nil {
		ret = append(ret, fmt.Errorf("QuotaInformers is required"))
	}
	if c.SecurityInformers == nil {
		ret = append(ret, fmt.Errorf("SecurityInformers is required"))
	}
	if c.RuleResolver == nil {
		ret = append(ret, fmt.Errorf("RuleResolver is required"))
	}
	if c.SubjectLocator == nil {
		ret = append(ret, fmt.Errorf("SubjectLocator is required"))
	}
	if c.DeprecatedOpenshiftClient == nil {
		ret = append(ret, fmt.Errorf("DeprecatedOpenshiftClient is required"))
	}
	if c.LimitVerifier == nil {
		ret = append(ret, fmt.Errorf("LimitVerifier is required"))
	}
	if c.RegistryNameFn == nil {
		ret = append(ret, fmt.Errorf("RegistryNameFn is required"))
	}
	if c.RouteAllocator == nil {
		ret = append(ret, fmt.Errorf("RouteAllocator is required"))
	}
	if c.ProjectAuthorizationCache == nil {
		ret = append(ret, fmt.Errorf("ProjectAuthorizationCache is required"))
	}
	if c.ProjectCache == nil {
		ret = append(ret, fmt.Errorf("ProjectCache is required"))
	}
	if c.ClusterQuotaMappingController == nil {
		ret = append(ret, fmt.Errorf("ClusterQuotaMappingController is required"))
	}

	return utilerrors.NewAggregate(ret)
}

// OpenshiftAPIServer is only responsible for serving the APIs for Openshift
// It does NOT expose oauth, related oauth endpoints, or any kube APIs.
type OpenshiftAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*OpenshiftAPIConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *OpenshiftAPIConfig) Complete() completedConfig {
	c.GenericConfig.Complete()

	return completedConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *OpenshiftAPIConfig) SkipComplete() completedConfig {
	return completedConfig{c}
}

// legacyStorageMutator mutates the arg to modify the RESTStorage map for legacy resources
type legacyStorageMutator interface {
	mutate(map[schema.GroupVersion]map[string]rest.Storage)
}

type legacyStorageMutatorFunc func(map[schema.GroupVersion]map[string]rest.Storage)

func (l legacyStorageMutatorFunc) mutate(legacyStorage map[schema.GroupVersion]map[string]rest.Storage) {
	l(legacyStorage)
}

type legacyStorageMutators []legacyStorageMutator

func (l legacyStorageMutators) mutate(legacyStorage map[schema.GroupVersion]map[string]rest.Storage) {
	for _, curr := range l {
		curr.mutate(legacyStorage)
	}
}

// this allows the storage for a given apiserver to add itself to the old /oapi endpoint's storage
type legacyStorageVersionMutator struct {
	version schema.GroupVersion
	storage map[string]rest.Storage
}

func (l *legacyStorageVersionMutator) mutate(legacyStorage map[schema.GroupVersion]map[string]rest.Storage) {
	legacyStorage[l.version] = l.storage
}

func (c *completedConfig) withAppsAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &oappsapiserver.AppsConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		KubeletClientConfig:       c.KubeletClientConfig,
		Codecs:                    kapi.Codecs,
		Registry:                  kapi.Registry,
		Scheme:                    kapi.Scheme,
	}
	server, err := config.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, nil, err
	}
	storage, err := config.V1RESTStorage()
	if err != nil {
		return nil, nil, err
	}
	server.GenericAPIServer.PrepareRun() // this triggers openapi construction

	legacyDCRollbackMutator := oappsapiserver.LegacyLegacyDCRollbackMutator{
		CoreAPIServerClientConfig: config.CoreAPIServerClientConfig,
		Version:                   v1.SchemeGroupVersion,
	}
	return server.GenericAPIServer, legacyStorageMutators{legacyStorageMutatorFunc(legacyDCRollbackMutator.Mutate), &legacyStorageVersionMutator{version: oappsapiv1.SchemeGroupVersion, storage: storage}}, nil
}

func (c *completedConfig) withBuildAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	if !c.EnableBuilds {
		return delegateAPIServer, legacyStorageMutatorFunc(func(map[schema.GroupVersion]map[string]rest.Storage) {}), nil
	}

	config := &buildapiserver.BuildServerConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		KubeletClientConfig:       c.KubeletClientConfig,
		Codecs:                    kapi.Codecs,
		Registry:                  kapi.Registry,
		Scheme:                    kapi.Scheme,
	}
	server, err := config.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, nil, err
	}
	storage, err := config.V1RESTStorage()
	if err != nil {
		return nil, nil, err
	}
	server.GenericAPIServer.PrepareRun() // this triggers openapi construction

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: buildapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withTemplateAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &templateapiserver.TemplateConfig{
		GenericConfig:       c.GenericConfig,
		AuthorizationClient: c.KubeClientInternal.Authorization(),
		Codecs:              kapi.Codecs,
		Registry:            kapi.Registry,
		Scheme:              kapi.Scheme,
	}
	server, err := config.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, nil, err
	}
	storage, err := config.V1RESTStorage()
	if err != nil {
		return nil, nil, err
	}
	server.GenericAPIServer.PrepareRun() // this triggers openapi construction

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: templateapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withUserAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &userapiserver.UserConfig{
		GenericConfig: c.GenericConfig,
		Codecs:        kapi.Codecs,
		Registry:      kapi.Registry,
		Scheme:        kapi.Scheme,
	}
	server, err := config.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, nil, err
	}
	storage, err := config.V1RESTStorage()
	if err != nil {
		return nil, nil, err
	}
	server.GenericAPIServer.PrepareRun() // this triggers openapi construction

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: userapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*OpenshiftAPIServer, error) {
	legacyStorageModifier := legacyStorageMutators{}

	delegateAPIServer := delegationTarget
	var currLegacyStorageMutator legacyStorageMutator
	var err error

	delegateAPIServer, currLegacyStorageMutator, err = c.withAppsAPIServer(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	legacyStorageModifier = append(legacyStorageModifier, currLegacyStorageMutator)

	delegateAPIServer, currLegacyStorageMutator, err = c.withBuildAPIServer(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	legacyStorageModifier = append(legacyStorageModifier, currLegacyStorageMutator)

	delegateAPIServer, currLegacyStorageMutator, err = c.withTemplateAPIServer(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	legacyStorageModifier = append(legacyStorageModifier, currLegacyStorageMutator)

	delegateAPIServer, currLegacyStorageMutator, err = c.withUserAPIServer(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	legacyStorageModifier = append(legacyStorageModifier, currLegacyStorageMutator)

	genericServer, err := c.OpenshiftAPIConfig.GenericConfig.SkipComplete().New("openshift-apiserver", delegateAPIServer) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &OpenshiftAPIServer{
		GenericAPIServer: genericServer,
	}

	storage, err := c.GetRestStorage()
	if err != nil {
		return nil, err
	}
	groupVersions := map[string][]string{}

	// TODO restructure this to be more friendly
	// Install Origin API groups
	for gv := range storage {
		// skip pure-legacy groups as API groups
		if gv == v1.SchemeGroupVersion {
			continue
		}
		if !kapi.Registry.IsEnabledVersion(gv) {
			continue
		}
		for _, infos := range apiGroupsVersions {
			for _, group := range infos.Versions {
				groupVersions[group.Group] = append(groupVersions[group.Group], gv.Version)
			}
		}
	}

	for group, versions := range groupVersions {
		apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(group, kapi.Registry, kapi.Scheme, kapi.ParameterCodec, kapi.Codecs)

		for _, version := range versions {
			gv := schema.GroupVersion{Group: group, Version: version}
			apiGroupInfo.VersionedResourcesStorageMap[version] = storage[gv]

			// TODO all of our groups currently have one version, by the time we get more than one, these should be split up
			// into their own api servers
			if isPreferredGroupVersion(gv) {
				apiGroupInfo.GroupMeta.GroupVersion = gv
			}
		}

		if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
			return nil, fmt.Errorf("unable to initialize %s API group: %v", apiGroupInfo.GroupMeta.GroupVersion, err)
		}
		glog.Infof("Starting Origin API at %s/%s/%s", api.GroupPrefix, apiGroupInfo.GroupMeta.GroupVersion.Group, apiGroupInfo.GroupMeta.GroupVersion.Version)
	}

	// after the old-style groupified storage is created, modify the storage map to include the already migrated storage
	// to be included in the legacy group
	if _, ok := storage[v1.SchemeGroupVersion]; !ok {
		storage[v1.SchemeGroupVersion] = map[string]rest.Storage{}
	}
	legacyStorageModifier.mutate(storage)

	if err := s.GenericAPIServer.InstallLegacyAPIGroup(api.Prefix, apiLegacyV1(LegacyStorage(storage))); err != nil {
		return nil, fmt.Errorf("Unable to initialize v1 API: %v", err)
	}
	glog.Infof("Started Origin API at %s/%s", api.Prefix, v1.SchemeGroupVersion.Version)

	// fix API doc string
	for _, service := range s.GenericAPIServer.Handler.GoRestfulContainer.RegisteredWebServices() {
		if service.RootPath() == api.Prefix+"/"+v1.SchemeGroupVersion.Version {
			service.Doc("OpenShift REST API, version v1").ApiVersion("v1")
		}
	}

	// this remains a non-healthz endpoint so that you can be healthy without being ready.
	initReadinessCheckRoute(s.GenericAPIServer.Handler.NonGoRestfulMux, "/healthz/ready", c.ProjectAuthorizationCache.ReadyForAccess)

	// this remains here and separate so that you can check both kube and openshift levels
	initOpenshiftVersionRoute(s.GenericAPIServer.Handler.GoRestfulContainer, "/version/openshift")

	// register our poststarthooks
	s.GenericAPIServer.AddPostStartHookOrDie("quota.openshift.io-clusterquotamapping", c.startClusterQuotaMapping)
	s.GenericAPIServer.AddPostStartHookOrDie("project.openshift.io-projectcache", c.startProjectCache)
	s.GenericAPIServer.AddPostStartHookOrDie("project.openshift.io-projectauthorizationcache", c.startProjectAuthorizationCache)
	s.GenericAPIServer.AddPostStartHookOrDie("security.openshift.io-bootstrapscc", c.bootstrapSCC)
	s.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-ensureSARolesDefault", c.ensureDefaultNamespaceServiceAccountRoles)
	s.GenericAPIServer.AddPostStartHookOrDie("authorization.openshift.io-ensureopenshift-infra", c.ensureOpenShiftInfraNamespace)

	return s, nil
}

// apiLegacyV1 returns the resources and codec for API version v1.
func apiLegacyV1(all map[string]rest.Storage) *genericapiserver.APIGroupInfo {
	apiGroupInfo := &genericapiserver.APIGroupInfo{
		GroupMeta:                    *kapi.Registry.GroupOrDie(api.GroupName),
		VersionedResourcesStorageMap: map[string]map[string]rest.Storage{},
		Scheme: kapi.Scheme,
		// version.ParameterCodec = runtime.NewParameterCodec(kapi.Scheme)
		ParameterCodec:              kapi.ParameterCodec,
		NegotiatedSerializer:        kapi.Codecs,
		SubresourceGroupVersionKind: map[string]schema.GroupVersionKind{},
	}

	// TODO, just create this with lowercase names
	storage := make(map[string]rest.Storage)
	for k, v := range all {
		storage[strings.ToLower(k)] = v
	}
	apiGroupInfo.VersionedResourcesStorageMap["v1"] = storage
	apiGroupInfo.SubresourceGroupVersionKind["deploymentconfigs/scale"] = v1beta1extensions.SchemeGroupVersion.WithKind("Scale")
	return apiGroupInfo
}

// initReadinessCheckRoute initializes an HTTP endpoint for readiness checking
func initReadinessCheckRoute(mux *genericmux.PathRecorderMux, path string, readyFunc func() bool) {
	mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		if readyFunc() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))

		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
}

// initVersionRoute initializes an HTTP endpoint for the server's version information.
func initOpenshiftVersionRoute(container *restful.Container, path string) {
	// Build version info once
	versionInfo, err := json.MarshalIndent(version.Get(), "", "  ")
	if err != nil {
		glog.Errorf("Unable to initialize version route: %v", err)
		return
	}

	// Set up a service to return the git code version.
	ws := new(restful.WebService)
	ws.Path(path)
	ws.Doc("git code version from which this is built")
	ws.Route(
		ws.GET("/").To(func(_ *restful.Request, resp *restful.Response) {
			writeJSON(resp, versionInfo)
		}).
			Doc("get the code version").
			Operation("getCodeVersion").
			Produces(restful.MIME_JSON))

	container.Add(ws)
}

func writeJSON(resp *restful.Response, json []byte) {
	resp.ResponseWriter.Header().Set("Content-Type", "application/json")
	resp.ResponseWriter.WriteHeader(http.StatusOK)
	resp.ResponseWriter.Write(json)
}

// apiGroupInfo represents a set of API group versions and their preferred version.
type apiGroupInfo struct {
	PreferredVersion string
	Versions         []schema.GroupVersion
}

// apiGroupsVersions holds the list of installed Origin API groups and their preferred version.
// FIXME: This should be handled in each REST storage separately and on in one place. That
//        will be addressed as a separate issue.
var apiGroupsVersions = []apiGroupInfo{
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{securityapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{projectapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{quotaapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{networkapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{routeapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{imageapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{authzapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{oauthapiv1.SchemeGroupVersion}},
}

// isPreferredGroupVersion returns true if the given GroupVersion is preferred version in
// the API group.
func isPreferredGroupVersion(gv schema.GroupVersion) bool {
	for _, info := range apiGroupsVersions {
		for _, version := range info.Versions {
			if version == gv && gv.Version == info.PreferredVersion {
				return true
			}
		}
	}
	return false
}

func (c *OpenshiftAPIConfig) startClusterQuotaMapping(context genericapiserver.PostStartHookContext) error {
	go c.ClusterQuotaMappingController.Run(5, context.StopCh)
	return nil
}

func (c *OpenshiftAPIConfig) startProjectCache(context genericapiserver.PostStartHookContext) error {
	// RunProjectCache populates project cache, used by scheduler and project admission controller.
	glog.Infof("Using default project node label selector: %s", c.ProjectCache.DefaultNodeSelector)
	go c.ProjectCache.Run(context.StopCh)
	return nil
}

func (c *OpenshiftAPIConfig) startProjectAuthorizationCache(context genericapiserver.PostStartHookContext) error {
	period := 1 * time.Second
	c.ProjectAuthorizationCache.Run(period)
	return nil
}

func (c *OpenshiftAPIConfig) bootstrapSCC(context genericapiserver.PostStartHookContext) error {
	ns := bootstrappolicy.DefaultOpenShiftInfraNamespace
	bootstrapSCCGroups, bootstrapSCCUsers := bootstrappolicy.GetBoostrapSCCAccess(ns)

	for _, scc := range bootstrappolicy.GetBootstrapSecurityContextConstraints(bootstrapSCCGroups, bootstrapSCCUsers) {
		_, err := legacyclient.NewFromClient(c.KubeClientInternal.Core().RESTClient()).Create(scc)
		if kapierror.IsAlreadyExists(err) {
			continue
		}
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create default security context constraint %s.  Got error: %v", scc.Name, err))
			continue
		}
		glog.Infof("Created default security context constraint %s", scc.Name)
	}
	return nil
}

// ensureOpenShiftInfraNamespace is called as part of global policy initialization to ensure infra namespace exists
func (c *OpenshiftAPIConfig) ensureOpenShiftInfraNamespace(context genericapiserver.PostStartHookContext) error {
	ns := bootstrappolicy.DefaultOpenShiftInfraNamespace

	// Ensure namespace exists
	namespace, err := c.KubeClientInternal.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	if kapierror.IsAlreadyExists(err) {
		// Get the persisted namespace
		namespace, err = c.KubeClientInternal.Core().Namespaces().Get(ns, metav1.GetOptions{})
		if err != nil {
			glog.Errorf("Error getting namespace %s: %v", ns, err)
			return nil
		}
	} else if err != nil {
		glog.Errorf("Error creating namespace %s: %v", ns, err)
		return nil
	}

	// Ensure we have the bootstrap SA for Nodes
	_, err = c.KubeClientInternal.Core().ServiceAccounts(ns).Create(&kapi.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: bootstrappolicy.InfraNodeBootstrapServiceAccountName}})
	if err != nil && !kapierror.IsAlreadyExists(err) {
		glog.Errorf("Error creating service account %s/%s: %v", namespace, bootstrappolicy.InfraNodeBootstrapServiceAccountName, err)
	}

	EnsureNamespaceServiceAccountRoleBindings(c.KubeClientInternal, c.DeprecatedOpenshiftClient, namespace)
	return nil
}

// ensureDefaultNamespaceServiceAccountRoles initializes roles for service accounts in the default namespace
func (c *OpenshiftAPIConfig) ensureDefaultNamespaceServiceAccountRoles(context genericapiserver.PostStartHookContext) error {
	// Wait for the default namespace
	var namespace *kapi.Namespace
	for i := 0; i < 30; i++ {
		ns, err := c.KubeClientInternal.Core().Namespaces().Get(metav1.NamespaceDefault, metav1.GetOptions{})
		if err == nil {
			namespace = ns
			break
		}
		if kapierror.IsNotFound(err) {
			time.Sleep(time.Second)
			continue
		}
		glog.Errorf("Error adding service account roles to %q namespace: %v", metav1.NamespaceDefault, err)
		return nil
	}
	if namespace == nil {
		glog.Errorf("Namespace %q not found, could not initialize the %q namespace", metav1.NamespaceDefault, metav1.NamespaceDefault)
		return nil
	}

	EnsureNamespaceServiceAccountRoleBindings(c.KubeClientInternal, c.DeprecatedOpenshiftClient, namespace)
	return nil
}

// EnsureNamespaceServiceAccountRoleBindings initializes roles for service accounts in the namespace
func EnsureNamespaceServiceAccountRoleBindings(kubeClientInternal kclientsetinternal.Interface, deprecatedOpenshiftClient *osclient.Client, namespace *kapi.Namespace) {
	const ServiceAccountRolesInitializedAnnotation = "openshift.io/sa.initialized-roles"

	// Short-circuit if we're already initialized
	if namespace.Annotations[ServiceAccountRolesInitializedAnnotation] == "true" {
		return
	}

	hasErrors := false
	for _, binding := range bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(namespace.Name) {
		addRole := &policy.RoleModificationOptions{
			RoleName:            binding.RoleRef.Name,
			RoleNamespace:       binding.RoleRef.Namespace,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(namespace.Name, deprecatedOpenshiftClient),
			Subjects:            binding.Subjects,
		}
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error { return addRole.AddRole() }); err != nil {
			glog.Errorf("Could not add service accounts to the %v role in the %q namespace: %v\n", binding.RoleRef.Name, namespace.Name, err)
			hasErrors = true
		}
	}

	// If we had errors, don't register initialization so we can try again
	if hasErrors {
		return
	}

	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	namespace.Annotations[ServiceAccountRolesInitializedAnnotation] = "true"
	// Log any error other than a conflict (the update will be retried and recorded again on next startup in that case)
	if _, err := kubeClientInternal.Core().Namespaces().Update(namespace); err != nil && !kapierror.IsConflict(err) {
		glog.Errorf("Error recording adding service account roles to %q namespace: %v", namespace.Name, err)
	}
}
