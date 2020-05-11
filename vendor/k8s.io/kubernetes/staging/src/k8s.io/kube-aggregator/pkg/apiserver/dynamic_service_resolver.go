package apiserver

import (
	"net/url"

	failuredetector "github.com/p0lyn0mial/failure-detector"

	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/apiserver/pkg/util/proxy"
)

// FailureDetector helps to assess the health conditions of endpoints.
//
// It does that by providing `Collector()` method for collecting samples and `XYZ()` for querying the current health of endpoints for a given service.
type FailureDetector interface {
	// Collector exposes a channel for collecting EndpointSamples
	Collector() chan<- *failuredetector.EndpointSample
}

// NewEndpointServiceResolverWithFailureDetector returns a service resolver with support for retry mechanisms, health reporting and failure detection
func NewEndpointServiceResolverWithFailureDetector(services listersv1.ServiceLister, endpoints listersv1.EndpointsLister, failureDetector FailureDetector) ServiceResolver {
	return &serviceResolver{
		services:  services,
		endpoints: endpoints,
		failureDetector: failureDetector,
	}
}

type serviceResolver struct {
	services  listersv1.ServiceLister
	endpoints listersv1.EndpointsLister
	failureDetector FailureDetector
}

// ResolveEndpoint resolves (randomly) an endpoint to a given service.
//
// Note:
// Kube uses one service resolver for webhooks and the aggregator this method satisfies webhook.ServiceResolver interface
func (r *serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	return proxy.ResolveEndpoint(r.services, r.endpoints, namespace, name, port)
}

// ResolveEndpointWithVisited resolves an endpoint excluding already visited ones.
// Facilitates supporting retry mechanisms.
func (r *serviceResolver) ResolveEndpointWithVisited(namespace, name string, port int32, visitedEPs []*url.URL) (*url.URL, error) {
	// TODO:
	// 1. query failureDetector to get the current list of EPs for the given service
	// 2. exclude already visited ones
	// 3. get all possible EPs for the service from the service resolver excluding already visited ones
	// 4. assign weights to EPs from 3 based on the input returned from 2
	// 5. sort the list from 4 by weight - new EP should get the weight equal 1 (indicates a healthy condition)
	// 6. pick one EP randomly taking EPs's weights into account from the list obtained in 5
	// 7. return EP from 6
	return proxy.ResolveEndpoint(r.services, r.endpoints, namespace, name, port, visitedEPs...)
}

// Collector exposes a channel for sending EndpointSamples for further processing and evaluation.
func (r *serviceResolver) Collector() chan <- *failuredetector.EndpointSample {
	return r.failureDetector.Collector()
}
