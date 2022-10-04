package ingress

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// ensureNodePortService ensures a NodePort service exists for a given
// ingresscontroller, if and only if one is desired.  Returns a Boolean
// indicating whether the NodePort service exists, the current NodePort service
// if it does exist, and an error value.
func (r *reconciler) ensureNodePortService(ic *operatorv1.IngressController, deploymentRef metav1.OwnerReference) (bool, *corev1.Service, error) {
	haveService, current, err := r.currentNodePortService(ic)
	if err != nil {
		return false, nil, err
	}

	// For compatibility, omit the "metrics" port iff the service already
	// exists and doesn't have a "metrics" port.  This serves two purposes:
	// (1) It avoids exhausting the nodeport range on upgrades if the
	// cluster has many nodeport services and few available nodeports.
	// (2) It enables the cluster administrator to remove the metrics port
	// from an existing nodeport service to avoid exposing the port.
	wantMetricsPort := false
	if !haveService {
		wantMetricsPort = true
	} else {
		for _, port := range current.Spec.Ports {
			if port.Name == "metrics" {
				wantMetricsPort = true
			}
		}
	}

	wantService, desired, err := desiredNodePortService(ic, deploymentRef, wantMetricsPort)
	if err != nil {
		return false, nil, err
	}

	// BZ2054200: Don't modify/delete services that are not directly owned by this controller.
	ownLBS := isServiceOwnedByIngressController(current, ic)

	switch {
	case !wantService && !haveService:
		return false, nil, nil
	case !wantService && haveService:
		if !ownLBS {
			return false, nil, fmt.Errorf("a conflicting nodeport service exists that is not owned by the ingress controller: %s", controller.LoadBalancerServiceName(ic))
		}
		if err := r.client.Delete(context.TODO(), current); err != nil {
			if !errors.IsNotFound(err) {
				return true, current, fmt.Errorf("failed to delete NodePort service: %v", err)
			}
		} else {
			log.Info("deleted NodePort service", "service", current)
		}
		return false, nil, nil
	case wantService && !haveService:
		if err := r.client.Create(context.TODO(), desired); err != nil {
			return false, nil, fmt.Errorf("failed to create NodePort service: %v", err)
		}
		log.Info("created NodePort service", "service", desired)
		return r.currentNodePortService(ic)
	case wantService && haveService:
		if !ownLBS {
			return false, nil, fmt.Errorf("a conflicting nodeport service exists that is not owned by the ingress controller: %s", controller.LoadBalancerServiceName(ic))
		}
		if updated, err := r.updateNodePortService(current, desired); err != nil {
			return true, current, fmt.Errorf("failed to update NodePort service: %v", err)
		} else if updated {
			return r.currentNodePortService(ic)
		}
	}

	return true, current, nil
}

// desiredNodePortService returns a Boolean indicating whether a NodePort
// service is desired, as well as the NodePort service if one is desired.
func desiredNodePortService(ic *operatorv1.IngressController, deploymentRef metav1.OwnerReference, wantMetricsPort bool) (bool, *corev1.Service, error) {
	if ic.Status.EndpointPublishingStrategy.Type != operatorv1.NodePortServiceStrategyType {
		return false, nil, nil
	}

	name := controller.NodePortServiceName(ic)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
			Namespace:   name.Namespace,
			Name:        name.Name,
			Labels: map[string]string{
				"app":                                  "router",
				"router":                               name.Name,
				manifests.OwningIngressControllerLabel: ic.Name,
			},
			OwnerReferences: []metav1.OwnerReference{deploymentRef},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(80),
					TargetPort: intstr.FromString("http"),
				},
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(443),
					TargetPort: intstr.FromString("https"),
				},
				{
					Name:       "metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(1936),
					TargetPort: intstr.FromString("metrics"),
				},
			},
			Selector: controller.IngressControllerDeploymentPodSelector(ic).MatchLabels,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
	if !wantMetricsPort {
		service.Spec.Ports = service.Spec.Ports[0:2]
	}

	if v, err := shouldUseLocalWithFallback(ic, service); err != nil {
		return true, service, err
	} else if v {
		service.Annotations[localWithFallbackAnnotation] = ""
	}

	return true, service, nil
}

