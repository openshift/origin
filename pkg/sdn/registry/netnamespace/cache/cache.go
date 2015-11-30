package cache

import (
	"fmt"

	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

type NetNamespaceCache struct {
	Client osclient.Interface
	Store  cache.Store
}

var netnsCache *NetNamespaceCache

func GetNetNamespaceCache() (*NetNamespaceCache, error) {
	if netnsCache == nil {
		return nil, fmt.Errorf("NetNamespace cache not initialized")
	}
	return netnsCache, nil
}

func RunNetNamespaceCache(c osclient.Interface) {
	if netnsCache != nil {
		return
	}

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return c.NetNamespaces().List()
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.NetNamespaces().Watch(resourceVersion)
			},
		},
		&sdnapi.NetNamespace{},
		store,
		0,
	)
	reflector.Run()
	netnsCache = &NetNamespaceCache{
		Client: c,
		Store:  store,
	}
}
