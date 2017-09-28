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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	kapi "k8s.io/kubernetes/pkg/api"
	v1beta1extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/v1"
	oappsapiv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	oappsapiserver "github.com/openshift/origin/pkg/apps/apiserver"
	authorizationapiserver "github.com/openshift/origin/pkg/authorization/apiserver"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	buildapiserver "github.com/openshift/origin/pkg/build/apiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiserver "github.com/openshift/origin/pkg/image/apiserver"
	networkapiserver "github.com/openshift/origin/pkg/network/apiserver"
	oauthapiserver "github.com/openshift/origin/pkg/oauth/apiserver"
	projectapiserver "github.com/openshift/origin/pkg/project/apiserver"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	quotaapiserver "github.com/openshift/origin/pkg/quota/apiserver"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	routeapiserver "github.com/openshift/origin/pkg/route/apiserver"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	securityapiserver "github.com/openshift/origin/pkg/security/apiserver"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	templateapiserver "github.com/openshift/origin/pkg/template/apiserver"
	userapiserver "github.com/openshift/origin/pkg/user/apiserver"
	"github.com/openshift/origin/pkg/version"

	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	networkapiv1 "github.com/openshift/origin/pkg/network/apis/network/v1"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	projectapiv1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	quotaapiv1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	userapiv1 "github.com/openshift/origin/pkg/user/apis/user/v1"

	// register api groups
	_ "github.com/openshift/origin/pkg/api/install"
)

type OpenshiftAPIConfig struct {
	GenericConfig *genericapiserver.Config

	KubeClientInternal    kclientsetinternal.Interface
	KubeletClientConfig   *kubeletclient.KubeletClientConfig
	KubeInternalInformers kinternalinformers.SharedInformerFactory

	QuotaInformers    quotainformer.SharedInformerFactory
	SecurityInformers securityinformer.SharedInformerFactory

	// these are all required to build our storage
	RuleResolver   rbacregistryvalidation.AuthorizationRuleResolver
	SubjectLocator authorizer.SubjectLocator

	// for Images
	LimitVerifier imageadmission.LimitVerifier
	// RegistryHostnameRetriever retrieves the internal and external hostname of
	// the integrated registry, or false if no such registry is available.
	RegistryHostnameRetriever          imageapi.RegistryHostnameRetriever
	AllowedRegistriesForImport         *configapi.AllowedRegistries
	MaxImagesBulkImportedPerRepository int

	RouteAllocator *routeallocationcontroller.RouteAllocationController

	ProjectAuthorizationCache *projectauth.AuthorizationCache
	ProjectCache              *projectcache.ProjectCache
	ProjectRequestTemplate    string
	ProjectRequestMessage     string

	EnableBuilds bool

	// oauth API server
	ServiceAccountMethod configapi.GrantHandlerType

	ClusterQuotaMappingController *clusterquotamapping.ClusterQuotaMappingController

	// SCCStorage is actually created with a kubernetes restmapper options to have the correct prefix,
	// so we have to have it special cased here to point to the right spot.
	SCCStorage *sccstorage.REST
}

