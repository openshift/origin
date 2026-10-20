package router

import (
	"context"
	"fmt"
	"io"
	"regexp"
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
	exutil "github.com/openshift/origin/test/extended/util"
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

var (
	dcmDiagnosticPEM     = regexp.MustCompile(`(?s)-----BEGIN [^-\r\n]+-----.*?(?:-----END [^-\r\n]+-----|$)`)
	dcmDiagnosticPEMTail = regexp.MustCompile(`(?s)^.*?-----END [^-\r\n]+-----`)
	dcmDiagnosticKeyLine = regexp.MustCompile(`(?m)^[\t ]*[A-Za-z0-9+/]{32,}={0,2}[\t ]*\r?$`)
)

func cleanupDCMIngressController(ctx context.Context, kubeClient kubernetes.Interface, operatorClient operatorclient.OperatorV1Interface, controller types.NamespacedName, failed bool) error {
	if controller.Name == "" {
		return nil
	}
	if failed {
		framework.Logf("%s", collectDCMRouterDiagnostics(ctx, kubeClient, controller.Name))
	}
	return operatorClient.IngressControllers(controller.Namespace).Delete(ctx, controller.Name, *metav1.NewDeleteOptions(1))
}

func collectDCMRouterDiagnostics(ctx context.Context, kubeClient kubernetes.Interface, controllerName string) string {
	ctx, cancel := context.WithTimeout(ctx, dcmDiagnosticsTimeout)
	defer cancel()
	var output strings.Builder
	fmt.Fprintf(&output, "DCM router diagnostics for %s\n", controllerName)
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
		fmt.Fprintf(&output, "Pod %s phase=%s deleting=%t\n", pod.Name, pod.Status.Phase, pod.DeletionTimestamp != nil)
		containers := append([]corev1.Container(nil), pod.Spec.InitContainers...)
		containers = append(containers, pod.Spec.Containers...)
		for containerIndex, container := range containers {
			if ctx.Err() != nil || containerIndex >= dcmDiagnosticMaxContainers {
				break
			}
			requestCtx, requestCancel := context.WithTimeout(ctx, dcmDiagnosticRequestTimeout)
			logs, logErr := readDCMRouterLog(requestCtx, kubeClient, pod.Name, container.Name)
			requestCancel()
			if logErr != nil {
				fmt.Fprintf(&output, "Unable to read logs for %s/%s\n", pod.Name, container.Name)
				continue
			}
			fmt.Fprintf(&output, "Logs for %s/%s:\n%s\n", pod.Name, container.Name, logs)
		}
	}
	return output.String()
}

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
	content, err := io.ReadAll(io.LimitReader(stream, dcmDiagnosticLogBytes))
	if err != nil {
		return "", err
	}
	safe := redactDCMRouterLog(string(content))
	if len(safe) > dcmDiagnosticLogBytes {
		safe = safe[:dcmDiagnosticLogBytes]
	}
	return safe, nil
}

func redactDCMRouterLog(content string) string {
	content = dcmDiagnosticPEM.ReplaceAllString(content, "<redacted PEM>")
	content = dcmDiagnosticPEMTail.ReplaceAllString(content, "<redacted PEM>")
	content = dcmDiagnosticKeyLine.ReplaceAllString(content, "<redacted key data>")
	return exutil.RedactBearerToken(content)
}
