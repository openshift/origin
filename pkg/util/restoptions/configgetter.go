package restoptions

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/genericapiserver"
	genericrest "k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/storage/storagebackend"
	"k8s.io/kubernetes/pkg/storage/storagebackend/factory"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/golang/glog"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

// UseConfiguredCacheSize indicates that the configured cache size should be used
const UseConfiguredCacheSize = -1

// configRESTOptionsGetter provides RESTOptions based on a provided config
type configRESTOptionsGetter struct {
	masterOptions configapi.MasterConfig

	restOptionsLock sync.Mutex
	restOptionsMap  map[unversioned.GroupResource]genericrest.RESTOptions

	storageFactory        genericapiserver.StorageFactory
	defaultResourceConfig *genericapiserver.ResourceConfig

	cacheEnabled            bool
	defaultCacheSize        int
	cacheSizes              map[unversioned.GroupResource]int
	quorumResources         map[unversioned.GroupResource]struct{}
	defaultResourcePrefixes map[unversioned.GroupResource]string
}

// NewConfigGetter returns a restoptions.Getter implemented using information from the provided master config.
// By default, the etcd watch cache is enabled with a size of 1000 per resource type.
// TODO: this class should either not need to know about configapi.MasterConfig, or not be in pkg/util
func NewConfigGetter(masterOptions configapi.MasterConfig, defaultResourceConfig *genericapiserver.ResourceConfig, defaultResourcePrefixes map[unversioned.GroupResource]string, quorumResources map[unversioned.GroupResource]struct{}) Getter {
	getter := &configRESTOptionsGetter{
		masterOptions:           masterOptions,
		cacheEnabled:            true,
		defaultCacheSize:        1000,
		cacheSizes:              map[unversioned.GroupResource]int{},
		restOptionsMap:          map[unversioned.GroupResource]genericrest.RESTOptions{},
		defaultResourceConfig:   defaultResourceConfig,
		quorumResources:         quorumResources,
		defaultResourcePrefixes: defaultResourcePrefixes,
	}

	if err := getter.loadSettings(); err != nil {
		glog.Error(err)
	}

	return getter
}

