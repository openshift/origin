package apiserver

import (
	"net/url"
	"testing"

	failuredetector "github.com/p0lyn0mial/failure-detector"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestEndpointServiceResolverWithFailureDetector(t *testing.T) {
	scenarios := []struct {
		name           string
		services       []*v1.Service
		endpoints      func(svc *v1.Service) []*v1.Endpoints
		seenURLs       []*url.URL
		fakeEndpoints  []*fakeWeightedEndpoint
		expectedValues expectation
	}{
		{
			name:      "an EP with the highest weight wins - no seenURLS",
			services:  defaultServices(),
			endpoints: matchingEndpoints,
			fakeEndpoints: []*fakeWeightedEndpoint{
				{url: "https://192.168.1.1:1443", weight: 0.8, healthy: true},
				{url: "https://192.168.1.2:1443", weight: 0.7, healthy: true},
				{url: "https://192.168.1.3:1443", weight: 0.9, healthy: true},
			},
			expectedValues: expectation{"https://192.168.1.3:1443", false},
		},

		{
			name:      "an EP with the highest weight wins - with seenURLS",
			services:  defaultServices(),
			endpoints: matchingEndpoints,
			seenURLs:  []*url.URL{{Scheme: "https", Host: "192.168.1.3:1443"}},
			fakeEndpoints: []*fakeWeightedEndpoint{
				{url: "https://192.168.1.1:1443", weight: 0.8, healthy: true},
				{url: "https://192.168.1.2:1443", weight: 0.7, healthy: true},
				{url: "https://192.168.1.3:1443", weight: 0.9, healthy: true},
			},
			expectedValues: expectation{"https://192.168.1.1:1443", false},
		},

		{
			name:      "a healthy EP with the highest weight wins - no seenURLS",
			services:  defaultServices(),
			endpoints: matchingEndpoints,
			fakeEndpoints: []*fakeWeightedEndpoint{
				{url: "https://192.168.1.1:1443", weight: 0.8, healthy: true},
				{url: "https://192.168.1.2:1443", weight: 0.7, healthy: true},
				{url: "https://192.168.1.3:1443", weight: 0.9, healthy: false},
			},
			expectedValues: expectation{"https://192.168.1.1:1443", false},
		},

		{
			name:      "no healthy endpoints",
			services:  defaultServices(),
			endpoints: matchingEndpoints,
			fakeEndpoints: []*fakeWeightedEndpoint{
				{url: "https://192.168.1.1:1443", weight: 0.8, healthy: false},
				{url: "https://192.168.1.2:1443", weight: 0.7, healthy: false},
				{url: "https://192.168.1.3:1443", weight: 0.9, healthy: false},
			},
			expectedValues: expectation{"", true},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {

			serviceCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			serviceLister := v1listers.NewServiceLister(serviceCache)
			for i := range scenario.services {
				if err := serviceCache.Add(scenario.services[i]); err != nil {
					t.Fatalf("%s unexpected service add error: %v", scenario.name, err)
				}
			}

			endpointCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			endpointLister := v1listers.NewEndpointsLister(endpointCache)
			if scenario.endpoints != nil {
				for _, svc := range scenario.services {
					for _, ep := range scenario.endpoints(svc) {
						if err := endpointCache.Add(ep); err != nil {
							t.Fatalf("%s unexpected endpoint add error: %v", scenario.name, err)
						}
					}
				}
			}

			target := &serviceResolver{serviceLister, endpointLister, &fakeFailureDetector{scenario.fakeEndpoints}}
			url, err := target.ResolveEndpointWithVisited("one", "alfa", 443, scenario.seenURLs)
			switch {
			case err == nil && scenario.expectedValues.error:
				t.Error("expected an error, got none")
			case err != nil && scenario.expectedValues.error:
				// ignore
			case err != nil:
				t.Errorf("unexpected error: %v", err)
			case scenario.expectedValues.url != url.String():
				t.Fatalf("unexpected %q URL returned", url.String())
			}
		})
	}
}

func defaultServices() []*v1.Service {
	return []*v1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
			Spec: v1.ServiceSpec{
				Type:      v1.ServiceTypeClusterIP,
				ClusterIP: "hit",
				Ports: []v1.ServicePort{
					{Name: "https", Port: 443, TargetPort: intstr.FromInt(1443)},
					{Port: 1234, TargetPort: intstr.FromInt(1234)},
				},
			},
		},
	}
}

type expectation struct {
	url   string
	error bool
}

func matchingEndpoints(svc *v1.Service) []*v1.Endpoints {
	ports := []v1.EndpointPort{}
	for _, p := range svc.Spec.Ports {
		if p.TargetPort.Type != intstr.Int {
			continue
		}
		ports = append(ports, v1.EndpointPort{Name: p.Name, Port: p.TargetPort.IntVal})
	}

	return []*v1.Endpoints{{
		ObjectMeta: metav1.ObjectMeta{Namespace: svc.Namespace, Name: svc.Name},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{Hostname: "dummy-host-1", IP: "192.168.1.1"},
					{Hostname: "dummy-host-2", IP: "192.168.1.2"},
					{Hostname: "dummy-host-3", IP: "192.168.1.3"},
				},
				Ports: ports,
			},
		},
	}}
}

type fakeWeightedEndpoint struct {
	url     string
	healthy bool
	weight  float32
}

type fakeFailureDetector struct {
	fakeWeightedEndpoints []*fakeWeightedEndpoint
}

func (fd *fakeFailureDetector) Collector() chan<- *failuredetector.EndpointSample {
	return nil
}

func (fd *fakeFailureDetector) EndpointStatus(namespace, service string, url *url.URL) (isHealthy bool, weight float32) {
	for _, fakeEndpoint := range fd.fakeWeightedEndpoints {
		if fakeEndpoint.url == url.String() {
			return fakeEndpoint.healthy, fakeEndpoint.weight
		}
	}

	return true, 1.0
}
