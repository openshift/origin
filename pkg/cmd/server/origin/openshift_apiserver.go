package origin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	kapi "k8s.io/kubernetes/pkg/api"
	v1beta1extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	authorizationinformer "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
	"github.com/openshift/origin/pkg/version"

	authzapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
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
	RuleResolver   rulevalidation.AuthorizationRuleResolver
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

	EnableTemplateServiceBroker bool

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

func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget, stopCh <-chan struct{}) (*OpenshiftAPIServer, error) {
	genericServer, err := c.OpenshiftAPIConfig.GenericConfig.SkipComplete().New("openshift-apiserver", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &OpenshiftAPIServer{
		GenericAPIServer: genericServer,
	}

	if err := installAPIs(c.OpenshiftAPIConfig, genericServer); err != nil {
		return nil, err
	}

	return s, nil
}

func installAPIs(c *OpenshiftAPIConfig, server *genericapiserver.GenericAPIServer) error {
	storage, err := c.GetRestStorage()
	if err != nil {
		return err
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

		if err := server.InstallAPIGroup(&apiGroupInfo); err != nil {
			return fmt.Errorf("unable to initialize %s API group: %v", apiGroupInfo.GroupMeta.GroupVersion, err)
		}
		glog.Infof("Starting Origin API at %s/%s/%s", api.GroupPrefix, apiGroupInfo.GroupMeta.GroupVersion.Group, apiGroupInfo.GroupMeta.GroupVersion.Version)
	}

	if err := server.InstallLegacyAPIGroup(api.Prefix, apiLegacyV1(LegacyStorage(storage))); err != nil {
		return fmt.Errorf("Unable to initialize v1 API: %v", err)
	}
	glog.Infof("Started Origin API at %s/%s", api.Prefix, v1.SchemeGroupVersion.Version)

	// fix API doc string
	for _, service := range server.Handler.GoRestfulContainer.RegisteredWebServices() {
		if service.RootPath() == api.Prefix+"/"+v1.SchemeGroupVersion.Version {
			service.Doc("OpenShift REST API, version v1").ApiVersion("v1")
		}
	}

	// this remains a non-healthz endpoint so that you can be healthy without being ready.
	initReadinessCheckRoute(server.Handler.NonGoRestfulMux, "/healthz/ready", c.ProjectAuthorizationCache.ReadyForAccess)

	// this remains here and separate so that you can check both kube and openshift levels
	initOpenshiftVersionRoute(server.Handler.GoRestfulContainer, "/version/openshift")

	return nil
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
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{buildapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{quotaapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{networkapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{routeapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{userapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{imageapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{deployapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{authzapiv1.SchemeGroupVersion}},
	{PreferredVersion: "v1", Versions: []schema.GroupVersion{templateapiv1.SchemeGroupVersion}},
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
