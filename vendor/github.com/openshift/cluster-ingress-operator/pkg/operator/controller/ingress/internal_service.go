package ingress

import (
	"context"
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Annotation used to inform the certificate generation service to
	// generate a cluster-signed certificate and populate the secret.
	ServingCertSecretAnnotation = "service.alpha.openshift.io/serving-cert-secret-name"
)

// ensureInternalRouterServiceForIngress ensures that an internal service exists
// for a given IngressController.
func (r *reconciler) ensureInternalIngressControllerService(ic *operatorv1.IngressController, deploymentRef metav1.OwnerReference) (*corev1.Service, error) {
	desired := desiredInternalIngressControllerService(ic, deploymentRef)
	current, err := r.currentInternalIngressControllerService(ic)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return current, nil
	}

	if err := r.client.Create(context.TODO(), desired); err != nil {
		return nil, fmt.Errorf("failed to create internal ingresscontroller service: %v", err)
	}
	log.Info("created internal ingresscontroller service", "service", desired)
	return desired, nil
}

func (r *reconciler) currentInternalIngressControllerService(ic *operatorv1.IngressController) (*corev1.Service, error) {
	current := &corev1.Service{}
	err := r.client.Get(context.TODO(), controller.InternalIngressControllerServiceName(ic), current)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return current, nil
}

func desiredInternalIngressControllerService(ic *operatorv1.IngressController, deploymentRef metav1.OwnerReference) *corev1.Service {
	s := manifests.InternalIngressControllerService()

	name := controller.InternalIngressControllerServiceName(ic)

	s.Namespace = name.Namespace
	s.Name = name.Name

	s.Labels = map[string]string{
		manifests.OwningIngressControllerLabel: ic.Name,
	}

	s.Annotations = map[string]string{
		// TODO: remove hard-coded name
		ServingCertSecretAnnotation: fmt.Sprintf("router-metrics-certs-%s", ic.Name),
	}

	s.Spec.Selector = controller.IngressControllerDeploymentPodSelector(ic).MatchLabels

	s.SetOwnerReferences([]metav1.OwnerReference{deploymentRef})

	return s
}
