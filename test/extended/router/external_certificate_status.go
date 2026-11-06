package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// routeHasAdmissionStatus requires a matching Admitted condition in every ingress.
func routeHasAdmissionStatus(route *routev1.Route, status corev1.ConditionStatus, reason string) bool {
	if len(route.Status.Ingress) == 0 {
		return false
	}
	for _, ingress := range route.Status.Ingress {
		matched := false
		for _, condition := range ingress.Conditions {
			if condition.Type == routev1.RouteAdmitted && condition.Status == status && condition.Reason == reason {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// routeAdmissionDiagnostics includes only allowlisted fields, excluding hostnames and free-form messages.
func routeAdmissionDiagnostics(route *routev1.Route) string {
	var ingresses []string
	for _, ingress := range route.Status.Ingress {
		var conditions []string
		for _, condition := range ingress.Conditions {
			conditions = append(conditions, fmt.Sprintf("type=%s status=%s reason=%s", condition.Type, condition.Status, condition.Reason))
		}
		ingresses = append(ingresses, fmt.Sprintf("conditions=[%s]", strings.Join(conditions, ", ")))
	}
	return fmt.Sprintf("resourceVersion=%s ingress=[%s]", route.ResourceVersion, strings.Join(ingresses, "; "))
}

// waitForRouteStatuses polls all expected routes together and retains safe admission diagnostics on failure.
func waitForRouteStatuses(ctx context.Context, interval time.Duration, namespace string, routes []*routev1.Route, status corev1.ConditionStatus, reason string, list func(context.Context) (*routev1.RouteList, error)) error {
	lastObserved := make(map[string]string, len(routes))
	err := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		observed, err := list(ctx)
		if err != nil {
			return false, err
		}
		byName := make(map[string]*routev1.Route, len(observed.Items))
		for index := range observed.Items {
			route := &observed.Items[index]
			byName[route.Name] = route
		}
		matched := len(routes) > 0
		for _, expected := range routes {
			route, exists := byName[expected.Name]
			if !exists {
				lastObserved[expected.Name] = "not found"
				matched = false
				continue
			}
			lastObserved[expected.Name] = routeAdmissionDiagnostics(route)
			if !routeHasAdmissionStatus(route, status, reason) {
				matched = false
			}
		}
		return matched, nil
	})
	if err == nil {
		return nil
	}
	var diagnostics []string
	for _, route := range routes {
		state, exists := lastObserved[route.Name]
		if !exists {
			state = "not observed"
		}
		diagnostics = append(diagnostics, fmt.Sprintf("%s/%s: %s", namespace, route.Name, state))
	}
	return fmt.Errorf("waiting for all routes to have Admitted reason=%s status=%s: %w; last observed admission states:\n%s", reason, status, err, strings.Join(diagnostics, "\n"))
}