// Validate helps ensure that we build this config correctly, because there are lots of bits to remember for now
func (c *OpenshiftAPIConfig) Validate() error {
	ret := []error{}

	if c.KubeClientInternal == nil {
		ret = append(ret, fmt.Errorf("KubeClientInternal is required"))
	}
	if c.KubeletClientConfig == nil {
		ret = append(ret, fmt.Errorf("KubeletClientConfig is required"))
	}
	if c.KubeInternalInformers == nil {
		ret = append(ret, fmt.Errorf("KubeInternalInformers is required"))
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
	if c.LimitVerifier == nil {
		ret = append(ret, fmt.Errorf("LimitVerifier is required"))
	}
	if c.RegistryHostnameRetriever == nil {
		ret = append(ret, fmt.Errorf("RegistryHostnameRetriever is required"))
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

func (c *completedConfig) withAuthorizationAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &authorizationapiserver.AuthorizationAPIServerConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		KubeInternalInformers:     c.KubeInternalInformers,
		RuleResolver:              c.RuleResolver,
		SubjectLocator:            c.SubjectLocator,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: authorizationapiv1.SchemeGroupVersion, storage: storage}, nil
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

func (c *completedConfig) withImageAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &imageapiserver.ImageAPIServerConfig{
		GenericConfig:                      c.GenericConfig,
		CoreAPIServerClientConfig:          c.GenericConfig.LoopbackClientConfig,
		LimitVerifier:                      c.LimitVerifier,
		RegistryHostnameRetriever:          c.RegistryHostnameRetriever,
		AllowedRegistriesForImport:         c.AllowedRegistriesForImport,
		MaxImagesBulkImportedPerRepository: c.MaxImagesBulkImportedPerRepository,
		Codecs:   kapi.Codecs,
		Registry: kapi.Registry,
		Scheme:   kapi.Scheme,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: imageapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withNetworkAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &networkapiserver.NetworkAPIServerConfig{
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: networkapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withOAuthAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &oauthapiserver.OAuthAPIServerConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		ServiceAccountMethod:      c.ServiceAccountMethod,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: oauthapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withProjectAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &projectapiserver.ProjectAPIServerConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		KubeInternalInformers:     c.KubeInternalInformers,
		ProjectAuthorizationCache: c.ProjectAuthorizationCache,
		ProjectCache:              c.ProjectCache,
		ProjectRequestTemplate:    c.ProjectRequestTemplate,
		ProjectRequestMessage:     c.ProjectRequestMessage,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: projectapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withQuotaAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &quotaapiserver.QuotaAPIServerConfig{
		GenericConfig:                 c.GenericConfig,
		ClusterQuotaMappingController: c.ClusterQuotaMappingController,
		QuotaInformers:                c.QuotaInformers,
		KubeInternalInformers:         c.KubeInternalInformers,
		Codecs:                        kapi.Codecs,
		Registry:                      kapi.Registry,
		Scheme:                        kapi.Scheme,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: quotaapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withRouteAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &routeapiserver.RouteAPIServerConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		RouteAllocator:            c.RouteAllocator,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: routeapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withSecurityAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &securityapiserver.SecurityAPIServerConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		// SCCStorage is actually created with a kubernetes restmapper options to have the correct prefix,
		// so we have to have it special cased here to point to the right spot.
		SCCStorage:            c.SCCStorage,
		SecurityInformers:     c.SecurityInformers,
		KubeInternalInformers: c.KubeInternalInformers,
		Codecs:                kapi.Codecs,
		Registry:              kapi.Registry,
		Scheme:                kapi.Scheme,
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

	return server.GenericAPIServer, &legacyStorageVersionMutator{version: securityapiv1.SchemeGroupVersion, storage: storage}, nil
}

func (c *completedConfig) withTemplateAPIServer(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error) {
	config := &templateapiserver.TemplateConfig{
		GenericConfig:             c.GenericConfig,
		CoreAPIServerClientConfig: c.GenericConfig.LoopbackClientConfig,
		Codecs:   kapi.Codecs,
		Registry: kapi.Registry,
		Scheme:   kapi.Scheme,
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

type apiServerAppenderFunc func(delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, legacyStorageMutator, error)

func addAPIServerOrDie(delegateAPIServer genericapiserver.DelegationTarget, legacyStorageModifiers legacyStorageMutators, apiServerAppenderFn apiServerAppenderFunc) (genericapiserver.DelegationTarget, legacyStorageMutators) {
	delegateAPIServer, currLegacyStorageMutator, err := apiServerAppenderFn(delegateAPIServer)
	if err != nil {
		glog.Fatal(err)
	}
	legacyStorageModifiers = append(legacyStorageModifiers, currLegacyStorageMutator)

	return delegateAPIServer, legacyStorageModifiers
}

func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*OpenshiftAPIServer, error) {
	delegateAPIServer := delegationTarget
	legacyStorageModifier := legacyStorageMutators{}

	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withAppsAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withAuthorizationAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withBuildAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withImageAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withNetworkAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withOAuthAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withProjectAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withQuotaAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withRouteAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withSecurityAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withTemplateAPIServer)
	delegateAPIServer, legacyStorageModifier = addAPIServerOrDie(delegateAPIServer, legacyStorageModifier, c.withUserAPIServer)

	genericServer, err := c.OpenshiftAPIConfig.GenericConfig.SkipComplete().New("openshift-apiserver", delegateAPIServer) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &OpenshiftAPIServer{
		GenericAPIServer: genericServer,
	}

	legacyStorage := map[schema.GroupVersion]map[string]rest.Storage{
		v1.SchemeGroupVersion: {},
	}
	legacyStorageModifier.mutate(legacyStorage)

	if err := s.GenericAPIServer.InstallLegacyAPIGroup(api.Prefix, apiLegacyV1(LegacyStorage(legacyStorage))); err != nil {
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

	var securityClient securityclient.Interface
	err := wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		var err error
		securityClient, err = securityclient.NewForConfig(context.LoopbackClientConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to initialize client: %v", err))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error getting client: %v", err))
		return err
	}

	for _, scc := range bootstrappolicy.GetBootstrapSecurityContextConstraints(bootstrapSCCGroups, bootstrapSCCUsers) {
		_, err := securityClient.Security().SecurityContextConstraints().Create(scc)
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

	ensureNamespaceServiceAccountRoleBindings(context, ns)

	var coreClient coreclient.CoreInterface
	err := wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		var err error
		coreClient, err = coreclient.NewForConfig(context.LoopbackClientConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to initialize client: %v", err))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error getting client: %v", err))
		return err
	}

	// Ensure we have the bootstrap SA for Nodes
	_, err = coreClient.ServiceAccounts(ns).Create(&kapi.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: bootstrappolicy.InfraNodeBootstrapServiceAccountName}})
	if err != nil && !kapierror.IsAlreadyExists(err) {
		glog.Errorf("Error creating service account %s/%s: %v", ns, bootstrappolicy.InfraNodeBootstrapServiceAccountName, err)
	}

	return nil
}

