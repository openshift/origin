package compat_otp

import (
	"context"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func WaitForRouterInternalIP(oc *exutil.CLI) (string, error) {
	return waitForNamedRouterServiceIP(oc, "router-internal-default")
}

func waitForRouterExternalIP(oc *exutil.CLI) (string, error) {
	return waitForNamedRouterServiceIP(oc, "router-default")
}

func routerShouldHaveExternalService(oc *exutil.CLI) (bool, error) {
	foundLoadBalancerServiceStrategyType := false
	err := wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		ic, err := oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Get(context.Background(), "default", metav1.GetOptions{})
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

func WaitForRouterServiceIP(oc *exutil.CLI) (string, error) {
	if useExternal, err := routerShouldHaveExternalService(oc); err != nil {
		return "", err
	} else if useExternal {
		return waitForRouterExternalIP(oc)
	}
	return WaitForRouterInternalIP(oc)
}

func waitForNamedRouterServiceIP(oc *exutil.CLI, name string) (string, error) {
	_, ns, err := GetRouterPodTemplate(oc)
	if err != nil {
		return "", err
	}

	// wait for the service to show up
	var endpoint string
	err = wait.PollImmediate(2*time.Second, 60*time.Second, func() (bool, error) {
		svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get(context.Background(), name, metav1.GetOptions{})
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
