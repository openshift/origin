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

func GetDeploymentRSPodTemplateHash(oc *CLI, deployName, namespace string, revision int64) (string, error) {
	rsList, err := oc.AdminKubeClient().AppsV1().ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: ParseLabelsOrDie(fmt.Sprintf("name=%s", deployName)).String()})
	if err != nil {
		return "", err
	}
	var rsObj *v1.ReplicaSet
	for _, rs := range rsList.Items {
		if rs.Annotations["deployment.kubernetes.io/revision"] == fmt.Sprintf("%d", revision) {
			item := rs
			rsObj = &item
		}
	}
	if rsObj == nil {
		return "", fmt.Errorf("Unable to find replicat set with 'deployment.kubernetes.io/revision=%v' annotation", revision)
	}

	return rsObj.Labels["pod-template-hash"], nil
}

// WaitForDeploymentReady waits for the deployment become ready
func WaitForDeploymentReady(oc *CLI, deployName, namespace string, revision int64) error {
	return WaitForDeploymentReadyWithTimeout(oc, deployName, namespace, revision, defaultMaxWaitingTime)
}

// WaitForDeploymentReadyWithTimeout waits for the deployment become ready with defined timeout
func WaitForDeploymentReadyWithTimeout(oc *CLI, deployName, namespace string, revision int64, timeout time.Duration) error {
	var (
		deployment    *v1.Deployment
		labelSelector string
		getErr        error
	)
	pollErr := wait.PollUntilContextTimeout(context.Background(), defaultPollingTime, timeout, true, func(context.Context) (isReady bool, err error) {
		deployment, getErr = oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context.Background(), deployName, metav1.GetOptions{})
		if getErr != nil {
			e2e.Logf("Unable to retrieve deployment %q:\n%v", deployName, getErr)
			return false, nil
		}
		if deployment.Status.AvailableReplicas == *deployment.Spec.Replicas {
			descOutput, err := oc.AsAdmin().Run("describe").WithoutNamespace().Args("deployment/"+deployment.Name, "-n", deployment.Namespace).Output()
			e2e.Logf("Deployment %q is ready", deployName)
			if err != nil {
				e2e.Logf("Failed to describe the deployment %q", deployName)
			} else {
				e2e.Logf("Describing deployment %s/%s\n%s\n\n:", deployment.Name, deployment.Namespace, descOutput)
			}
			return true, nil
		}
		if revision >= 0 && deployment.Status.ObservedGeneration != revision {
			e2e.Logf("Unexpected observed generation: %d, expected %d", deployment.Status.ObservedGeneration, revision)
			return false, nil
		}
		e2e.Logf("Deployment %q is still unready, available replicas %d/%d, observed generation %d", deployName, deployment.Status.AvailableReplicas, *deployment.Spec.Replicas, deployment.Status.ObservedGeneration)
		return false, nil
	})

	if pollErr != nil {
		e2e.Logf("Waiting for deployment %s ready timeout", deployName)
		if deployment != nil && deployment.Spec.Selector != nil {
			for key, value := range deployment.Spec.Selector.MatchLabels {
				labelSelector = fmt.Sprintf("%s=%s", key, value)
				break
			}
			DumpDeploymentPodsLogs(oc, deployName, namespace, labelSelector)
		}
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
