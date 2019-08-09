package util

import (
	"time"

	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func WaitForRouterInternalIP(oc *CLI) (string, error) {
	return waitForNamedRouterServiceIP(oc, "router-internal-default")
}

func routerShouldHaveExternalService(oc *CLI) (bool, error) {
	foundLoadBalancerServiceStrategyType := false
	err := wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		ic, err := oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Get("default", metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			return false, nil
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if ic.Status.EndpointPublishingStrategy == nil {
			return false, nil
		}
		if ic.Status.EndpointPublishingStrategy.Type == "LoadBalancerService" {
			foundLoadBalancerServiceStrategyType = true
		}
		return true, nil
	})
	return foundLoadBalancerServiceStrategyType, err
}

func WaitForDefaultIngressControllerRoutableEndpoint(oc *CLI) (string, error) {
	if useExternal, err := routerShouldHaveExternalService(oc); err != nil {
		return "", err
	} else if useExternal {
		return waitForNamedRouterServiceIP(oc, "router-default")
	}
	return WaitForRouterInternalIP(oc)
}

func waitForNamedRouterServiceIP(oc *CLI, name string) (string, error) {
	_, ns, err := GetRouterPodTemplate(oc)
	if err != nil {
		return "", err
	}

	// wait for the service to show up
	var endpoint string
	err = wait.PollImmediate(2*time.Second, 60*time.Second, func() (bool, error) {
		svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get(name, metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			return false, nil
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if len(svc.Status.LoadBalancer.Ingress) != 0 {
				if len(svc.Status.LoadBalancer.Ingress[0].IP) != 0 {
					endpoint = svc.Status.LoadBalancer.Ingress[0].IP
					return true, nil
				}
				if len(svc.Status.LoadBalancer.Ingress[0].Hostname) != 0 {
					endpoint = svc.Status.LoadBalancer.Ingress[0].Hostname
					return true, nil
				}
			}
			return false, nil
		}
		endpoint = svc.Spec.ClusterIP
		return true, nil
	})
	return endpoint, err
}

// WaitForDefaultIngressControllerEndpoints will return Endpoints for the default
// ingresscontroller once it exists and contains any number of addresses.
//
// Endpoints should be used for any test which needs to communicate with a specific
// ingress controller (rather than behind the load-balanced abstraction of a service).
func WaitForDefaultIngressControllerEndpoints(oc *CLI) (*corev1.Endpoints, error) {
	var endpoints *corev1.Endpoints

	err := wait.PollImmediate(2*time.Second, 60*time.Second, func() (bool, error) {
		ep, err := oc.AdminKubeClient().CoreV1().Endpoints("openshift-ingress").Get("router-default", metav1.GetOptions{})
		if err != nil {
			if kapierrs.IsNotFound(err) {
				return false, nil
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		if len(ep.Subsets) > 0 && len(ep.Subsets[0].Addresses) > 0 {
			endpoints = ep
			return true, nil
		}
		return false, nil
	})

	o.Expect(err).NotTo(o.HaveOccurred())
	return endpoints, err
}
