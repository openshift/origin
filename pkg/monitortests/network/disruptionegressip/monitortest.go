package disruptionegressip

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/sirupsen/logrus"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionpodnetwork"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var (
	//go:embed namespace.yaml
	namespaceYaml []byte
)

const (
	egressIPCRName    = "disruption-egressip-test"
	targetDepName     = "egressip-disruption-target"
	targetSvcName     = "egressip-disruption-target"
	pollerDepName     = "egressip-disruption-poller"
	targetPort        = 8080
	maxDisruptionSecs = 120

	egressIPConfigAnnotation = "cloud.network.openshift.io/egress-ipconfig"
	egressAssignableLabel    = "k8s.ovn.org/egress-assignable"

	testName = "[sig-network] disruption/egress-ip should be available throughout the test"
)

var (
	egressIPGVR = schema.GroupVersionResource{
		Group:    "k8s.ovn.org",
		Version:  "v1",
		Resource: "egressips",
	}

	cloudPrivateIPConfigGVR = schema.GroupVersionResource{
		Group:    "cloud.network.openshift.io",
		Version:  "v1",
		Resource: "cloudprivateipconfigs",
	}
)

type availability struct {
	payloadImagePullSpec string
	notSupportedReason   error
	namespaceName        string
	targetNamespaceName  string
	kubeClient           kubernetes.Interface
	dynamicClient        dynamic.Interface

	egressIP       string
	egressNodeName string
	platformType   configv1.PlatformType
	lbAddress      string
}

func NewAvailabilityInvariant(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &availability{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

func (w *availability) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error

	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	w.dynamicClient, err = dynamic.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	isMicroShift, err := exutil.IsMicroShiftCluster(w.kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "MicroShift not supported"}
		return w.notSupportedReason
	}

	configClient, err := configclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	network, err := configClient.ConfigV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if network.Status.NetworkType != "OVNKubernetes" {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: fmt.Sprintf("network type %q is not OVNKubernetes", network.Status.NetworkType),
		}
		return w.notSupportedReason
	}

	infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "single-node topology not supported",
		}
		return w.notSupportedReason
	}

	workerNodes, err := w.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker",
	})
	if err != nil {
		return err
	}
	if len(workerNodes.Items) < 2 {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "need at least 2 worker nodes for EgressIP testing",
		}
		return w.notSupportedReason
	}

	// Find worker nodes with IPv4 egress-ipconfig annotation.
	var egressNode *corev1.Node
	var nodesWithEgressConfig int
	for i := range workerNodes.Items {
		if ipv4Subnet(&workerNodes.Items[i]) != "" {
			nodesWithEgressConfig++
			if egressNode == nil {
				egressNode = &workerNodes.Items[i]
			}
		}
	}
	if nodesWithEgressConfig < 2 || egressNode == nil {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "need at least 2 worker nodes with IPv4 egress-ipconfig annotation",
		}
		return w.notSupportedReason
	}
	w.egressNodeName = egressNode.Name

	// Resolve the poller image (openshift-tests) before creating resources.
	pollerImage, err := disruptionpodnetwork.GetOpenshiftTestsImagePullSpec(ctx, adminRESTConfig, w.payloadImagePullSpec, nil)
	if err != nil {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: fmt.Sprintf("unable to determine openshift-tests image: %v", err),
		}
		return w.notSupportedReason
	}

	w.platformType = infra.Status.PlatformStatus.Type

	// Label egress node using a JSON patch to avoid GET+UPDATE race.
	if err := w.patchNodeLabel(ctx, w.egressNodeName, egressAssignableLabel, ""); err != nil {
		return fmt.Errorf("failed to label egress node: %w", err)
	}

	// Create the EgressIP namespace first (needed for the EgressIP CR namespace selector).
	ns := resourceread.ReadNamespaceV1OrDie(namespaceYaml)
	actualNs, err := w.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	w.namespaceName = actualNs.Name
	fmt.Fprintf(os.Stderr, "egressip disruption test: created EgressIP namespace %s\n", w.namespaceName)

	// Allocate an EgressIP from the egress node's IPv4 subnet.
	w.egressIP, err = w.allocateEgressIP(ctx, egressNode, w.platformType)
	if err != nil {
		return fmt.Errorf("failed to allocate egress IP: %w", err)
	}

	// Create EgressIP CR (references w.namespaceName in namespace selector).
	if err := w.createEgressIPCR(ctx); err != nil {
		return fmt.Errorf("failed to create EgressIP CR: %w", err)
	}

	// Deploy target in a separate namespace behind a LoadBalancer service.
	// The target must be outside the EgressIP namespace so EgressIP rules
	// don't apply to the target's own traffic.
	if err := w.deployTarget(ctx, infra); err != nil {
		return fmt.Errorf("failed to deploy target: %w", err)
	}

	// Wait for the LoadBalancer to get an external IP.
	if err := w.waitForLoadBalancerIP(ctx, 10*time.Minute); err != nil {
		return fmt.Errorf("LoadBalancer not ready: %w", err)
	}

	if err := w.deployPoller(ctx, pollerImage); err != nil {
		return fmt.Errorf("failed to deploy poller: %w", err)
	}
	if err := w.waitForDeploymentReady(ctx, w.namespaceName, pollerDepName, 5*time.Minute); err != nil {
		return fmt.Errorf("poller deployment not ready: %w", err)
	}

	// Wait for EgressIP to be assigned.
	if err := w.waitForEgressIPAssigned(ctx, 2*time.Minute); err != nil {
		return fmt.Errorf("EgressIP not assigned: %w", err)
	}

	// On cloud platforms, wait for CloudPrivateIPConfig to be fully assigned
	// (IP attached to the node's NIC by the cloud provider).
	if err := w.waitForCloudPrivateIPConfigAssigned(ctx, 3*time.Minute); err != nil {
		return fmt.Errorf("CloudPrivateIPConfig not assigned: %w", err)
	}

	// Pre-flight: verify SNAT is working before we start monitoring.
	if err := w.verifySNAT(ctx, 3*time.Minute); err != nil {
		return fmt.Errorf("pre-flight SNAT check failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "egressip disruption test: monitoring started (egressIP=%s, egressNode=%s, lbAddress=%s)\n",
		w.egressIP, w.egressNodeName, w.lbAddress)
	return nil
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}
	if w.kubeClient == nil {
		return nil, nil, nil
	}

	// Give the poller a moment to flush its last log lines.
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	// Read poller pod logs and parse into intervals.
	intervals, junits, errs := w.collectPollerData(ctx)
	if len(errs) > 0 {
		return intervals, junits, fmt.Errorf("errors collecting poller data: %v", errs)
	}
	return intervals, junits, nil
}

