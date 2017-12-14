package aggregated_logging

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	routesapi "github.com/openshift/origin/pkg/route/apis/route"
)

const (
	testRoutesKey = "routes"
)

type fakeRoutesDiagnostic struct {
	fakeDiagnostic
	fakeRoutes   routesapi.RouteList
	clienterrors map[string]error
}

func newFakeRoutesDiagnostic(t *testing.T) *fakeRoutesDiagnostic {
	return &fakeRoutesDiagnostic{
		fakeDiagnostic: *newFakeDiagnostic(t),
		clienterrors:   map[string]error{},
	}
}

func (f *fakeRoutesDiagnostic) addRouteWith(condType routesapi.RouteIngressConditionType, condStatus kapi.ConditionStatus, cert string, key string) {
	ingress := routesapi.RouteIngress{
		Conditions: []routesapi.RouteIngressCondition{
			{
				Type:   condType,
				Status: condStatus,
			},
		},
	}
	route := routesapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "aname"},
		Status: routesapi.RouteStatus{
			Ingress: []routesapi.RouteIngress{ingress},
		},
	}
	if len(cert) != 0 && len(key) != 0 {
		tls := routesapi.TLSConfig{
			Certificate: cert,
			Key:         key,
		}
		route.Spec.TLS = &tls
	}
	f.fakeRoutes.Items = append(f.fakeRoutes.Items, route)
}

func (f *fakeRoutesDiagnostic) routes(project string, options metav1.ListOptions) (*routesapi.RouteList, error) {
	value, ok := f.clienterrors[testRoutesKey]
	if ok {
		return nil, value
	}
	return &f.fakeRoutes, nil
}

func TestRouteWhenErrorFromClient(t *testing.T) {
	d := newFakeRoutesDiagnostic(t)
	d.clienterrors[testRoutesKey] = errors.New("some client error")

	checkRoutes(d, d, fakeProject)
	d.assertMessage("AGL0305", "Exp an error when there is a client error retrieving routes", log.ErrorLevel)
	d.dumpMessages()
}

func TestRouteWhenZeroRoutesAvailable(t *testing.T) {
	d := newFakeRoutesDiagnostic(t)

	checkRoutes(d, d, fakeProject)
	d.assertMessage("AGL0310", "Exp an error when there are no routes to support logging", log.ErrorLevel)
	d.dumpMessages()
}

// Test error route != accepted
func TestRouteWhenRouteNotAccepted(t *testing.T) {
	d := newFakeRoutesDiagnostic(t)
	d.addRouteWith(routesapi.RouteExtendedValidationFailed, kapi.ConditionTrue, "", "")

	checkRoutes(d, d, fakeProject)
	d.assertMessage("AGL0325", "Exp an error when a route was not accepted", log.ErrorLevel)
	d.assertMessage("AGL0331", "Exp to skip the cert check since none specified", log.DebugLevel)
	d.dumpMessages()
}
func TestRouteWhenRouteAccepted(t *testing.T) {
	d := newFakeRoutesDiagnostic(t)
	d.addRouteWith(routesapi.RouteAdmitted, kapi.ConditionTrue, "", "")

	checkRoutes(d, d, fakeProject)
	d.assertNoErrors()
	d.dumpMessages()
}

func TestRouteWhenErrorDecodingCert(t *testing.T) {
	d := newFakeRoutesDiagnostic(t)
	d.addRouteWith(routesapi.RouteExtendedValidationFailed, kapi.ConditionTrue, "cert", "key")

	checkRoutes(d, d, fakeProject)
	d.assertMessage("AGL0350", "Exp an error when unable to decode cert", log.ErrorLevel)
	d.dumpMessages()
}

func TestRouteWhenErrorParsingCert(t *testing.T) {
	d := newFakeRoutesDiagnostic(t)
	d.addRouteWith(routesapi.RouteExtendedValidationFailed, kapi.ConditionTrue, "cert", "key")

	checkRoutes(d, d, fakeProject)
	d.assertMessage("AGL0350", "Exp an error when unable to decode cert", log.ErrorLevel)
	d.dumpMessages()
}
