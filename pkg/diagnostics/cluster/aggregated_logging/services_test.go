package aggregated_logging

import (
	"errors"
	"testing"

	"github.com/openshift/origin/pkg/diagnostics/log"
	kapi "k8s.io/kubernetes/pkg/api"
)

type fakeServicesDiagnostic struct {
	list kapi.ServiceList
	fakeDiagnostic
	endpoints   map[string]kapi.Endpoints
	endpointErr error
}

func newFakeServicesDiagnostic(t *testing.T) *fakeServicesDiagnostic {
	return &fakeServicesDiagnostic{
		list:           kapi.ServiceList{},
		fakeDiagnostic: *newFakeDiagnostic(t),
		endpoints:      map[string]kapi.Endpoints{},
	}
}

func (f *fakeServicesDiagnostic) services(project string, options kapi.ListOptions) (*kapi.ServiceList, error) {
	if f.err != nil {
		return &f.list, f.err
	}
	return &f.list, nil
}
func (f *fakeServicesDiagnostic) endpointsForService(project string, service string) (*kapi.Endpoints, error) {
	if f.endpointErr != nil {
		return nil, f.endpointErr
	}
	endpoints, _ := f.endpoints[service]
	return &endpoints, nil
}

func (f *fakeServicesDiagnostic) addEndpointSubsetTo(service string) {
	endpoints := kapi.Endpoints{}
	endpoints.Subsets = []kapi.EndpointSubset{{}}
	f.endpoints[service] = endpoints
}

func (f *fakeServicesDiagnostic) addServiceNamed(name string) {
	meta := kapi.ObjectMeta{Name: name}
	f.list.Items = append(f.list.Items, kapi.Service{ObjectMeta: meta})
}

// test error from client
func TestCheckingServicesWhenFailedResponseFromClient(t *testing.T) {
	d := newFakeServicesDiagnostic(t)
	d.err = errors.New("an error")
	checkServices(d, d, fakeProject)
	d.assertMessage("AGL0205",
		"Exp an error when unable to retrieve services because of a client error",
		log.ErrorLevel)
}

func TestCheckingServicesWhenMissingServices(t *testing.T) {
	d := newFakeServicesDiagnostic(t)
	d.addServiceNamed("logging-es")

	checkServices(d, d, fakeProject)
	d.assertMessage("AGL0215",
		"Exp an warning when an expected sercies is not found",
		log.WarnLevel)
}

func TestCheckingServicesWarnsWhenRetrievingEndpointsErrors(t *testing.T) {
	d := newFakeServicesDiagnostic(t)
	d.addServiceNamed("logging-es")
	d.endpointErr = errors.New("an endpoint error")

	checkServices(d, d, fakeProject)
	d.assertMessage("AGL0220",
		"Exp a warning when there is an error retrieving endpoints for a service",
		log.WarnLevel)
}

func TestCheckingServicesWarnsWhenServiceHasNoEndpoints(t *testing.T) {
	d := newFakeServicesDiagnostic(t)
	for _, service := range loggingServices.List() {
		d.addServiceNamed(service)
	}

	checkServices(d, d, fakeProject)
	d.assertMessage("AGL0225",
		"Exp a warning when an expected service has no endpoints",
		log.WarnLevel)
}

func TestCheckingServicesHasNoErrorsOrWarningsForExpServices(t *testing.T) {
	d := newFakeServicesDiagnostic(t)
	for _, service := range loggingServices.List() {
		d.addServiceNamed(service)
		d.addEndpointSubsetTo(service)
	}

	checkServices(d, d, fakeProject)
	d.assertNoErrors()
	d.assertNoWarnings()
}