func (w *availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	if w.kubeClient == nil {
		return nil, nil
	}

	disruptions := 0
	var totalDisruption time.Duration
	for i := range finalIntervals {
		interval := &finalIntervals[i]
		if interval.Source != monitorapi.SourceDisruption {
			continue
		}
		if !strings.Contains(interval.Message.HumanMessage, "egress-ip") {
			continue
		}
		if interval.Level > monitorapi.Info {
			disruptions++
			totalDisruption += interval.To.Sub(interval.From)
		}
	}

	testCase := &junitapi.JUnitTestCase{
		Name: testName,
	}
	if disruptions > 0 {
		testCase.SystemOut = fmt.Sprintf("Observed %d disruption intervals totaling %s", disruptions, totalDisruption)
		if totalDisruption.Seconds() > maxDisruptionSecs {
			testCase.FailureOutput = &junitapi.FailureOutput{
				Output: fmt.Sprintf("EgressIP disruption exceeded %ds threshold: %d intervals totaling %s",
					maxDisruptionSecs, disruptions, totalDisruption),
			}
		}
	}

	return []*junitapi.JUnitTestCase{testCase}, nil
}

func (w *availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return w.notSupportedReason
}

func (w *availability) Cleanup(ctx context.Context) error {
	if w.kubeClient == nil {
		return nil
	}

	log := logrus.WithField("monitorTest", "egressip-disruption")

	if w.egressNodeName != "" {
		log.Infof("removing egress-assignable label from node %s", w.egressNodeName)
		if err := w.removeNodeLabel(ctx, w.egressNodeName, egressAssignableLabel); err != nil {
			log.WithError(err).Error("failed to remove egress label")
		}
	}

	if w.dynamicClient != nil {
		log.Info("deleting EgressIP CR")
		err := w.dynamicClient.Resource(egressIPGVR).Delete(ctx, egressIPCRName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			log.WithError(err).Error("failed to delete EgressIP CR")
		}
	}

	for _, ns := range []string{w.namespaceName, w.targetNamespaceName} {
		if ns == "" {
			continue
		}
		log.Infof("deleting namespace %s", ns)
		if err := w.kubeClient.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				log.WithError(err).Errorf("error during namespace deletion for %s", ns)
			}
			continue
		}

		startTime := time.Now()
		err := wait.PollUntilContextTimeout(ctx, 15*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := w.kubeClient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			log.WithError(err).Errorf("timeout waiting for namespace %s deletion", ns)
		} else {
			log.Infof("namespace %s deleted in %.0fs", ns, time.Since(startTime).Seconds())
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type pollerLogEntry struct {
	Timestamp string `json:"ts"`
	OK        bool   `json:"ok"`
	Error     string `json:"err,omitempty"`
	GotIP     string `json:"got,omitempty"`
}

func (w *availability) collectPollerData(ctx context.Context) (monitorapi.Intervals, []*junitapi.JUnitTestCase, []error) {
	pods, err := w.kubeClient.CoreV1().Pods(w.namespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: "app=egressip-disruption-poller",
	})
	if err != nil {
		return nil, nil, []error{fmt.Errorf("failed to list poller pods: %w", err)}
	}
	if len(pods.Items) == 0 {
		return nil, nil, []error{fmt.Errorf("no poller pods found")}
	}

	var allIntervals monitorapi.Intervals
	var allErrors []error
	seen := make(map[string]bool)
	for _, pod := range pods.Items {
		intervals, errs := w.collectPodLogs(ctx, pod.Name, false)
		allErrors = append(allErrors, errs...)

		for _, iv := range intervals {
			seen[iv.From.Format(time.RFC3339Nano)] = true
			allIntervals = append(allIntervals, iv)
		}

		// Also try previous container logs to capture data from before a restart.
		prevIntervals, _ := w.collectPodLogs(ctx, pod.Name, true)
		for _, iv := range prevIntervals {
			key := iv.From.Format(time.RFC3339Nano)
			if !seen[key] {
				seen[key] = true
				allIntervals = append(allIntervals, iv)
			}
		}
	}

	logJunit := &junitapi.JUnitTestCase{
		Name: "[sig-network] can collect egress-ip poller pod logs",
	}
	if len(allIntervals) == 0 && len(allErrors) > 0 {
		logJunit.FailureOutput = &junitapi.FailureOutput{
			Output: fmt.Sprintf("errors collecting poller logs: %v", allErrors),
		}
	}

	return allIntervals, []*junitapi.JUnitTestCase{logJunit}, allErrors
}

