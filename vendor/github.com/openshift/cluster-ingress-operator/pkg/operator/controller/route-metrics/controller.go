package routemetrics

import (
	"context"
	"fmt"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	logf "github.com/openshift/cluster-ingress-operator/pkg/log"
	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "route_metrics_controller"
)

var (
	log = logf.Logger.WithName(controllerName)
)

// New creates the route metrics controller. This is the controller
// that handles all the logic for gathering and exporting
// metrics related to route resources.
func New(mgr manager.Manager, namespace string) (controller.Controller, error) {
	// Create a new cache to watch on Route objects from every namespace.
	newCache, err := cache.New(mgr.GetConfig(), cache.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return nil, err
	}
	// Add the cache to the manager so that the cache is started along with the other runnables.
	mgr.Add(newCache)
	reconciler := &reconciler{
		cache:            newCache,
		namespace:        namespace,
		routeToIngresses: make(map[types.NamespacedName]sets.String),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler: reconciler,
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			// Rate-limit to 1 update every 5 seconds per
			// ingresscontroller to avoid burning CPU if route
			// updates are frequent.
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 30*time.Second),
			// 10 qps, 100 bucket size, same as DefaultControllerRateLimiter().
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
	})
	if err != nil {
		return nil, err
	}
	// add watch for changes in IngressController
	if err := c.Watch(&source.Kind{Type: &operatorv1.IngressController{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}
	// add watch for changes in Route
	if err := c.Watch(source.NewKindWithCache(&routev1.Route{}, newCache),
		handler.EnqueueRequestsFromMapFunc(reconciler.routeToIngressController)); err != nil {
		return nil, err
	}
	return c, nil
}

// routeToIngressController creates a reconcile.Request for all the Ingress Controllers related to the Route object.
func (r *reconciler) routeToIngressController(obj client.Object) []reconcile.Request {
	var requests []reconcile.Request
	// Cast the received object into Route object.
	route := obj.(*routev1.Route)

	// Create the NamespacedName for the Route.
	routeNamespacedName := types.NamespacedName{
		Namespace: route.Namespace,
		Name:      route.Name,
	}

	// Create a set of current Ingresses of the Route to easily retrieve them.
	currentRouteIngresses := sets.NewString()

	// Iterate through the related Route's Ingresses.
	for _, ri := range route.Status.Ingress {
		// Check if the Route was admitted by the RouteIngress.
		for _, cond := range ri.Conditions {
			if cond.Type == routev1.RouteAdmitted && cond.Status == corev1.ConditionTrue {
				log.Info("queueing ingresscontroller", "name", ri.RouterName)
				// Create a reconcile.Request for the router named in the RouteIngress.
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ri.RouterName,
						Namespace: r.namespace,
					},
				}
				requests = append(requests, request)

				// Add the Router Name to the currentIngressSet.
				currentRouteIngresses.Insert(ri.RouterName)
			}
		}
	}

	// Get the previous set of Ingresses of the Route.
	previousRouteIngresses := r.routeToIngresses[routeNamespacedName]

	// Iterate through the previousRouteIngresses.
	for routerName := range previousRouteIngresses {
		// Check if the currentRouteIngresses contains the Router Name. If it does not,
		// then the Ingress was removed from the Route Status. The reconcile loop is needed
		// to be run for the corresponding Ingress Controller.
		if !currentRouteIngresses.Has(routerName) {
			log.Info("queueing ingresscontroller", "name", routerName)
			// Create a reconcile.Request for the router named in the RouteIngress.
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      routerName,
					Namespace: r.namespace,
				},
			}
			requests = append(requests, request)
		}
	}

	// Map the currentRouteIngresses to Route's NamespacedName.
	r.routeToIngresses[routeNamespacedName] = currentRouteIngresses

	return requests
}

// reconciler handles the actual ingresscontroller reconciliation logic in response to events.
type reconciler struct {
	cache     cache.Cache
	namespace string
	// routeToIngresses stores the Ingress Controllers that have admitted a given route.
	routeToIngresses map[types.NamespacedName]sets.String
}

