package router

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
)

const (
	dcmDiagnosticsTimeout       = 20 * time.Second
	dcmDiagnosticRequestTimeout = 5 * time.Second
	dcmDiagnosticMaxPods        = 3
	dcmDiagnosticMaxContainers  = 4
	dcmDiagnosticLogBytes       = 16 * 1024
	dcmDiagnosticLogLines       = 100
	dcmRouterControllerLabel    = "ingresscontroller.operator.openshift.io/deployment-ingresscontroller"
)

// cleanupDCMIngressController captures failure evidence before deleting the test controller.
func cleanupDCMIngressController(ctx context.Context, kubeClient kubernetes.Interface, operatorClient operatorclient.OperatorV1Interface, controller types.NamespacedName, failed bool) error {
	if controller.Name == "" {
		return nil
	}
	if failed {
		framework.Logf("%s", collectDCMRouterDiagnostics(ctx, kubeClient, controller.Name))
	}
	return operatorClient.IngressControllers(controller.Namespace).Delete(ctx, controller.Name, *metav1.NewDeleteOptions(1))
}

// collectDCMRouterDiagnostics emits only fixed classifications, counters and booleans.
// Resource names, free-form status messages and log bodies are not diagnostic output.
func collectDCMRouterDiagnostics(ctx context.Context, kubeClient kubernetes.Interface, controllerName string) string {
	ctx, cancel := context.WithTimeout(ctx, dcmDiagnosticsTimeout)
	defer cancel()
	var output strings.Builder
	output.WriteString("DCM router diagnostics (log bodies suppressed)\n")
	requestCtx, requestCancel := context.WithTimeout(ctx, dcmDiagnosticRequestTimeout)
	pods, err := kubeClient.CoreV1().Pods("openshift-ingress").List(requestCtx, metav1.ListOptions{
		LabelSelector: labels.Set{dcmRouterControllerLabel: controllerName}.String(),
		Limit:         dcmDiagnosticMaxPods,
	})
	requestCancel()
	if err != nil {
		output.WriteString("Unable to list router pods\n")
		return output.String()
	}
	collected := 0
	for _, pod := range pods.Items {
		if ctx.Err() != nil || collected >= dcmDiagnosticMaxPods {
			break
		}
		if pod.Namespace != "openshift-ingress" || pod.Labels[dcmRouterControllerLabel] != controllerName {
			continue
		}
		collected++
		phase := "Other"
		switch pod.Status.Phase {
		case corev1.PodPending, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed, corev1.PodUnknown:
			phase = string(pod.Status.Phase)
		}
		fmt.Fprintf(&output, "Pod %d phase=%s deleting=%t\n", collected, phase, pod.DeletionTimestamp != nil)
		containers := append([]corev1.Container(nil), pod.Spec.InitContainers...)
		containers = append(containers, pod.Spec.Containers...)
		for containerIndex, container := range containers {
			if ctx.Err() != nil || containerIndex >= dcmDiagnosticMaxContainers {
				break
			}
			initContainer := containerIndex < len(pod.Spec.InitContainers)
			fmt.Fprintf(&output, "Container %d init=%t", containerIndex+1, initContainer)
			statuses := pod.Status.ContainerStatuses
			if initContainer {
				statuses = pod.Status.InitContainerStatuses
			}
			for _, status := range statuses {
				if status.Name == container.Name {
					fmt.Fprintf(&output, " ready=%t restarts=%d state=%s lastState=%s", status.Ready, status.RestartCount, dcmContainerState(status.State), dcmContainerState(status.LastTerminationState))
					break
				}
			}
			output.WriteByte('\n')
			requestCtx, requestCancel := context.WithTimeout(ctx, dcmDiagnosticRequestTimeout)
			logs, logErr := readDCMRouterLog(requestCtx, kubeClient, pod.Name, container.Name)
			requestCancel()
			if logErr != nil {
				output.WriteString("Unable to read container logs\n")
				continue
			}
			fmt.Fprintf(&output, "%s\n", logs)
		}
	}
	return output.String()
}

// dcmContainerState preserves lifecycle failures without forwarding arbitrary reasons or messages.
func dcmContainerState(state corev1.ContainerState) string {
	switch {
	case state.Waiting != nil:
		reason := "Other"
		switch state.Waiting.Reason {
		case "PodInitializing", "ContainerCreating", "CrashLoopBackOff", "ErrImagePull", "ImagePullBackOff", "CreateContainerConfigError", "CreateContainerError", "RunContainerError":
			reason = state.Waiting.Reason
		}
		return "waiting:" + reason
	case state.Running != nil:
		return "running"
	case state.Terminated != nil:
		return fmt.Sprintf("terminated:exit=%d,signal=%d", state.Terminated.ExitCode, state.Terminated.Signal)
	default:
		return "unobserved"
	}
}

// readDCMRouterLog reports bounded availability metadata, never log text, including partial reads.
func readDCMRouterLog(ctx context.Context, kubeClient kubernetes.Interface, podName, containerName string) (string, error) {
	stream, err := kubeClient.CoreV1().Pods("openshift-ingress").GetLogs(podName, &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  ptr.To[int64](dcmDiagnosticLogLines),
		LimitBytes: ptr.To[int64](dcmDiagnosticLogBytes),
	}).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	content, err := io.ReadAll(io.LimitReader(stream, dcmDiagnosticLogBytes+1))
	if err != nil {
		return "", err
	}
	clipped := len(content) >= dcmDiagnosticLogBytes
	if len(content) > dcmDiagnosticLogBytes {
		content = content[:dcmDiagnosticLogBytes]
	}
	return fmt.Sprintf("Log sample bytes=%d completeLines=%d clipped=%t (body suppressed)", len(content), bytes.Count(content, []byte{'\n'}), clipped), nil
}
