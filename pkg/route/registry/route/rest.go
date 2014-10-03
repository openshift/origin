package route

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/api/validation"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

func NewREST(registry Registry) *REST {
	return &REST{
		registry: registry,
	}
}

func (rs *REST) New() runtime.Object {
	return &api.Route{}
}

// List obtains a list of Routes that match selector.
func (rs *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	list, err := rs.registry.ListRoutes(selector)
	if err != nil {
		return nil, err
	}
	return list, err
}

// Get obtains the route specified by its id.
func (rs *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	route, err := rs.registry.GetRoute(id)
	if err != nil {
		return nil, err
	}
	return route, err
}

// Delete asynchronously deletes the Route specified by its id.
func (rs *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	_, err := rs.registry.GetRoute(id)
	if err != nil {
		return nil, err
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, rs.registry.DeleteRoute(id)
	}), nil
}

// Create registers a given new Route instance to rs.registry.
func (rs *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, fmt.Errorf("not a route: %#v", obj)
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, errors.NewInvalid("route", route.ID, errs)
	}
	if len(route.ID) == 0 {
		route.ID = uuid.NewUUID().String()
	}

	route.CreationTimestamp = util.Now()

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := rs.registry.CreateRoute(route)
		if err != nil {
			return nil, err
		}
		return rs.registry.GetRoute(route.ID)
	}), nil
}

// Update replaces a given Route instance with an existing instance in rs.registry.
func (rs *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, fmt.Errorf("not a route: %#v", obj)
	}
	if len(route.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", route)
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, errors.NewInvalid("route", route.ID, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := rs.registry.UpdateRoute(route)
		if err != nil {
			return nil, err
		}
		return rs.registry.GetRoute(route.ID)
	}), nil
}

// Watch returns Routes events via a watch.Interface.
// It implements apiserver.ResourceWatcher.
func (rs *REST) Watch(ctx kubeapi.Context, label, field labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return rs.registry.WatchRoutes(label, field, resourceVersion)
}