func (w *availability) collectPodLogs(ctx context.Context, podName string, previous bool) (monitorapi.Intervals, []error) {
	req := w.kubeClient.CoreV1().Pods(w.namespaceName).GetLogs(podName, &corev1.PodLogOptions{
		Previous: previous,
	})
	logStream, err := req.Stream(ctx)
	if err != nil {
		if previous {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("failed to get logs for pod %s: %w", podName, err)}
	}
	defer logStream.Close()

	var intervals monitorapi.Intervals
	scanner := bufio.NewScanner(logStream)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry pollerLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			continue
		}

		if entry.OK {
			continue
		}

		msg := fmt.Sprintf("egress-ip disruption: %s", entry.Error)
		if entry.GotIP != "" {
			msg = fmt.Sprintf("egress-ip disruption: %s (got %s, expected %s)", entry.Error, entry.GotIP, w.egressIP)
		}

		intervals = append(intervals, monitorapi.Interval{
			Condition: monitorapi.Condition{
				Level: monitorapi.Error,
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "egress-ip",
					},
				},
				Message: monitorapi.Message{
					Reason:       monitorapi.DisruptionBeganEventReason,
					HumanMessage: msg,
				},
			},
			Source: monitorapi.SourceDisruption,
			From:   ts,
			To:     ts.Add(1 * time.Second),
		})
	}

	return intervals, nil
}