// currentNodePortService returns a Boolean indicating whether a NodePort
// service exists for the given ingresscontroller, as well as the NodePort
// service if it does exist and an error value.
func (r *reconciler) currentNodePortService(ic *operatorv1.IngressController) (bool, *corev1.Service, error) {
	service := &corev1.Service{}
	if err := r.client.Get(context.TODO(), controller.NodePortServiceName(ic), service); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, service, nil
}

// updateNodePortService updates a NodePort service.  Returns a Boolean
// indicating whether the service was updated, and an error value.
func (r *reconciler) updateNodePortService(current, desired *corev1.Service) (bool, error) {
	changed, updated := nodePortServiceChanged(current, desired)
	if !changed {
		return false, nil
	}

	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		return false, err
	}
	log.Info("updated NodePort service", "namespace", updated.Namespace, "name", updated.Name, "diff", diff)
	return true, nil
}

// managedNodePortServiceAnnotations is a set of annotation keys for annotations
// that the operator manages for NodePort-type services.
var managedNodePortServiceAnnotations = sets.NewString(
	localWithFallbackAnnotation,
)

// nodePortServiceChanged checks if the current NodePort service spec matches
// the expected spec and if not returns an updated one.
func nodePortServiceChanged(current, expected *corev1.Service) (bool, *corev1.Service) {
	changed := false

	serviceCmpOpts := []cmp.Option{
		// Ignore fields that the API, other controllers, or user may
		// have modified.
		cmpopts.IgnoreFields(corev1.ServicePort{}, "NodePort"),
		cmpopts.IgnoreFields(corev1.ServiceSpec{}, "ClusterIP", "ClusterIPs", "ExternalIPs", "HealthCheckNodePort"),
		cmp.Comparer(cmpServiceAffinity),
		cmpopts.EquateEmpty(),
	}
	if !cmp.Equal(current.Spec, expected.Spec, serviceCmpOpts...) {
		changed = true
	}

	annotationCmpOpts := []cmp.Option{
		cmpopts.IgnoreMapEntries(func(k, _ string) bool {
			return !managedNodePortServiceAnnotations.Has(k)
		}),
	}
	if !cmp.Equal(current.Annotations, expected.Annotations, annotationCmpOpts...) {
		changed = true
	}

	if !changed {
		return false, nil
	}

	updated := current.DeepCopy()
	updated.Spec = expected.Spec

	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}
	for annotation := range managedNodePortServiceAnnotations {
		currentVal, have := current.Annotations[annotation]
		expectedVal, want := expected.Annotations[annotation]
		if want && (!have || currentVal != expectedVal) {
			updated.Annotations[annotation] = expected.Annotations[annotation]
		} else if have && !want {
			delete(updated.Annotations, annotation)
		}
	}
	// Preserve fields that the API, other controllers, or user may have
	// modified.
	updated.Spec.ClusterIP = current.Spec.ClusterIP
	updated.Spec.ExternalIPs = current.Spec.ExternalIPs
	updated.Spec.HealthCheckNodePort = current.Spec.HealthCheckNodePort
	for i, updatedPort := range updated.Spec.Ports {
		for _, currentPort := range current.Spec.Ports {
			if currentPort.Name == updatedPort.Name {
				updated.Spec.Ports[i].NodePort = currentPort.NodePort
			}
		}
	}

	return true, updated
}

func cmpServiceAffinity(a, b corev1.ServiceAffinity) bool {
	if len(a) == 0 {
		a = corev1.ServiceAffinityNone
	}
	if len(b) == 0 {
		b = corev1.ServiceAffinityNone
	}
	return a == b
}
