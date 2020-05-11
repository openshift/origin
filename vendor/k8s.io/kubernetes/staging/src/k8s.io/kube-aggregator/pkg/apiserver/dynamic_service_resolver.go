package apiserver

import (
	"fmt"
	"net/url"
	"sort"

	failuredetector "github.com/p0lyn0mial/failure-detector"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/util/proxy"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

// FailureDetector helps to assess the health conditions of endpoints.
//
// It does that by providing `Collector()` method for collecting samples and `XYZ()` for querying the current health of endpoints for a given service.
type FailureDetector interface {
	// Collector exposes a channel for collecting EndpointSamples
	Collector() chan<- *failuredetector.EndpointSample

	// EndpointStatus returns the current status of the given endpoint for the given service
	EndpointStatus(namespace, service string, url *url.URL) (isHealthy bool, weight float32)
}

// NewEndpointServiceResolverWithFailureDetector returns a service resolver with support for retry mechanisms, health reporting and failure detection
func NewEndpointServiceResolverWithFailureDetector(services listersv1.ServiceLister, endpoints listersv1.EndpointsLister, failureDetector FailureDetector) ServiceResolver {
	return &serviceResolver{
		services:        services,
		endpoints:       endpoints,
		failureDetector: failureDetector,
	}
}

type serviceResolver struct {
	services        listersv1.ServiceLister
	endpoints       listersv1.EndpointsLister
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
	potentialEndpoints, err := r.remainingEndpoints(namespace, name, port, visitedEPs)
	if err != nil {
		return nil, err
	}
	return r.resolveEndpointHealthyPriority(namespace, name, potentialEndpoints)
}

// Collector exposes a channel for sending EndpointSamples for further processing and evaluation.
func (r *serviceResolver) Collector() chan<- *failuredetector.EndpointSample {
	return r.failureDetector.Collector()
}

// remainingEndpoints gets all remaining end points for the given service except already visited ones
func (r *serviceResolver) remainingEndpoints(namespace, name string, port int32, visitedEPs []*url.URL) ([]*url.URL, error) {
	allNewEndpoints := []*url.URL{}

	for {
		// TODO: in the future simply list all EP instead of using ResolveEndpoint which assigns an EP randomly
		newEndpoint, err := proxy.ResolveEndpoint(r.services, r.endpoints, namespace, name, port, visitedEPs...)
		if err != nil {
			break
		}
		allNewEndpoints = append(allNewEndpoints, newEndpoint)
		visitedEPs = append(visitedEPs, newEndpoint)
	}

	if len(allNewEndpoints) == 0 {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("no endpoints available for service %q/%q", namespace, name))
	}

	return allNewEndpoints, nil
}

// weightedEndpoint a helper struct for keeping weight and url together
type weightedEndpoint struct {
	url    *url.URL
	weight float32
}

// weightedEndpointsByPriority a helper type for sorting weightedEndpoint by priority
type weightedEndpointsByPriority []*weightedEndpoint

func (wep weightedEndpointsByPriority) Len() int           { return len(wep) }
func (wep weightedEndpointsByPriority) Swap(i, j int)      { wep[i], wep[j] = wep[j], wep[i] }
func (wep weightedEndpointsByPriority) Less(i, j int) bool { return wep[i].weight > wep[j].weight }

// resolveEndpointHealthyPriority pick the best endpoint for the given service from potentialEndpoints
// it does that by querying failure detector, removing unhealthy endpoints and sorting the remaining by priority
func (r *serviceResolver) resolveEndpointHealthyPriority(namespace, name string, potentialEndpoints []*url.URL) (*url.URL, error) {
	healthyWeightedEndpoints := weightedEndpointsByPriority{}

	for _, endpoint := range potentialEndpoints {
		isHealthy, weight := r.failureDetector.EndpointStatus(namespace, name, endpoint)
		if isHealthy {
			healthyWeightedEndpoints = append(healthyWeightedEndpoints, &weightedEndpoint{url: endpoint, weight: weight})
		}
	}

	sort.Sort(healthyWeightedEndpoints)

	if len(healthyWeightedEndpoints) == 0 {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("no endpoints available for service %q/%q", namespace, name))
	}
	return healthyWeightedEndpoints[0].url, nil
}