// deployTarget creates a target Deployment and LoadBalancer Service in a
// separate namespace. Traffic to the LB's external IP is truly external,
// so OVN applies EgressIP SNAT. The target's /clientip endpoint reveals
// the source IP, allowing the poller to verify SNAT is working.
func (w *availability) deployTarget(ctx context.Context, infra *configv1.Infrastructure) error {
	targetNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "e2e-egressip-target-",
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce":                       "baseline",
				"pod-security.kubernetes.io/audit":                         "baseline",
				"pod-security.kubernetes.io/warn":                          "baseline",
				"security.openshift.io/scc.podSecurityLabelSync":           "false",
				"security.openshift.io/disable-securitycontextconstraints": "true",
			},
			Annotations: map[string]string{
				"workload.openshift.io/allowed": "management",
			},
		},
	}
	actualNs, err := w.kubeClient.CoreV1().Namespaces().Create(ctx, targetNs, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create target namespace: %w", err)
	}
	w.targetNamespaceName = actualNs.Name
	fmt.Fprintf(os.Stderr, "egressip disruption test: created target namespace %s\n", w.targetNamespaceName)

	originalAgnhost := k8simage.GetOriginalImageConfigs()[k8simage.Agnhost]
	agnhostImage := image.LocationFor(originalAgnhost.GetE2EImage())

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetDepName,
			Namespace: w.targetNamespaceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "egressip-disruption-target"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "egressip-disruption-target"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "target",
							Image: agnhostImage,
							Args:  []string{"netexec", fmt.Sprintf("--http-port=%d", targetPort)},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(targetPort),
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}
	if _, err := w.kubeClient.AppsV1().Deployments(w.targetNamespaceName).Create(ctx, dep, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create target deployment: %w", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetSvcName,
			Namespace: w.targetNamespaceName,
			Annotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-healthcheck-interval":            "8",
				"service.beta.kubernetes.io/aws-load-balancer-healthcheck-unhealthy-threshold": "3",
				"service.beta.kubernetes.io/aws-load-balancer-healthcheck-healthy-threshold":   "2",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			// Local preserves the client source IP so /clientip returns the
			// egress IP rather than a kube-proxy SNAT address.
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			Selector:              map[string]string{"app": "egressip-disruption-target"},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt32(int32(targetPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// AWS requires NLB for dual-stack.
	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.AWS != nil {
		switch infra.Status.PlatformStatus.AWS.IPFamily {
		case configv1.DualStackIPv4Primary, configv1.DualStackIPv6Primary:
			svc.Annotations["service.beta.kubernetes.io/aws-load-balancer-type"] = "nlb"
		}
	}

	if _, err := w.kubeClient.CoreV1().Services(w.targetNamespaceName).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create target service: %w", err)
	}

	if err := w.waitForDeploymentReady(ctx, w.targetNamespaceName, targetDepName, 5*time.Minute); err != nil {
		return fmt.Errorf("target deployment not ready: %w", err)
	}

	fmt.Fprintf(os.Stderr, "egressip disruption test: target deployed in %s\n", w.targetNamespaceName)
	return nil
}

func (w *availability) waitForLoadBalancerIP(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		svc, err := w.kubeClient.CoreV1().Services(w.targetNamespaceName).Get(ctx, targetSvcName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return false, nil
		}
		ingress := svc.Status.LoadBalancer.Ingress[0]
		if ingress.IP != "" {
			w.lbAddress = ingress.IP
		} else if ingress.Hostname != "" {
			w.lbAddress = ingress.Hostname
		}
		if w.lbAddress != "" {
			fmt.Fprintf(os.Stderr, "egressip disruption test: LoadBalancer address: %s\n", w.lbAddress)
			return true, nil
		}
		return false, nil
	})
}

func (w *availability) deployPoller(ctx context.Context, pollerImage string) error {
	pollerScript := fmt.Sprintf(`#!/bin/sh
LB_URL="http://%s:80/clientip"
EXPECTED="%s"

while true; do
  ts=$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ 2>/dev/null || echo "unknown")
  resp=$(curl -s --connect-timeout 2 --max-time 5 "${LB_URL}" 2>/dev/null)
  if [ -n "${resp}" ]; then
    got=$(echo "${resp}" | sed 's/:[0-9]*$//')
    if [ "${got}" = "${EXPECTED}" ]; then
      echo "{\"ts\":\"${ts}\",\"ok\":true}"
    else
      echo "{\"ts\":\"${ts}\",\"ok\":false,\"err\":\"snat-mismatch\",\"got\":\"${got}\"}"
    fi
  else
    echo "{\"ts\":\"${ts}\",\"ok\":false,\"err\":\"target-unreachable\"}"
  fi
  sleep 1
done
`, w.lbAddress, w.egressIP)

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pollerDepName,
			Namespace: w.namespaceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "egressip-disruption-poller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "egressip-disruption-poller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "poller",
							Image:   pollerImage,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{pollerScript},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}
	_, err := w.kubeClient.AppsV1().Deployments(w.namespaceName).Create(ctx, dep, metav1.CreateOptions{})
	return err
}

