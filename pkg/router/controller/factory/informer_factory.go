package factory

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	idlerclient "github.com/openshift/service-idler/pkg/client/clientset/versioned/typed/idling/v1alpha2"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kcache "k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	informerfactory "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
)

type RouterInformerFactory struct {
	KClient       kclientset.Interface
	RClient       routeclientset.Interface
	ProjectClient projectclient.ProjectResourceInterface
	IdlerClient   idlerclient.IdlersGetter

	ResyncInterval  time.Duration
	Namespace       string
	LabelSelector   string
	FieldSelector   string
	NamespaceLabels labels.Selector
	ProjectLabels   labels.Selector
	RouteModifierFn func(route *routeapi.Route)

	informers map[reflect.Type]kcache.SharedIndexInformer
}

func (f *RouterInformerFactory) createEndpointsSharedInformer() {
	// we do not scope endpoints by labels or fields because the route labels != endpoints labels
	lw := &kcache.ListWatch{
		ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
			return f.KClient.Core().Endpoints(f.Namespace).List(options)
		},
		WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
			return f.KClient.Core().Endpoints(f.Namespace).Watch(options)
		},
	}
	ep := &kapi.Endpoints{}
	objType := reflect.TypeOf(ep)
	indexers := kcache.Indexers{kcache.NamespaceIndex: kcache.MetaNamespaceIndexFunc}
	informer := kcache.NewSharedIndexInformer(lw, ep, f.ResyncInterval, indexers)
	f.informers[objType] = informer
}

func (f *RouterInformerFactory) createServicesSharedInformer() {
	// we do not scope services by labels or fields because the route labels != endpoints labels
	lw := &kcache.ListWatch{
		ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
			return f.KClient.Core().Services(f.Namespace).List(options)
		},
		WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
			return f.KClient.Core().Services(f.Namespace).Watch(options)
		},
	}
	ep := &kapi.Service{}
	objType := reflect.TypeOf(ep)
	indexers := kcache.Indexers{kcache.NamespaceIndex: kcache.MetaNamespaceIndexFunc}
	informer := kcache.NewSharedIndexInformer(lw, ep, f.ResyncInterval, indexers)
	f.informers[objType] = informer
}

func (f *RouterInformerFactory) setSelectors(options *v1.ListOptions) {
	options.LabelSelector = f.LabelSelector
	options.FieldSelector = f.FieldSelector
}

func (f *RouterInformerFactory) createNodesSharedInformer() {
	// Use stock node informer as we don't need namespace/labels/fields filtering on nodes
	ifactory := informerfactory.NewSharedInformerFactory(f.KClient, f.ResyncInterval)
	informer := ifactory.Core().InternalVersion().Nodes().Informer()
	objType := reflect.TypeOf(&kapi.Node{})
	f.informers[objType] = informer
}

func (f *RouterInformerFactory) createNamespacesSharedInformer() {
	lw := &kcache.ListWatch{
		ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = f.NamespaceLabels.String()
			return f.KClient.Core().Namespaces().List(options)
		},
		WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = f.NamespaceLabels.String()
			return f.KClient.Core().Namespaces().Watch(options)
		},
	}
	ns := &kapi.Namespace{}
	objType := reflect.TypeOf(ns)
	indexers := kcache.Indexers{kcache.NamespaceIndex: kcache.MetaNamespaceIndexFunc}
	informer := kcache.NewSharedIndexInformer(lw, ns, f.ResyncInterval, indexers)
	f.informers[objType] = informer
}

func (f *RouterInformerFactory) createIdlersSharedInformer() {
	lw := &kcache.ListWatch{
		ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
			return f.IdlerClient.Idlers(f.Namespace).List(options)
		},
		WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
			return f.IdlerClient.Idlers(f.Namespace).Watch(options)
		},
	}
	idler := &idling.Idler{}
	objType := reflect.TypeOf(idler)
	indexers := kcache.Indexers{kcache.NamespaceIndex: kcache.MetaNamespaceIndexFunc}
	informer := kcache.NewSharedIndexInformer(lw, idler, f.ResyncInterval, indexers)
	f.informers[objType] = informer
}

func (f *RouterInformerFactory) createRoutesSharedInformer() {
	lw := &kcache.ListWatch{
		ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
			f.setSelectors(&options)
			routeList, err := f.RClient.Route().Routes(f.Namespace).List(options)
			if err != nil {
				return nil, err
			}
			if f.RouteModifierFn != nil {
				for i := range routeList.Items {
					f.RouteModifierFn(&routeList.Items[i])
				}
			}
			// Return routes in order of age to avoid rejections during resync
			sort.Sort(routeAge(routeList.Items))
			return routeList, nil
		},
		WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
			f.setSelectors(&options)
			w, err := f.RClient.Route().Routes(f.Namespace).Watch(options)
			if err != nil {
				return nil, err
			}
			if f.RouteModifierFn != nil {
				w = watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
					if route, ok := in.Object.(*routeapi.Route); ok {
						f.RouteModifierFn(route)
					}
					return in, true
				})
			}
			return w, nil
		},
	}
	rt := &routeapi.Route{}
	objType := reflect.TypeOf(rt)
	indexers := kcache.Indexers{kcache.NamespaceIndex: kcache.MetaNamespaceIndexFunc}
	informer := kcache.NewSharedIndexInformer(lw, rt, f.ResyncInterval, indexers)
	f.informers[objType] = informer
}

// Run starts all informers and waits for their caches to be synced
func (f *RouterInformerFactory) Run() {
	// Start informers
	for _, informer := range f.informers {
		go informer.Run(utilwait.NeverStop)
	}
	// Wait for informers cache to be synced
	for objType, informer := range f.informers {
		if !kcache.WaitForCacheSync(utilwait.NeverStop, informer.HasSynced) {
			utilruntime.HandleError(fmt.Errorf("failed to sync cache for %+v shared informer", objType))
		}
	}
}

func (f *RouterInformerFactory) initInformerFor(obj runtime.Object) {
	switch obj.(type) {
	case *kapi.Namespace:
		f.createNamespacesSharedInformer()
	case *kapi.Endpoints:
		f.createEndpointsSharedInformer()
	case *routeapi.Route:
		f.createRoutesSharedInformer()
	case *kapi.Service:
		f.createServicesSharedInformer()
	case *kapi.Node:
		f.createNodesSharedInformer()
	case *idling.Idler:
		f.createIdlersSharedInformer()
	}
}

// InformerFor returns an informer for the given type, lazily loading it if necessary.
func (f *RouterInformerFactory) InformerFor(obj runtime.Object) (kcache.SharedIndexInformer, bool) {
	objType := reflect.TypeOf(obj)
	_, present := f.informers[objType]
	if !present {
		f.initInformerFor(obj)
	}
	informer, present := f.informers[objType]
	return informer, present
}

// InformerFor returns an informer for the given type, without lazy loading.
func (f *RouterInformerFactory) StrictInformerFor(obj runtime.Object) (kcache.SharedIndexInformer, bool) {
	objType := reflect.TypeOf(obj)
	informer, present := f.informers[objType]
	return informer, present
}

func NewDefaultRouterInformerFactory(rc routeclientset.Interface, pc projectclient.ProjectResourceInterface, kc kclientset.Interface, ic idlerclient.IdlersGetter) *RouterInformerFactory {
	return &RouterInformerFactory{
		KClient:        kc,
		RClient:        rc,
		ProjectClient:  pc,
		IdlerClient:    ic,
		ResyncInterval: DefaultResyncInterval,

		Namespace: v1.NamespaceAll,
		informers: map[reflect.Type]kcache.SharedIndexInformer{},
	}
}
