package util

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Define test waiting time const
const (
	defaultMaxWaitingTime = 200 * time.Second
	defaultPollingTime    = 2 * time.Second
)

// GetDeploymentPods gets the pods list of the deployment by labelSelector
func GetDeploymentPods(oc *CLI, deployName, namespace, labelSelector string) (*corev1.PodList, error) {
	return oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: ParseLabelsOrDie(labelSelector).String()})
}

// GetDeploymentTemplateAnnotations gets the deployment template annotations
func GetDeploymentTemplateAnnotations(oc *CLI, deployName, namespace string) map[string]string {
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context.Background(), deployName, metav1.GetOptions{})
	e2e.ExpectNoError(err)
	return deployment.Spec.Template.Annotations
}

// WaitForDeploymentReady waits for the deployment become ready
func WaitForDeploymentReady(oc *CLI, deployName, namespace string) error {
	var (
		deployment    *v1.Deployment
		labelSelector string
		getErr        error
	)
	pollErr := wait.PollUntilContextTimeout(context.Background(), defaultPollingTime, defaultMaxWaitingTime, true, func(context.Context) (isReady bool, err error) {
		deployment, getErr = oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context.Background(), deployName, metav1.GetOptions{})
		if getErr != nil {
			e2e.Logf("Unable to retrieve deployment %q:\n%v", deployName, getErr)
		}
		if deployment.Status.AvailableReplicas == *deployment.Spec.Replicas {
			e2e.Logf("Deployment %q is ready", deployName)
			return true, nil
		}
		e2e.Logf("Deployment %q is still unready, available replicas %d/%d", deployName, deployment.Status.AvailableReplicas, *deployment.Spec.Replicas)
		return false, nil
	})

	if pollErr != nil {
		e2e.Logf("Waiting for deployment %s ready timeout", deployName)
		for key, value := range deployment.Spec.Selector.MatchLabels {
			labelSelector = fmt.Sprintf("%s=%s", key, value)
			break
		}
		DumpDeploymentPodsLogs(oc, deployName, namespace, labelSelector)
	}
	return pollErr
}

// DumpDeploymentPodsLogs will dump the deployment pods logs for a deployment for debug purposes
func DumpDeploymentPodsLogs(oc *CLI, deployName, namespace, labelSelector string) {
	e2e.Logf("Dumping deployment/%s pods logs", deployName)

	pods, err := GetDeploymentPods(oc, deployName, namespace, labelSelector)
	if err != nil {
		e2e.Logf("Unable to retrieve pods for deployment %q:\n%v", deployName, err)
		return
	}

	DumpPodLogs(pods.Items, oc)
}