func (w *availability) createEgressIPCR(ctx context.Context) error {
	egressIPObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "EgressIP",
			"metadata": map[string]any{
				"name": egressIPCRName,
			},
			"spec": map[string]any{
				"egressIPs": []any{w.egressIP},
				"namespaceSelector": map[string]any{
					"matchLabels": map[string]any{
						"kubernetes.io/metadata.name": w.namespaceName,
					},
				},
			},
		},
	}
	_, err := w.dynamicClient.Resource(egressIPGVR).Create(ctx, egressIPObj, metav1.CreateOptions{})
	return err
}

func (w *availability) waitForEgressIPAssigned(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		obj, err := w.dynamicClient.Resource(egressIPGVR).Get(ctx, egressIPCRName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		status, found, err := unstructured.NestedSlice(obj.Object, "status", "items")
		if err != nil || !found {
			return false, nil
		}
		for _, item := range status {
			if m, ok := item.(map[string]any); ok {
				if ip, _ := m["egressIP"].(string); ip == w.egressIP {
					if node, _ := m["node"].(string); node != "" {
						fmt.Fprintf(os.Stderr, "egressip disruption test: EgressIP %s assigned to node %s\n", ip, node)
						return true, nil
					}
				}
			}
		}
		return false, nil
	})
}

func (w *availability) waitForCloudPrivateIPConfigAssigned(ctx context.Context, timeout time.Duration) error {
	_, err := w.dynamicClient.Resource(cloudPrivateIPConfigGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		fmt.Fprintf(os.Stderr, "egressip disruption test: CloudPrivateIPConfig CRD not available, skipping CPIC readiness check\n")
		return nil
	}

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		cpic, err := w.dynamicClient.Resource(cloudPrivateIPConfigGVR).Get(ctx, w.egressIP, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "egressip disruption test: CloudPrivateIPConfig %s not found yet\n", w.egressIP)
			return false, nil
		}

		conditions, found, err := unstructured.NestedSlice(cpic.Object, "status", "conditions")
		if err != nil || !found {
			fmt.Fprintf(os.Stderr, "egressip disruption test: CloudPrivateIPConfig %s has no status conditions yet\n", w.egressIP)
			return false, nil
		}

		for _, c := range conditions {
			cond, ok := c.(map[string]any)
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			if condType == "Assigned" {
				if condStatus == string(metav1.ConditionTrue) {
					fmt.Fprintf(os.Stderr, "egressip disruption test: CloudPrivateIPConfig %s is assigned\n", w.egressIP)
					return true, nil
				}
				fmt.Fprintf(os.Stderr, "egressip disruption test: CloudPrivateIPConfig %s condition Assigned=%s\n", w.egressIP, condStatus)
				return false, nil
			}
		}
		fmt.Fprintf(os.Stderr, "egressip disruption test: CloudPrivateIPConfig %s has no Assigned condition yet\n", w.egressIP)
		return false, nil
	})
}

func (w *availability) verifySNAT(ctx context.Context, timeout time.Duration) error {
	var lastStatus string
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := w.kubeClient.CoreV1().Pods(w.namespaceName).List(ctx, metav1.ListOptions{
			LabelSelector: "app=egressip-disruption-poller",
		})
		if err != nil || len(pods.Items) == 0 {
			lastStatus = "no poller pods found"
			return false, nil
		}
		req := w.kubeClient.CoreV1().Pods(w.namespaceName).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{
			TailLines: int64Ptr(5),
		})
		logStream, err := req.Stream(ctx)
		if err != nil {
			lastStatus = fmt.Sprintf("failed to stream logs: %v", err)
			return false, nil
		}
		defer logStream.Close()

		scanner := bufio.NewScanner(logStream)
		for scanner.Scan() {
			var entry pollerLogEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				continue
			}
			if entry.OK {
				fmt.Fprintf(os.Stderr, "egressip disruption test: pre-flight SNAT check passed\n")
				return true, nil
			}
			if entry.Error != "" {
				lastStatus = fmt.Sprintf("err=%s got=%s expected=%s", entry.Error, entry.GotIP, w.egressIP)
			}
		}
		fmt.Fprintf(os.Stderr, "egressip disruption test: SNAT not ready yet (%s)\n", lastStatus)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("%w (last poller status: %s)", err, lastStatus)
	}
	return nil
}

func (w *availability) waitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		dep, err := w.kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return dep.Status.AvailableReplicas > 0, nil
	})
}

