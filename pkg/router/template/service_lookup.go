package templaterouter

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// ServiceLookup is an interface for fetching the service associated with the given endpoints
type ServiceLookup interface {
	LookupService(*api.Endpoints) (*api.Service, error)
}

func NewListWatchServiceLookup(svcGetter client.ServicesNamespacer, resync time.Duration) ServiceLookup {
	svcStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	lw := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return svcGetter.Services(api.NamespaceAll).List(options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return svcGetter.Services(api.NamespaceAll).Watch(options)
		},
	}
	cache.NewReflector(lw, &api.Service{}, svcStore, resync).Run()

	return &serviceLWLookup{
		store: svcStore,
	}
}

type serviceLWLookup struct {
	store cache.Store
}

func (c *serviceLWLookup) LookupService(endpoints *api.Endpoints) (*api.Service, error) {
	var rawSvc interface{}
	var ok bool
	var err error

	if rawSvc, ok, err = c.store.Get(endpoints); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.NewNotFound(unversioned.GroupResource{
			Group:    api.GroupName,
			Resource: "Service",
		}, endpoints.Name)
	}

	return rawSvc.(*api.Service), nil
}
