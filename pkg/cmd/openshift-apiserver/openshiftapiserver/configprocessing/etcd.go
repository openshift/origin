package configprocessing

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

// GetEtcdOptions takes configuration information and flag overrides to produce the upstream etcdoptions.
func GetEtcdOptions(startingFlags map[string][]string, etcdConnectionInfo configapi.EtcdConnectionInfo, storagePrefix string, defaultWatchCacheSizes map[schema.GroupResource]int) (*options.EtcdOptions, error) {
	storageConfig := storagebackend.NewDefaultConfig(storagePrefix, nil)
	storageConfig.Type = "etcd3"
	storageConfig.ServerList = etcdConnectionInfo.URLs
	storageConfig.KeyFile = etcdConnectionInfo.ClientCert.KeyFile
	storageConfig.CertFile = etcdConnectionInfo.ClientCert.CertFile
	storageConfig.CAFile = etcdConnectionInfo.CA

	etcdOptions := options.NewEtcdOptions(storageConfig)
	etcdOptions.DefaultStorageMediaType = "application/vnd.kubernetes.protobuf"
	etcdOptions.DefaultWatchCacheSize = 0
	if err := cmdflags.ResolveIgnoreMissing(startingFlags, etcdOptions.AddFlags); len(err) > 0 {
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