func (g *configRESTOptionsGetter) loadSettings() error {
	options := apiserveroptions.NewServerRunOptions()
	if g.masterOptions.KubernetesMasterConfig != nil {
		if errs := cmdflags.Resolve(g.masterOptions.KubernetesMasterConfig.APIServerArguments, options.AddFlags); len(errs) > 0 {
			return kerrors.NewAggregate(errs)
		}
	}

	storageGroupsToEncodingVersion, err := options.GenericServerRunOptions.StorageGroupsToEncodingVersion()
	if err != nil {
		return err
	}

	storageConfig := options.GenericServerRunOptions.StorageConfig
	storageConfig.Prefix = g.masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix
	storageConfig.ServerList = g.masterOptions.EtcdClientInfo.URLs
	storageConfig.KeyFile = g.masterOptions.EtcdClientInfo.ClientCert.KeyFile
	storageConfig.CertFile = g.masterOptions.EtcdClientInfo.ClientCert.CertFile
	storageConfig.CAFile = g.masterOptions.EtcdClientInfo.CA

	resourceEncodingConfig := genericapiserver.NewDefaultResourceEncodingConfig()

	storageFactory, err := genericapiserver.BuildDefaultStorageFactory(
		storageConfig,
		options.GenericServerRunOptions.DefaultStorageMediaType,
		kapi.Codecs,
		resourceEncodingConfig,
		storageGroupsToEncodingVersion,
		nil,
		g.defaultResourceConfig,
		options.GenericServerRunOptions.RuntimeConfig)
	if err != nil {
		return err
	}

	// TODO: the following works, but better make single resource overrides possible in BuildDefaultStorageFactory
	// instead of being late to the party and patching here:

	// use legacy group name "" for all resources that existed when apigroups were introduced
	for _, gvr := range []unversioned.GroupVersionResource{
		{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterpolicybindings"},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterpolicies"},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "policybindings"},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "rolebindingrestrictions"},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "policies"},
		{Group: "build.openshift.io", Version: "v1", Resource: "builds"},
		{Group: "build.openshift.io", Version: "v1", Resource: "buildconfigs"},
		{Group: "apps.openshift.io", Version: "v1", Resource: "deploymentconfigs"},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams"},
		{Group: "image.openshift.io", Version: "v1", Resource: "images"},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthclientauthorizations"},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthaccesstokens"},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthauthorizetokens"},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthclients"},
		{Group: "project.openshift.io", Version: "v1", Resource: "projects"},
		{Group: "quota.openshift.io", Version: "v1", Resource: "clusterresourcequotas"},
		{Group: "route.openshift.io", Version: "v1", Resource: "routes"},
		{Group: "network.openshift.io", Version: "v1", Resource: "netnamespaces"},
		{Group: "network.openshift.io", Version: "v1", Resource: "hostsubnets"},
		{Group: "network.openshift.io", Version: "v1", Resource: "clusternetworks"},
		{Group: "network.openshift.io", Version: "v1", Resource: "egressnetworkpolicies"},
		{Group: "template.openshift.io", Version: "v1", Resource: "templates"},
		{Group: "user.openshift.io", Version: "v1", Resource: "groups"},
		{Group: "user.openshift.io", Version: "v1", Resource: "users"},
		{Group: "user.openshift.io", Version: "v1", Resource: "identities"},
	} {
		resourceEncodingConfig.SetResourceEncoding(gvr.GroupResource(), unversioned.GroupVersion{Version: gvr.Version}, unversioned.GroupVersion{Version: runtime.APIVersionInternal})
	}

	storageFactory.DefaultResourcePrefixes = g.defaultResourcePrefixes
	g.storageFactory = storageFactory

	g.cacheEnabled = options.GenericServerRunOptions.EnableWatchCache

	errs := []error{}
	for _, c := range options.GenericServerRunOptions.WatchCacheSizes {
		tokens := strings.Split(c, "#")
		if len(tokens) != 2 {
			errs = append(errs, fmt.Errorf("invalid watch cache size value '%s', expecting <resource>#<size> format (e.g. builds#100)", c))
			continue
		}

		resource := unversioned.ParseGroupResource(tokens[0])

		size, err := strconv.Atoi(tokens[1])
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid watch cache size value '%s': %v", c, err))
			continue
		}

		g.cacheSizes[resource] = size
	}
	return kerrors.NewAggregate(errs)
}

func (g *configRESTOptionsGetter) GetRESTOptions(resource unversioned.GroupResource) (genericrest.RESTOptions, error) {
	g.restOptionsLock.Lock()
	defer g.restOptionsLock.Unlock()
	if resourceOptions, ok := g.restOptionsMap[resource]; ok {
		return resourceOptions, nil
	}

	config, err := g.storageFactory.NewConfig(resource)
	if err != nil {
		return genericrest.RESTOptions{}, err
	}

	if _, ok := g.quorumResources[resource]; ok {
		config.Quorum = true
	}

	configuredCacheSize, specified := g.cacheSizes[resource]
	if !specified || configuredCacheSize < 0 {
		configuredCacheSize = g.defaultCacheSize
	}

	decorator := func(s *storagebackend.Config, requestedSize int, objectType runtime.Object, resourcePrefix string, scopeStrategy rest.NamespaceScopedStrategy, newListFn func() runtime.Object, triggerFn storage.TriggerPublisherFunc) (storage.Interface, factory.DestroyFunc) {
		capacity := requestedSize
		if capacity == UseConfiguredCacheSize {
			capacity = configuredCacheSize
		}

		if capacity == 0 || !g.cacheEnabled {
			glog.V(5).Infof("using uncached watch storage for %s", resource.String())
			return genericrest.UndecoratedStorage(s, capacity, objectType, resourcePrefix, scopeStrategy, newListFn, triggerFn)
		}

		glog.V(5).Infof("using watch cache storage (capacity=%d) for %s %#v", capacity, resource.String(), s)
		return registry.StorageWithCacher(s, capacity, objectType, resourcePrefix, scopeStrategy, newListFn, triggerFn)
	}

	resourceOptions := genericrest.RESTOptions{
		StorageConfig:           config,
		Decorator:               decorator,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          g.storageFactory.ResourcePrefix(resource),
	}
	g.restOptionsMap[resource] = resourceOptions

	return resourceOptions, nil
}
