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
	server := apiserveroptions.NewAPIServer()
	if g.masterOptions.KubernetesMasterConfig != nil {
		if errs := cmdflags.Resolve(g.masterOptions.KubernetesMasterConfig.APIServerArguments, server.AddFlags); len(errs) > 0 {
			return kerrors.NewAggregate(errs)
		}
	}

	storageGroupsToEncodingVersion, err := server.StorageGroupsToEncodingVersion()
	if err != nil {
		return err
	}

	storageConfig := server.StorageConfig
	storageConfig.Prefix = g.masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix
	storageConfig.ServerList = g.masterOptions.EtcdClientInfo.URLs
	storageConfig.KeyFile = g.masterOptions.EtcdClientInfo.ClientCert.KeyFile
	storageConfig.CertFile = g.masterOptions.EtcdClientInfo.ClientCert.CertFile
	storageConfig.CAFile = g.masterOptions.EtcdClientInfo.CA

	storageFactory, err := genericapiserver.BuildDefaultStorageFactory(
		storageConfig, server.DefaultStorageMediaType, kapi.Codecs,
		genericapiserver.NewDefaultResourceEncodingConfig(), storageGroupsToEncodingVersion,
		nil,
		g.defaultResourceConfig, server.RuntimeConfig)
	if err != nil {
		return err
	}
	storageFactory.DefaultResourcePrefixes = g.defaultResourcePrefixes
	g.storageFactory = storageFactory

	g.cacheEnabled = server.EnableWatchCache

	errs := []error{}
	for _, c := range server.WatchCacheSizes {
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