// Reconcile expects request to refer to an Ingress Controller resource, and will do all the work to gather metrics related to
// the resource.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Info("reconciling", "request", request)

	// Fetch the Ingress Controller object.
	ingressController := &operatorv1.IngressController{}
	if err := r.cache.Get(ctx, request.NamespacedName, ingressController); err != nil {
		if kerrors.IsNotFound(err) {
			// This means the Ingress Controller object was already deleted/finalized.
			log.Info("Ingress Controller not found; reconciliation will be skipped", "request", request)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get Ingress Controller %q: %w", request, err)
	}

	// If the Ingress Controller is marked to be deleted, then return early. The corresponding RouteMetricsControllerRoutesPerShard metric label
	// will be deleted in "ensureIngressDeleted" function of ingresscontroller.
	if ingressController.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// NOTE: Even though the route admitted status should reflect validity of the namespace and route labelselectors, we still will validate
	// the namespace and route labels as there are still edge scenarios where the route status may be inaccurate.

	// List all the Namespaces filtered by our ingress's Namespace selector.
	namespaceMatchingLabelsSelector := client.MatchingLabelsSelector{Selector: labels.Everything()}
	if ingressController.Spec.NamespaceSelector != nil {
		namespaceSelector, err := metav1.LabelSelectorAsSelector(ingressController.Spec.NamespaceSelector)
		if err != nil {
			log.Error(err, "ingresscontroller has an invalid namespace selector", "ingresscontroller",
				ingressController.Name, "namespaceSelector", ingressController.Spec.NamespaceSelector)
			return reconcile.Result{}, nil
		}
		namespaceMatchingLabelsSelector = client.MatchingLabelsSelector{Selector: namespaceSelector}
	}

	namespaceList := corev1.NamespaceList{}
	if err := r.cache.List(ctx, &namespaceList, namespaceMatchingLabelsSelector); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list Namespaces %q: %w", request, err)
	}
	// Create a set of Namespaces to easily look up Namespaces that matches the Routes assigned to the Ingress Controller.
	namespacesSet := sets.NewString()
	for i := range namespaceList.Items {
		namespacesSet.Insert(namespaceList.Items[i].Name)
	}

	// List routes filtered by our ingress's route selector.
	routeMatchingLabelsSelector := client.MatchingLabelsSelector{Selector: labels.Everything()}
	if ingressController.Spec.RouteSelector != nil {
		routeSelector, err := metav1.LabelSelectorAsSelector(ingressController.Spec.RouteSelector)
		if err != nil {
			log.Error(err, "ingresscontroller has an invalid route selector", "ingresscontroller",
				ingressController.Name, "routeSelector", ingressController.Spec.RouteSelector)
			return reconcile.Result{}, nil
		}
		routeMatchingLabelsSelector = client.MatchingLabelsSelector{Selector: routeSelector}
	}
	routeList := routev1.RouteList{}
	if err := r.cache.List(ctx, &routeList, routeMatchingLabelsSelector); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list Routes for the Shard %q: %w", request, err)
	}

	// Variable to store the number of routes admitted by the Shard (Ingress Controller).
	routesAdmitted := 0

	// Iterate through the list Routes.
	for _, route := range routeList.Items {
		// Check if the Route's Namespace matches one of the Namespaces in the set namespacesSet and
		// the Route is admitted by the Ingress Controller.
		if namespacesSet.Has(route.Namespace) && routeStatusAdmitted(route, ingressController.Name) {
			// If the Route is admitted then, the routesAdmitted should be incremented by 1 for the Shard.
			routesAdmitted++
		}
	}

	// Set the value of the metric to the number of routesAdmitted for the corresponding Shard (Ingress Controller).
	SetRouteMetricsControllerRoutesPerShardMetric(request.Name, float64(routesAdmitted))

	return reconcile.Result{}, nil
}

// routeStatusAdmitted returns true if a given route's status shows admitted by the Ingress Controller.
func routeStatusAdmitted(route routev1.Route, ingressControllerName string) bool {
	// Iterate through the related Ingress Controllers.
	for _, ingress := range route.Status.Ingress {
		// Check if the RouterName matches the name of the Ingress Controller.
		if ingress.RouterName == ingressControllerName {
			// Check if the Route was admitted by the Ingress Controller.
			for _, cond := range ingress.Conditions {
				if cond.Type == routev1.RouteAdmitted && cond.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}
	}
	return false
}
