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

	cacheEnabled     bool
	defaultCacheSize int
	cacheSizes       map[unversioned.GroupResource]int
}

// NewConfigGetter returns a restoptions.Getter implemented using information from the provided master config.
// By default, the etcd watch cache is enabled with a size of 1000 per resource type.
func NewConfigGetter(masterOptions configapi.MasterConfig, defaultResourceConfig *genericapiserver.ResourceConfig) Getter {
	getter := &configRESTOptionsGetter{
		masterOptions:         masterOptions,
		cacheEnabled:          true,
		defaultCacheSize:      1000,
		cacheSizes:            map[unversioned.GroupResource]int{},
		restOptionsMap:        map[unversioned.GroupResource]genericrest.RESTOptions{},
		defaultResourceConfig: defaultResourceConfig,
	}

	if err := getter.loadSettings(); err != nil {
		glog.Error(err)
	}

	return getter
}

func (g *configRESTOptionsGetter) loadSettings() error {
	if g.masterOptions.KubernetesMasterConfig == nil {
		return nil
	}

	server := apiserveroptions.NewAPIServer()
	if errs := cmdflags.Resolve(g.masterOptions.KubernetesMasterConfig.APIServerArguments, server.AddFlags); len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}

	storageGroupsToEncodingVersion, err := server.StorageGroupsToEncodingVersion()
	if err != nil {
		return err
	}

	storageFactory, err := genericapiserver.BuildDefaultStorageFactory(
		server.StorageConfig, server.DefaultStorageMediaType, kapi.Codecs,
		genericapiserver.NewDefaultResourceEncodingConfig(), storageGroupsToEncodingVersion,
		nil,
		g.defaultResourceConfig, server.RuntimeConfig)
	if err != nil {
		return err
	}
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

	// Override OpenShift specific settings here
	// config.Prefix = g.masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix
	config.ServerList = g.masterOptions.EtcdClientInfo.URLs
	config.KeyFile = g.masterOptions.EtcdClientInfo.ClientCert.KeyFile
	config.CertFile = g.masterOptions.EtcdClientInfo.ClientCert.CertFile
	config.CAFile = g.masterOptions.EtcdClientInfo.CA

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

		glog.V(5).Infof("using watch cache storage (capacity=%d) for %s", capacity, resource.String())
		return registry.StorageWithCacher(s, capacity, objectType, resourcePrefix, scopeStrategy, newListFn, triggerFn)
	}

	resourceOptions := genericrest.RESTOptions{
		StorageConfig:           config,
		Decorator:               decorator,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          g.masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix,
	}
	g.restOptionsMap[resource] = resourceOptions

	return resourceOptions, nil
}
