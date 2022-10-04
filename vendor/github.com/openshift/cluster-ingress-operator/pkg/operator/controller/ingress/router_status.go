package ingress

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"reflect"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	operatorcontroller "github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A brief overview of the operator's interactions with route status:
// Though the openshift-router is mainly responsible for route object status, the operator plays a small, but
// significant role in ensuring the route status is accurate. The openshift-router updates the route object's status
// when it is admitted to an ingress controller. However, the openshift-router is unable to reliably update the route's
// status when it stops managing the route. Here are the scenarios where the operator steps in:
// #1 When the ingress controller, the corresponding router deployment, and its pods are deleted.
//    - The operator knows when a router is deleted because it is the one responsible for deleting it. So it
//      simply calls clearRouteStatus to clear status of routes that openshift-router has admitted.
// #2 When the ingress controller sharding configuration (i.e., selectors) is changed.
//    - When the selectors (routeSelector and namespaceSelector) are updated, the operator simply clears the status of
//      any route that it is no longer selecting using the updated selectors.
//    - We determine what routes are admitted by the current state of the selectors (just like the openshift-router).

// syncRouteStatus ensures that all routes status have been synced with the ingress controller's state.
func (r *reconciler) syncRouteStatus(ic *operatorv1.IngressController) []error {
	// Clear routes that are not admitted by this ingress controller if route selectors have been updated.
	if routeSelectorsUpdated(ic) {
		// Only clear once we are done rolling out routers.
		// We want to avoid race condition in which we clear status and an old router re-admits it before terminated.
		if done, err := r.isRouterDeploymentRolloutComplete(ic); err != nil {
			return []error{err}
		} else if done {
			// Clear routes status not admitted by this ingress controller.
			if errs := r.clearRoutesNotAdmittedByIngress(ic); len(errs) > 0 {
				return errs
			}

			// Now sync the selectors from the spec to the status, so we indicate we are done clearing status.
			if err := r.syncIngressControllerSelectorStatus(ic); err != nil {
				return []error{err}
			}
		}
	}
	return nil
}

// isRouterDeploymentRolloutComplete determines whether the rollout of the ingress router deployment is complete.
func (r *reconciler) isRouterDeploymentRolloutComplete(ic *operatorv1.IngressController) (bool, error) {
	deployment := appsv1.Deployment{}
	deploymentName := operatorcontroller.RouterDeploymentName(ic)
	if err := r.client.Get(context.TODO(), deploymentName, &deployment); err != nil {
		return false, fmt.Errorf("failed to get deployment %s: %w", deploymentName, err)
	}

	if deployment.Generation != deployment.Status.ObservedGeneration {
		return false, nil
	}
	if deployment.Status.Replicas != deployment.Status.UpdatedReplicas {
		return false, nil
	}

	return true, nil
}

// clearAllRoutesStatusForIngressController clears any route status that have been
// admitted by provided ingress controller.
func (r *reconciler) clearAllRoutesStatusForIngressController(icName string) []error {
	// List all routes.
	errs := []error{}
	start := time.Now()
	routeList := &routev1.RouteList{}
	routesCleared := 0
	if err := r.client.List(context.TODO(), routeList); err != nil {
		return append(errs, fmt.Errorf("failed to list all routes in order to clear route status for deployment %s: %w", icName, err))
	}
	// Clear status on the routes that belonged to icName.
	for i := range routeList.Items {
		if cleared, err := r.clearRouteStatus(&routeList.Items[i], icName); err != nil {
			errs = append(errs, err)
		} else if cleared {
			routesCleared++
		}
	}
	elapsed := time.Since(start)
	log.Info("cleared all route status for ingress", "Ingress Controller",
		icName, "Routes Status Cleared", routesCleared, "Time Elapsed", elapsed)

	return errs
}