func (w *availability) patchNodeLabel(ctx context.Context, nodeName, key, value string) error {
	patch := fmt.Sprintf(`{"metadata":{"labels":{%q:%q}}}`, key, value)
	_, err := w.kubeClient.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

func (w *availability) removeNodeLabel(ctx context.Context, nodeName, key string) error {
	patch := fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, key)
	_, err := w.kubeClient.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (w *availability) allocateEgressIP(ctx context.Context, node *corev1.Node, platformType configv1.PlatformType) (string, error) {
	ipnetStr := ipv4Subnet(node)
	if ipnetStr == "" {
		return "", fmt.Errorf("node %s has no IPv4 egress subnet", node.Name)
	}

	reservedIPs, err := w.buildReservedIPs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build reserved IP list: %w", err)
	}

	freeIPs, err := getFirstFreeIPs(ipnetStr, reservedIPs, platformType, 1)
	if err != nil {
		return "", err
	}
	if len(freeIPs) == 0 {
		return "", fmt.Errorf("no free IPs available in subnet %s", ipnetStr)
	}
	return freeIPs[0], nil
}

func ipv4Subnet(node *corev1.Node) string {
	annotation, ok := node.Annotations[egressIPConfigAnnotation]
	if !ok {
		return ""
	}

	type ifAddr struct {
		IPv4 string `json:"ipv4,omitempty"`
	}
	type nodeEgressIPConfig struct {
		IFAddr ifAddr `json:"ifaddr"`
	}

	var configs []nodeEgressIPConfig
	if err := json.Unmarshal([]byte(annotation), &configs); err != nil || len(configs) == 0 {
		return ""
	}
	return configs[0].IFAddr.IPv4
}

func (w *availability) buildReservedIPs(ctx context.Context) ([]string, error) {
	var reserved []string

	nodes, err := w.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				reserved = append(reserved, addr.Address)
			}
		}
	}

	egressIPList, err := w.dynamicClient.Resource(egressIPGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range egressIPList.Items {
			spec, _, _ := unstructured.NestedMap(item.Object, "spec")
			if ips, found, _ := unstructured.NestedStringSlice(spec, "egressIPs"); found {
				reserved = append(reserved, ips...)
			}
		}
	}

	cpicList, err := w.dynamicClient.Resource(cloudPrivateIPConfigGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range cpicList.Items {
			reserved = append(reserved, item.GetName())
		}
	}

	return reserved, nil
}

func getFirstFreeIPs(ipnetStr string, reservedIPs []string, platformType configv1.PlatformType, count int) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(ipnetStr)
	if err != nil {
		return nil, err
	}
	ipList := subnetIPs(*ipnet)

	switch platformType {
	case configv1.AWSPlatformType:
		if len(ipList) < 6 {
			return nil, fmt.Errorf("AWS subnet %s too small", ipnetStr)
		}
		ipList = ipList[5 : len(ipList)-1]
	case configv1.AzurePlatformType:
		if len(ipList) < 5 {
			return nil, fmt.Errorf("Azure subnet %s too small", ipnetStr)
		}
		ipList = ipList[4 : len(ipList)-1]
	case configv1.GCPPlatformType:
		if len(ipList) < 3 {
			return nil, fmt.Errorf("GCP subnet %s too small", ipnetStr)
		}
		ipList = ipList[2 : len(ipList)-1]
	case configv1.OpenStackPlatformType:
		if len(ipList) < 64 {
			return nil, fmt.Errorf("OpenStack subnet %s too small", ipnetStr)
		}
		ipList = ipList[len(ipList)-32 : len(ipList)-1]
	default:
		if len(ipList) > 2 {
			ipList = ipList[1 : len(ipList)-1]
		}
	}

	reserved := make(map[string]bool, len(reservedIPs))
	for _, r := range reservedIPs {
		reserved[r] = true
	}

	var free []string
	for _, ip := range ipList {
		if !reserved[ip.String()] {
			free = append(free, ip.String())
			if len(free) >= count {
				return free, nil
			}
		}
	}

	return free, fmt.Errorf("could not find %d free IPs in %s (found %d)", count, ipnetStr, len(free))
}

func subnetIPs(ipnet net.IPNet) []net.IP {
	var ips []net.IP
	ip := make(net.IP, len(ipnet.IP))
	copy(ip, ipnet.IP)
	for ; ipnet.Contains(ip); ip = incIP(ip) {
		ips = append(ips, ip)
	}
	return ips
}

func incIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}

func int64Ptr(v int64) *int64 {
	return &v
}
