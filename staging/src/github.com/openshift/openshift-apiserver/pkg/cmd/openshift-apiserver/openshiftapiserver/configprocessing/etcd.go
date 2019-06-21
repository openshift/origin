package configprocessing

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	apiserverflag "k8s.io/component-base/cli/flag"

	configv1 "github.com/openshift/api/config/v1"
	cmdflags "github.com/openshift/openshift-apiserver/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing/flags"
)

// GetEtcdOptions takes configuration information and flag overrides to produce the upstream etcdoptions.
func GetEtcdOptions(startingFlags map[string][]string, serializedConfig configv1.EtcdStorageConfig, defaultWatchCacheSizes map[schema.GroupResource]int) (*options.EtcdOptions, error) {
	storageConfig := storagebackend.NewDefaultConfig(serializedConfig.StoragePrefix, nil)
	storageConfig.Type = "etcd3"
	storageConfig.Transport.ServerList = serializedConfig.URLs
	storageConfig.Transport.KeyFile = serializedConfig.KeyFile
	storageConfig.Transport.CertFile = serializedConfig.CertFile
	storageConfig.Transport.CAFile = serializedConfig.CA

	etcdOptions := options.NewEtcdOptions(storageConfig)
	etcdOptions.DefaultStorageMediaType = "application/vnd.kubernetes.protobuf"
	etcdOptions.DefaultWatchCacheSize = 0
	fss := apiserverflag.NamedFlagSets{}
	etcdOptions.AddFlags(fss.FlagSet("etcd"))
	if err := cmdflags.ResolveIgnoreMissing(startingFlags, fss); len(err) > 0 {
		return nil, utilerrors.NewAggregate(err)
	}

	if etcdOptions.EnableWatchCache {
		watchCacheSizes := map[schema.GroupResource]int{}
		for k, v := range defaultWatchCacheSizes {
			watchCacheSizes[k] = v
		}

		if userSpecified, err := options.ParseWatchCacheSizes(etcdOptions.WatchCacheSizes); err == nil {
			for resource, size := range userSpecified {
				watchCacheSizes[resource] = size
			}
		}

		var err error
		etcdOptions.WatchCacheSizes, err = options.WriteWatchCacheSizes(watchCacheSizes)
		if err != nil {
			return nil, err
		}
	}

	return etcdOptions, nil
}
