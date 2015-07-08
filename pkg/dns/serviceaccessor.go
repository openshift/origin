package dns

import (
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

// ServiceAccessor is the interface used by the ServiceResolver to access
// services.
type ServiceAccessor interface {
	client.ServicesNamespacer
	ServiceByPortalIP(ip string) (*api.Service, error)
}

// cachedServiceAccessor provides a cache of services that can answer queries
// about service lookups efficiently.
type cachedServiceAccessor struct {
	reflector *cache.Reflector
	store     cache.Indexer
}

// cachedServiceAccessor implements ServiceAccessor
var _ ServiceAccessor = &cachedServiceAccessor{}

// NewCachedServiceAccessor returns a service accessor that can answer queries about services.
// It uses a backing cache to make PortalIP lookups efficient.
func NewCachedServiceAccessor(client *client.Client, stopCh <-chan struct{}) ServiceAccessor {
	lw := cache.NewListWatchFromClient(client, "services", api.NamespaceAll, fields.Everything())
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, map[string]cache.IndexFunc{
		"portalIP":  indexServiceByPortalIP, // for reverse lookups
		"namespace": cache.MetaNamespaceIndexFunc,
	})
	reflector := cache.NewReflector(lw, &api.Service{}, store, 2*time.Minute)
	if stopCh != nil {
		reflector.RunUntil(stopCh)
	} else {
		reflector.Run()
	}
	return &cachedServiceAccessor{
		reflector: reflector,
		store:     store,
	}
}

// ServiceByPortalIP returns the first service that matches the provided portalIP value.
// errors.IsNotFound(err) will be true if no such service exists.
func (a *cachedServiceAccessor) ServiceByPortalIP(ip string) (*api.Service, error) {
	items, err := a.store.Index("portalIP", &api.Service{Spec: api.ServiceSpec{ClusterIP: ip}})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.NewNotFound("service", "portalIP="+ip)
	}
	return items[0].(*api.Service), nil
}

// indexServiceByPortalIP creates an index between a portalIP and the service that
// uses it.
func indexServiceByPortalIP(obj interface{}) (string, error) {
	return obj.(*api.Service).Spec.ClusterIP, nil
}

func (a *cachedServiceAccessor) Services(namespace string) client.ServiceInterface {
	return cachedServiceNamespacer{a, namespace}
}

// TODO: needs to be unified with Registry interfaces once that work is done.
type cachedServiceNamespacer struct {
	accessor  *cachedServiceAccessor
	namespace string
}

var _ client.ServiceInterface = cachedServiceNamespacer{}

func (a cachedServiceNamespacer) Get(name string) (*api.Service, error) {
	item, ok, err := a.accessor.store.Get(&api.Service{ObjectMeta: api.ObjectMeta{Namespace: a.namespace, Name: name}})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.NewNotFound("service", name)
	}
	return item.(*api.Service), nil
}

func (a cachedServiceNamespacer) List(label labels.Selector) (*api.ServiceList, error) {
	if !label.Empty() {
		return nil, fmt.Errorf("label selection on the cache is not currently implemented")
	}
	items, err := a.accessor.store.Index("namespace", &api.Service{ObjectMeta: api.ObjectMeta{Namespace: a.namespace}})
	if err != nil {
		return nil, err
	}
	services := make([]api.Service, 0, len(items))
	for i := range items {
		services = append(services, *items[i].(*api.Service))
	}
	return &api.ServiceList{
		// TODO: set ResourceVersion so that we can make watch work.
		Items: services,
	}, nil
}

func (a cachedServiceNamespacer) Create(srv *api.Service) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Update(srv *api.Service) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Delete(name string) error {
	return fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}