// ensureDefaultNamespaceServiceAccountRoles initializes roles for service accounts in the default namespace
func (c *OpenshiftAPIConfig) ensureDefaultNamespaceServiceAccountRoles(context genericapiserver.PostStartHookContext) error {
	ensureNamespaceServiceAccountRoleBindings(context, metav1.NamespaceDefault)
	return nil
}

// ensureNamespaceServiceAccountRoleBindings initializes roles for service accounts in the namespace
func ensureNamespaceServiceAccountRoleBindings(context genericapiserver.PostStartHookContext, namespaceName string) {
	const ServiceAccountRolesInitializedAnnotation = "openshift.io/sa.initialized-roles"

	var coreClient coreclient.CoreInterface
	err := wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		var err error
		coreClient, err = coreclient.NewForConfig(context.LoopbackClientConfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to initialize client: %v", err))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error getting client: %v", err))
		return
	}

	// Ensure namespace exists
	namespace, err := coreClient.Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})
	if kapierror.IsAlreadyExists(err) {
		// Get the persisted namespace
		namespace, err = coreClient.Namespaces().Get(namespaceName, metav1.GetOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error getting namespace %s: %v", namespaceName, err))
			return
		}
	} else if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error creating namespace %s: %v", namespaceName, err))
		return
	}

	// Short-circuit if we're already initialized
	if namespace.Annotations[ServiceAccountRolesInitializedAnnotation] == "true" {
		return
	}

	policyData := &rbacrest.PolicyData{
		RoleBindings: map[string][]rbac.RoleBinding{
			namespace.Name: bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(namespace.Name),
		},
	}
	if err := policyData.EnsureRBACPolicy()(context); err != nil {
		utilruntime.HandleError(err)
		return
	}

	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	namespace.Annotations[ServiceAccountRolesInitializedAnnotation] = "true"
	// Log any error other than a conflict (the update will be retried and recorded again on next startup in that case)
	if _, err := coreClient.Namespaces().Update(namespace); err != nil && !kapierror.IsConflict(err) {
		utilruntime.HandleError(fmt.Errorf("Error recording adding service account roles to %q namespace: %v", namespace.Name, err))
		return
	}

}
