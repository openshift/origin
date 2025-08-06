package compat_otp

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/utils/format"
	"k8s.io/utils/ptr"
)

// WaitForDeploymentsReady polls listDeployments() until deployments obtained are ready
func WaitForDeploymentsReady(ctx context.Context, listDeployments func(ctx context.Context) (*appsv1.DeploymentList, error),
	isDeploymentReady func(*appsv1.Deployment) bool, timeout, interval time.Duration, printDebugInfo bool) {
	g.GinkgoHelper()
	e2e.Logf("Waiting for deployments to be ready")
	o.Eventually(func() bool {
		deployList, err := listDeployments(ctx)
		if err != nil {
			e2e.Logf("Error listing deployments: %v, keep polling", err)
			return false
		}
		if len(deployList.Items) == 0 {
			e2e.Logf("No deployments found, keep polling")
			return false
		}
		for _, deploy := range deployList.Items {
			e2e.Logf("Waiting for deployment %s", deploy.Name)
			if isDeploymentReady(&deploy) {
				continue
			}
			e2e.Logf("Deployment/%v is not ready, keep polling", deploy.Name)
			if printDebugInfo {
				e2e.Logf("Deployment status:\n%s", format.Object(deploy.Status, 0))
			}
			return false
		}
		return true
	}).WithTimeout(timeout).WithPolling(interval).WithContext(ctx).Should(o.BeTrue(), "Failed waiting for deployments to be ready")
	e2e.Logf("Deployments are ready")
}

// IsDeploymentReady checks if an *appsv1.Deployment is ready
func IsDeploymentReady(deploy *appsv1.Deployment) bool {
	expectedReplicas := ptr.Deref[int32](deploy.Spec.Replicas, -1)
	return expectedReplicas == deploy.Status.AvailableReplicas &&
		expectedReplicas == deploy.Status.UpdatedReplicas &&
		expectedReplicas == deploy.Status.ReadyReplicas &&
		deploy.Generation <= deploy.Status.ObservedGeneration &&
		deploy.Status.UnavailableReplicas == 0
}
