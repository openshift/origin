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
	genericserveroptions "k8s.io/kubernetes/pkg/genericapiserver/options"
	genericrest "k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/golang/glog"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

// UseConfiguredCacheSize indicates that the configured cache size should be used
const UseConfiguredCacheSize = -1

// configRESTOptionsGetter provides RESTOptions based on a provided config
type configRESTOptionsGetter struct {
	masterOptions configapi.MasterConfig

	restOptionsLock sync.Mutex
	restOptionsMap  map[unversioned.GroupResource]genericrest.RESTOptions

	etcdHelper storage.Interface

	cacheEnabled     bool
	defaultCacheSize int
	cacheSizes       map[unversioned.GroupResource]int
}

// NewConfigGetter returns a restoptions.Getter implemented using information from the provided master config.
// By default, the etcd watch cache is enabled with a size of 1000 per resource type.
func NewConfigGetter(masterOptions configapi.MasterConfig) Getter {
	getter := &configRESTOptionsGetter{
		masterOptions:    masterOptions,
		cacheEnabled:     true,
		defaultCacheSize: 1000,
		cacheSizes:       map[unversioned.GroupResource]int{},
		restOptionsMap:   map[unversioned.GroupResource]genericrest.RESTOptions{},
	}

	if err := getter.loadWatchCacheSettings(); err != nil {
		glog.Error(err)
	}

	return getter
}

func (g *configRESTOptionsGetter) loadWatchCacheSettings() error {
	if g.masterOptions.KubernetesMasterConfig == nil {
		return nil
	}

	server := apiserveroptions.NewAPIServer()
	if errs := cmdflags.Resolve(g.masterOptions.KubernetesMasterConfig.APIServerArguments, server.AddFlags); len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}

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

	if g.etcdHelper == nil {
		// TODO: choose destination etcd based on input resource
		etcdClient, err := etcd.MakeNewEtcdClient(g.masterOptions.EtcdClientInfo)
		if err != nil {
			return genericrest.RESTOptions{}, err
		}
		// TODO: choose destination group/version based on input group/resource
		// TODO: Tune the cache size
		groupVersion := unversioned.GroupVersion{Group: "", Version: g.masterOptions.EtcdStorageConfig.OpenShiftStorageVersion}
		g.etcdHelper = etcdstorage.NewEtcdStorage(etcdClient, kapi.Codecs.LegacyCodec(groupVersion), g.masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix, false, genericserveroptions.DefaultDeserializationCacheSize)
	}

	configuredCacheSize, specified := g.cacheSizes[resource]
	if !specified || configuredCacheSize < 0 {
		configuredCacheSize = g.defaultCacheSize
	}

	decorator := func(s storage.Interface, requestedSize int, objectType runtime.Object, resourcePrefix string, scopeStrategy rest.NamespaceScopedStrategy, newListFunc func() runtime.Object) storage.Interface {
		capacity := requestedSize
		if capacity == UseConfiguredCacheSize {
			capacity = configuredCacheSize
		}

		if capacity == 0 || !g.cacheEnabled {
			glog.V(5).Infof("using uncached watch storage for %s", resource.String())
			return genericrest.UndecoratedStorage(s, capacity, objectType, resourcePrefix, scopeStrategy, newListFunc)
		} else {
			glog.V(5).Infof("using watch cache storage (capacity=%d) for %s", capacity, resource.String())
			return registry.StorageWithCacher(s, capacity, objectType, resourcePrefix, scopeStrategy, newListFunc)
		}
	}

	resourceOptions := genericrest.RESTOptions{
		Storage:                 g.etcdHelper,
		Decorator:               decorator,
		DeleteCollectionWorkers: 1,
	}
	g.restOptionsMap[resource] = resourceOptions

	return resourceOptions, nil
}
