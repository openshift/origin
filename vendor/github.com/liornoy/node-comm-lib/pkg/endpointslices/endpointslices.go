package endpointslices

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type QueryBuilder interface {
	Query() []discoveryv1.EndpointSlice
	WithLabels(labels map[string]string) QueryBuilder
	WithHostNetwork() QueryBuilder
	WithServiceType(serviceType corev1.ServiceType) QueryBuilder
}

type QueryParams struct {
	client.Client
	pods     []corev1.Pod
	filter   []bool
	epSlices []discoveryv1.EndpointSlice
	services []corev1.Service
}

func NewQuery(c client.Client) (*QueryParams, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}

	var (
		epSlicesList discoveryv1.EndpointSliceList
		servicesList corev1.ServiceList
		podsList     corev1.PodList
	)
	err := c.List(context.TODO(), &epSlicesList, &client.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list endpointslices: %w", err)
	}

	err = c.List(context.TODO(), &servicesList, &client.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	err = c.List(context.TODO(), &podsList, &client.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	ret := QueryParams{
		Client:   c,
		epSlices: epSlicesList.Items,
		services: servicesList.Items,
		pods:     podsList.Items,
		filter:   make([]bool, len(epSlicesList.Items))}

	return &ret, nil
}

func (q *QueryParams) Query() []discoveryv1.EndpointSlice {
	ret := make([]discoveryv1.EndpointSlice, 0)

	for i, filter := range q.filter {
		if filter {
			ret = append(ret, q.epSlices[i])
		}
	}

	return ret
}

func (q *QueryParams) WithLabels(labels map[string]string) QueryBuilder {
	for i, epSlice := range q.epSlices {
		if q.withLabels(epSlice, labels) {
			q.filter[i] = true
		}
	}

	return q
}

func (q *QueryParams) WithHostNetwork() QueryBuilder {
	for i, epSlice := range q.epSlices {
		if q.withHostNetwork(epSlice) {
			q.filter[i] = true
		}
	}

	return q
}

func (q *QueryParams) WithServiceType(serviceType corev1.ServiceType) QueryBuilder {
	for i, epSlice := range q.epSlices {
		if q.withServiceType(epSlice, serviceType) {
			q.filter[i] = true
		}
	}

	return q
}

func (q *QueryParams) withLabels(epSlice discoveryv1.EndpointSlice, labels map[string]string) bool {
	for key, value := range labels {
		if mValue, ok := epSlice.Labels[key]; !ok || mValue != value {
			return false
		}
	}

	return true
}

func (q *QueryParams) withServiceType(epSlice discoveryv1.EndpointSlice, serviceType corev1.ServiceType) bool {
	if len(epSlice.OwnerReferences) == 0 {
		return false
	}

	for _, ownerRef := range epSlice.OwnerReferences {
		name := ownerRef.Name
		namespace := epSlice.Namespace
		service := getService(name, namespace, q.services)
		if service == nil {
			continue
		}
		if service.Spec.Type == serviceType {
			return true
		}
	}

	return false
}

func (q *QueryParams) withHostNetwork(epSlice discoveryv1.EndpointSlice) bool {
	if len(epSlice.Endpoints) == 0 {
		return false
	}

	for _, endpoint := range epSlice.Endpoints {
		if endpoint.TargetRef == nil {
			continue
		}
		name := endpoint.TargetRef.Name
		namespace := endpoint.TargetRef.Namespace
		pod := getPod(name, namespace, q.pods)
		if pod == nil {
			continue
		}
		if pod.Spec.HostNetwork {
			return true
		}
	}

	return false
}

func getPod(name, namespace string, pods []corev1.Pod) *corev1.Pod {
	for i, p := range pods {
		if p.Name == name && p.Namespace == namespace {
			return &pods[i]
		}
	}

	return nil
}

func getService(name, namespace string, services []corev1.Service) *corev1.Service {
	for i, service := range services {
		if service.Name == name && service.Namespace == namespace {
			return &services[i]
		}
	}

	return nil
}