// clearRouteStatus clears a route's status that is admitted by a specific ingress controller.
func (r *reconciler) clearRouteStatus(route *routev1.Route, icName string) (bool, error) {
	// Go through each route and clear status if admitted by this ingress controller.
	var updated routev1.Route
	for i := range route.Status.Ingress {
		if condition := findCondition(&route.Status.Ingress[i], routev1.RouteAdmitted); condition != nil {
			if route.Status.Ingress[i].RouterName == icName {
				// Remove this status since it matches our routerName.
				route.DeepCopyInto(&updated)
				updated.Status.Ingress = append(route.Status.Ingress[:i], route.Status.Ingress[i+1:]...)
				if err := r.client.Status().Update(context.TODO(), &updated); err != nil {
					return false, fmt.Errorf("failed to clear route status of %s/%s for routerName %s: %w",
						route.Namespace, route.Name, icName, err)
				}
				log.Info("cleared admitted status for route", "Route", route.Namespace+"/"+route.Name,
					"Ingress Controller", icName)
				return true, nil
			}
		}
	}

	return false, nil
}

// routeSelectorsUpdated returns whether any of the route selectors have been updated by comparing
// the status selector fields to the spec selector fields.
func routeSelectorsUpdated(ingress *operatorv1.IngressController) bool {
	if !reflect.DeepEqual(ingress.Spec.RouteSelector, ingress.Status.RouteSelector) ||
		!reflect.DeepEqual(ingress.Spec.NamespaceSelector, ingress.Status.NamespaceSelector) {
		return true
	}
	return false
}

// clearRoutesNotAdmittedByIngress clears routes status that are not selected by a specific ingress controller.
func (r *reconciler) clearRoutesNotAdmittedByIngress(ingress *operatorv1.IngressController) []error {
	start := time.Now()
	errs := []error{}

	// List all routes.
	routeList := &routev1.RouteList{}
	if err := r.client.List(context.TODO(), routeList); err != nil {
		return append(errs, fmt.Errorf("failed to list all routes in order to clear route status: %w", err))
	}

	// List namespaces filtered by our ingress's namespace selector.
	namespaceSelector, err := metav1.LabelSelectorAsSelector(ingress.Spec.NamespaceSelector)
	if err != nil {
		return append(errs, fmt.Errorf("ingresscontroller %s has an invalid namespace selector: %w", ingress.Name, err))
	}
	filteredNamespaceList := &corev1.NamespaceList{}
	if err := r.client.List(context.TODO(), filteredNamespaceList,
		client.MatchingLabelsSelector{Selector: namespaceSelector}); err != nil {
		return append(errs, fmt.Errorf("failed to list all namespaces in order to clear route status for %s: %w", ingress.Name, err))
	}

	// Create a set of namespaces to easily look up namespaces in this shard.
	namespacesInShard := sets.NewString()
	for i := range filteredNamespaceList.Items {
		namespacesInShard.Insert(filteredNamespaceList.Items[i].Name)
	}

	// List routes filtered by our ingress's route selector.
	routeSelector, err := metav1.LabelSelectorAsSelector(ingress.Spec.RouteSelector)
	if err != nil {
		return append(errs, fmt.Errorf("ingresscontroller %s has an invalid route selector: %w", ingress.Name, err))
	}

	// Iterate over the entire route list and clear if not selected by route selector OR namespace selector.
	routesCleared := 0
	for i := range routeList.Items {
		route := &routeList.Items[i]

		routeInShard := routeSelector.Matches(labels.Set(route.Labels))
		namespaceInShard := namespacesInShard.Has(route.Namespace)

		if !routeInShard || !namespaceInShard {
			if cleared, err := r.clearRouteStatus(route, ingress.ObjectMeta.Name); err != nil {
				errs = append(errs, err)
			} else if cleared {
				routesCleared++
			}
		}

	}
	elapsed := time.Since(start)
	log.Info("cleared route status after selector update", "Ingress Controller", ingress.Name, "Routes Status Cleared", routesCleared, "Time Elapsed", elapsed)
	return errs
}

// findCondition locates the first condition that corresponds to the requested type.
func findCondition(ingress *routev1.RouteIngress, t routev1.RouteIngressConditionType) *routev1.RouteIngressCondition {
	for i := range ingress.Conditions {
		if ingress.Conditions[i].Type == t {
			return &ingress.Conditions[i]
		}
	}
	return nil
}
