package dns

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
)

// ServiceAccessor is the interface used by the ServiceResolver to access
// services.
type ServiceAccessor interface {
	client.ServicesNamespacer
	ServiceByClusterIP(ip string) (*api.Service, error)
}

// cachedServiceAccessor provides a cache of services that can answer queries
// about service lookups efficiently.
type cachedServiceAccessor struct {
	store cache.Indexer
}

// cachedServiceAccessor implements ServiceAccessor
var _ ServiceAccessor = &cachedServiceAccessor{}

func NewCachedServiceAccessorAndStore() (ServiceAccessor, cache.Store) {
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, map[string]cache.IndexFunc{
		"clusterIP": indexServiceByClusterIP, // for reverse lookups
		"namespace": cache.MetaNamespaceIndexFunc,
	})
	return &cachedServiceAccessor{store: store}, store
}

// NewCachedServiceAccessor returns a service accessor that can answer queries about services.
// It uses a backing cache to make ClusterIP lookups efficient.
func NewCachedServiceAccessor(client cache.Getter, stopCh <-chan struct{}) ServiceAccessor {
	accessor, store := NewCachedServiceAccessorAndStore()
	lw := cache.NewListWatchFromClient(client, "services", api.NamespaceAll, fields.Everything())
	reflector := cache.NewReflector(lw, &api.Service{}, store, 30*time.Minute)
	if stopCh != nil {
		reflector.RunUntil(stopCh)
	} else {
		reflector.Run()
	}
	return accessor
}

// ServiceByClusterIP returns the first service that matches the provided clusterIP value.
// errors.IsNotFound(err) will be true if no such service exists.
func (a *cachedServiceAccessor) ServiceByClusterIP(ip string) (*api.Service, error) {
	items, err := a.store.Index("clusterIP", &api.Service{Spec: api.ServiceSpec{ClusterIP: ip}})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.NewNotFound(api.Resource("service"), "clusterIP="+ip)
	}
	return items[0].(*api.Service), nil
}

// indexServiceByClusterIP creates an index between a clusterIP and the service that
// uses it.
func indexServiceByClusterIP(obj interface{}) ([]string, error) {
	return []string{obj.(*api.Service).Spec.ClusterIP}, nil
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
		return nil, errors.NewNotFound(api.Resource("service"), name)
	}
	return item.(*api.Service), nil
}

func (a cachedServiceNamespacer) List(options api.ListOptions) (*api.ServiceList, error) {
	if !options.LabelSelector.Empty() {
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
func (a cachedServiceNamespacer) UpdateStatus(srv *api.Service) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Delete(name string) error {
	return fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Watch(options api.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) ProxyGet(scheme, name, port, path string, params map[string]string) restclient.ResponseWrapper {
	return nil
}

// cachedEndpointsAccessor provides a cache of services that can answer queries
// about service lookups efficiently.
type cachedEndpointsAccessor struct {
	store cache.Store
}

func NewCachedEndpointsAccessorAndStore() (client.EndpointsNamespacer, cache.Store) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	return &cachedEndpointsAccessor{store: store}, store
}

func (a *cachedEndpointsAccessor) Endpoints(namespace string) client.EndpointsInterface {
	return cachedEndpointsNamespacer{accessor: a, namespace: namespace}
}

// TODO: needs to be unified with Registry interfaces once that work is done.
type cachedEndpointsNamespacer struct {
	accessor  *cachedEndpointsAccessor
	namespace string
}

var _ client.EndpointsInterface = cachedEndpointsNamespacer{}

func (a cachedEndpointsNamespacer) Get(name string) (*api.Endpoints, error) {
	item, ok, err := a.accessor.store.Get(&api.Endpoints{ObjectMeta: api.ObjectMeta{Namespace: a.namespace, Name: name}})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.NewNotFound(api.Resource("endpoints"), name)
	}
	return item.(*api.Endpoints), nil
}

func (a cachedEndpointsNamespacer) List(options api.ListOptions) (*api.EndpointsList, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedEndpointsNamespacer) Create(srv *api.Endpoints) (*api.Endpoints, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedEndpointsNamespacer) Update(srv *api.Endpoints) (*api.Endpoints, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedEndpointsNamespacer) Delete(name string) error {
	return fmt.Errorf("not implemented")
}
func (a cachedEndpointsNamespacer) Watch(options api.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}
