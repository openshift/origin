package router

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRouteHasAdmissionStatus rejects incomplete or mismatched admission conditions in any ingress.
func TestRouteHasAdmissionStatus(t *testing.T) {
	t.Parallel()
	matching := routev1.RouteIngressCondition{Type: routev1.RouteAdmitted, Status: corev1.ConditionFalse, Reason: "ExternalCertificateValidationFailed"}
	wrongReason := matching
	wrongReason.Reason = "ExternalCertificateSARCompleted"
	wrongStatus := matching
	wrongStatus.Status = corev1.ConditionTrue
	wrongType := matching
	wrongType.Type = "Other"
	for _, test := range []struct {
		name    string
		ingress []routev1.RouteIngress
		want    bool
	}{
		{name: "no ingress"},
		{name: "no conditions", ingress: []routev1.RouteIngress{{}}},
		{name: "wrong reason", ingress: []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{wrongReason}}}},
		{name: "wrong status", ingress: []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{wrongStatus}}}},
		{name: "wrong type", ingress: []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{wrongType}}}},
		{name: "match", ingress: []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{matching}}}, want: true},
		{name: "one ingress unmatched", ingress: []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{matching}}, {}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			route := &routev1.Route{Status: routev1.RouteStatus{Ingress: test.ingress}}
			if got := routeHasAdmissionStatus(route, corev1.ConditionFalse, matching.Reason); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

// TestWaitForRouteStatuses checks batch success and useful, safe errors for missing routes and polling failures.
func TestWaitForRouteStatuses(t *testing.T) {
	t.Parallel()
	routes := []*routev1.Route{{ObjectMeta: metav1.ObjectMeta{Name: "route-0"}}, {ObjectMeta: metav1.ObjectMeta{Name: "route-1"}}}
	reason := "ExternalCertificateValidationFailed"
	apiError := errors.New("route list failed")
	for _, test := range []struct {
		name      string
		listError error
		match     bool
		missing   bool
	}{
		{name: "timeout"},
		{name: "API error", listError: apiError},
		{name: "match", match: true},
		{name: "missing route", missing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := waitForRouteStatuses(ctx, time.Millisecond, "test-namespace", routes, corev1.ConditionFalse, reason, func(received context.Context) (*routev1.RouteList, error) {
				if received != ctx {
					t.Fatal("list did not receive polling context")
				}
				list := &routev1.RouteList{}
				for _, route := range routes {
					observed := route.DeepCopy()
					observed.Spec.TLS = &routev1.TLSConfig{Key: "must-not-appear"}
					if test.match {
						observed.Status.Ingress = []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{{Type: routev1.RouteAdmitted, Reason: reason, Status: corev1.ConditionFalse}}}}
					}
					if !test.missing {
						list.Items = append(list.Items, *observed)
					}
				}
				return list, test.listError
			})
			if test.match {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			wantError := context.DeadlineExceeded
			if test.listError != nil {
				wantError = test.listError
			}
			if !errors.Is(err, wantError) {
				t.Fatalf("got %v, want %v", err, wantError)
			}
			for _, route := range routes {
				if !strings.Contains(err.Error(), "test-namespace/"+route.Name) {
					t.Errorf("missing route identity: %v", err)
				}
			}
			if strings.Contains(err.Error(), "must-not-appear") {
				t.Fatal("diagnostics exposed TLS key")
			}
			if !strings.Contains(err.Error(), "reason="+reason+" status=False") {
				t.Errorf("missing expectation: %v", err)
			}
		})
	}
}

// TestRouteStatusesConvergeTogether requires all six asynchronously updated routes to match before success.
func TestRouteStatusesConvergeTogether(t *testing.T) {
	t.Parallel()
	var routes []*routev1.Route
	for _, name := range []string{"route-0", "route-1", "route-2", "route-3", "route-4", "route-5"} {
		routes = append(routes, &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: name}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	err := waitForRouteStatuses(ctx, time.Millisecond, "test-namespace", routes, corev1.ConditionFalse, "ExternalCertificateValidationFailed", func(context.Context) (*routev1.RouteList, error) {
		calls++
		observed := &routev1.RouteList{}
		for index, route := range routes {
			updated := route.DeepCopy()
			if index < calls {
				updated.Status.Ingress = []routev1.RouteIngress{{Conditions: []routev1.RouteIngressCondition{{Type: routev1.RouteAdmitted, Status: corev1.ConditionFalse, Reason: "ExternalCertificateValidationFailed"}}}}
			}
			observed.Items = append(observed.Items, *updated)
		}
		return observed, nil
	})
	if err != nil || calls != len(routes) {
		t.Fatalf("got error %v after %d polls, want convergence after %d", err, calls, len(routes))
	}
}

// TestRouteStatusDiagnosticsPreserveLastObservation checks safe snapshots on timeout and subsequent API errors.
func TestRouteStatusDiagnosticsPreserveLastObservation(t *testing.T) {
	t.Parallel()
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-0", ResourceVersion: "123"},
		Spec: routev1.RouteSpec{
			Host: "sentinel-spec-host",
			TLS:  &routev1.TLSConfig{Certificate: "sentinel-certificate", Key: "sentinel-key", ExternalCertificate: &routev1.LocalObjectReference{Name: "sentinel-secret"}},
		},
		Status: routev1.RouteStatus{Ingress: []routev1.RouteIngress{{
			Host: "sentinel-ingress-host", RouterName: "sentinel-router-name", RouterCanonicalHostname: "sentinel-router-canonical-hostname",
			Conditions: []routev1.RouteIngressCondition{{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue, Reason: "ExternalCertificateSARCompleted", Message: "sentinel-condition-message"}},
		}}},
	}
	for _, test := range []struct {
		name     string
		apiError error
	}{
		{name: "timeout"},
		{name: "API error after observation", apiError: errors.New("list failed after observation")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			calls := 0
			err := waitForRouteStatuses(ctx, time.Millisecond, "test-namespace", []*routev1.Route{route}, corev1.ConditionFalse, "ExternalCertificateValidationFailed", func(context.Context) (*routev1.RouteList, error) {
				calls++
				if calls > 1 && test.apiError != nil {
					return nil, test.apiError
				}
				return &routev1.RouteList{Items: []routev1.Route{*route}}, nil
			})
			wantError := context.DeadlineExceeded
			if test.apiError != nil {
				wantError = test.apiError
			}
			if !errors.Is(err, wantError) {
				t.Fatalf("got %v, want wrapped %v", err, wantError)
			}
			for _, detail := range []string{"test-namespace/route-0", "resourceVersion=123", "type=Admitted", "reason=ExternalCertificateSARCompleted", "status=True"} {
				if !strings.Contains(err.Error(), detail) {
					t.Errorf("missing last observed detail %q: %v", detail, err)
				}
			}
			if strings.Contains(err.Error(), "sentinel-") {
				t.Errorf("diagnostics include a non-allowlisted field: %v", err)
			}
		})
	}
}

// TestEmptyRouteSetDoesNotPass prevents an empty expected-route set from satisfying the admission check.
func TestEmptyRouteSetDoesNotPass(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	err := waitForRouteStatuses(ctx, time.Millisecond, "test-namespace", nil, corev1.ConditionFalse, "ExternalCertificateValidationFailed", func(context.Context) (*routev1.RouteList, error) {
		cancel()
		return &routev1.RouteList{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation rather than success", err)
	}
}
