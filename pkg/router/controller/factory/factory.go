package factory

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
)

type RouterControllerFactory struct {
	KClient  kclient.Interface
	OSClient osclient.Interface
}

func (factory *RouterControllerFactory) Create(plugin router.Plugin) *controller.RouterController {
	routeEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&routeLW{factory.OSClient}, &routeapi.Route{}, routeEventQueue, 2*time.Minute).Run()

	endpointsEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&endpointsLW{factory.KClient}, &kapi.Endpoints{}, endpointsEventQueue, 2*time.Minute).Run()

	return &controller.RouterController{
		Plugin: plugin,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints, error) {
			eventType, obj, err := endpointsEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*kapi.Endpoints), nil
		},
		NextRoute: func() (watch.EventType, *routeapi.Route, error) {
			eventType, obj, err := routeEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*routeapi.Route), nil
		},
	}
}

type routeLW struct {
	client osclient.Interface
}

func (lw *routeLW) List() (runtime.Object, error) {
	return lw.client.Routes(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
}

func (lw *routeLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Routes(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
}

type endpointsLW struct {
	client kclient.Interface
}

func (lw *endpointsLW) List() (runtime.Object, error) {
	return lw.client.Endpoints(kapi.NamespaceAll).List(labels.Everything())
}

func (lw *endpointsLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Endpoints(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
}
