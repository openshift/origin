package restoptions

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubeapiserver"

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
	restOptionsMap  map[schema.GroupResource]generic.RESTOptions

	storageFactory        serverstorage.StorageFactory
	defaultResourceConfig *serverstorage.ResourceConfig

	cacheEnabled            bool
	defaultCacheSize        int
	cacheSizes              map[schema.GroupResource]int
	quorumResources         map[schema.GroupResource]struct{}
	defaultResourcePrefixes map[schema.GroupResource]string
}

// NewConfigGetter returns a restoptions.Getter implemented using information from the provided master config.
// By default, the etcd watch cache is enabled with a size of 1000 per resource type.
// TODO: this class should either not need to know about configapi.MasterConfig, or not be in pkg/util
func NewConfigGetter(masterOptions configapi.MasterConfig, defaultResourceConfig *serverstorage.ResourceConfig, defaultResourcePrefixes map[schema.GroupResource]string, quorumResources map[schema.GroupResource]struct{}) Getter {
	getter := &configRESTOptionsGetter{
		masterOptions:           masterOptions,
		cacheEnabled:            true,
		defaultCacheSize:        1000,
		cacheSizes:              map[schema.GroupResource]int{},
		restOptionsMap:          map[schema.GroupResource]generic.RESTOptions{},
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

	storageGroupsToEncodingVersion, err := options.StorageSerialization.StorageGroupsToEncodingVersion()
	if err != nil {
		return err
	}

	storageConfig := options.Etcd.StorageConfig
	storageConfig.Prefix = g.masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix
	storageConfig.ServerList = g.masterOptions.EtcdClientInfo.URLs
	storageConfig.KeyFile = g.masterOptions.EtcdClientInfo.ClientCert.KeyFile
	storageConfig.CertFile = g.masterOptions.EtcdClientInfo.ClientCert.CertFile
	storageConfig.CAFile = g.masterOptions.EtcdClientInfo.CA

	resourceEncodingConfig := serverstorage.NewDefaultResourceEncodingConfig(kapi.Registry)

	storageFactory, err := kubeapiserver.NewStorageFactory(
		storageConfig,
		options.Etcd.DefaultStorageMediaType,
		kapi.Codecs,
		resourceEncodingConfig,
		storageGroupsToEncodingVersion,
		nil,
		g.defaultResourceConfig,
		options.APIEnablement.RuntimeConfig)
	if err != nil {
		return err
	}

	// TODO: the following works, but better make single resource overrides possible in BuildDefaultStorageFactory
	// instead of being late to the party and patching here:

	// use legacy group name "" for all resources that existed when apigroups were introduced
	for _, gvr := range []schema.GroupVersionResource{
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
		resourceEncodingConfig.SetResourceEncoding(gvr.GroupResource(), schema.GroupVersion{Version: gvr.Version}, schema.GroupVersion{Version: runtime.APIVersionInternal})
	}

	storageFactory.DefaultResourcePrefixes = g.defaultResourcePrefixes
	g.storageFactory = storageFactory

	g.cacheEnabled = options.Etcd.EnableWatchCache

	errs := []error{}
	for _, c := range options.GenericServerRunOptions.WatchCacheSizes {
		tokens := strings.Split(c, "#")
		if len(tokens) != 2 {
			errs = append(errs, fmt.Errorf("invalid watch cache size value '%s', expecting <resource>#<size> format (e.g. builds#100)", c))
			continue
		}

		resource := schema.ParseGroupResource(tokens[0])

		size, err := strconv.Atoi(tokens[1])
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid watch cache size value '%s': %v", c, err))
			continue
		}

		g.cacheSizes[resource] = size
	}
	return kerrors.NewAggregate(errs)
}

func (g *configRESTOptionsGetter) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	g.restOptionsLock.Lock()
	defer g.restOptionsLock.Unlock()
	if resourceOptions, ok := g.restOptionsMap[resource]; ok {
		return resourceOptions, nil
	}

	config, err := g.storageFactory.NewConfig(resource)
	if err != nil {
		return generic.RESTOptions{}, err
	}

	if _, ok := g.quorumResources[resource]; ok {
		config.Quorum = true
	}

	configuredCacheSize, specified := g.cacheSizes[resource]
	if !specified || configuredCacheSize < 0 {
		configuredCacheSize = g.defaultCacheSize
	}

	decorator := func(
		copier runtime.ObjectCopier,
		storageConfig *storagebackend.Config,
		requestedSize int,
		objectType runtime.Object,
		resourcePrefix string,
		keyFunc func(obj runtime.Object) (string, error),
		newListFn func() runtime.Object,
		getAttrsFunc storage.AttrFunc,
		triggerFn storage.TriggerPublisherFunc,
	) (storage.Interface, factory.DestroyFunc) {
		capacity := requestedSize
		if capacity == UseConfiguredCacheSize {
			capacity = configuredCacheSize
		}

		if capacity == 0 || !g.cacheEnabled {
			glog.V(5).Infof("using uncached watch storage for %s", resource.String())
			return generic.UndecoratedStorage(copier, storageConfig, capacity, objectType, resourcePrefix, keyFunc, newListFn, getAttrsFunc, triggerFn)
		}

		glog.V(5).Infof("using watch cache storage (capacity=%d) for %s %#v", capacity, resource.String(), storageConfig)
		return registry.StorageWithCacher(copier, storageConfig, capacity, objectType, resourcePrefix, keyFunc, newListFn, getAttrsFunc, triggerFn)
	}

	resourceOptions := generic.RESTOptions{
		StorageConfig:           config,
		Decorator:               decorator,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          g.storageFactory.ResourcePrefix(resource),
	}
	g.restOptionsMap[resource] = resourceOptions

	return resourceOptions, nil
}
