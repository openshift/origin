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
	egressIPCRName     = "disruption-egressip-test"
	targetDepName      = "egressip-disruption-target"
	pollerDepName      = "egressip-disruption-poller"
	targetPort         = 8199
	maxDisruptionSecs  = 120

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
	kubeClient           kubernetes.Interface
	dynamicClient        dynamic.Interface

	egressIP       string
	egressNodeName string
	targetNodeIP   string
}

func NewAvailabilityInvariant(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &availability{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
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

	// Find worker nodes with the egress-ipconfig annotation and an IPv4 subnet.
	egressNodeIdx := -1
	targetNodeIdx := -1
	for i := range workerNodes.Items {
		if ipv4Subnet(&workerNodes.Items[i]) != "" {
			if egressNodeIdx == -1 {
				egressNodeIdx = i
			} else if targetNodeIdx == -1 {
				targetNodeIdx = i
			}
		}
	}
	if egressNodeIdx == -1 || targetNodeIdx == -1 {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "need at least 2 worker nodes with IPv4 egress-ipconfig annotation",
		}
		return w.notSupportedReason
	}

	egressNode := &workerNodes.Items[egressNodeIdx]
	targetNode := &workerNodes.Items[targetNodeIdx]
	w.egressNodeName = egressNode.Name

	w.targetNodeIP = nodeInternalIP(targetNode)
	if w.targetNodeIP == "" {
		return fmt.Errorf("could not find internal IP for node %s", targetNode.Name)
	}

	// Resolve the poller image (openshift-tests) before creating resources.
	pollerImage, err := disruptionpodnetwork.GetOpenshiftTestsImagePullSpec(ctx, adminRESTConfig, w.payloadImagePullSpec, nil)
	if err != nil {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: fmt.Sprintf("unable to determine openshift-tests image: %v", err),
		}
		return w.notSupportedReason
	}

	// Create namespace.
	ns := resourceread.ReadNamespaceV1OrDie(namespaceYaml)
	actualNs, err := w.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	w.namespaceName = actualNs.Name
	fmt.Fprintf(os.Stderr, "egressip disruption test: created namespace %s\n", w.namespaceName)

	// Label egress node using a JSON patch to avoid GET+UPDATE race.
	if err := w.patchNodeLabel(ctx, w.egressNodeName, egressAssignableLabel, ""); err != nil {
		return fmt.Errorf("failed to label egress node: %w", err)
	}

	// Allocate an EgressIP from the egress node's IPv4 subnet.
	w.egressIP, err = w.allocateEgressIP(ctx, egressNode, infra.Status.PlatformStatus.Type)
	if err != nil {
		return fmt.Errorf("failed to allocate egress IP: %w", err)
	}

	// Create EgressIP CR.
	if err := w.createEgressIPCR(ctx); err != nil {
		return fmt.Errorf("failed to create EgressIP CR: %w", err)
	}

	// Deploy target (host-networked on target node, running agnhost netexec).
	if err := w.deployTarget(ctx, targetNode); err != nil {
		return fmt.Errorf("failed to deploy target: %w", err)
	}

	// Deploy poller (in the EgressIP namespace, curls the target).
	if err := w.deployPoller(ctx, pollerImage); err != nil {
		return fmt.Errorf("failed to deploy poller: %w", err)
	}

	// Wait for deployments to have available replicas.
	if err := w.waitForDeploymentReady(ctx, targetDepName, 5*time.Minute); err != nil {
		return fmt.Errorf("target deployment not ready: %w", err)
	}
	if err := w.waitForDeploymentReady(ctx, pollerDepName, 5*time.Minute); err != nil {
		return fmt.Errorf("poller deployment not ready: %w", err)
	}

	// Wait for EgressIP to be assigned.
	if err := w.waitForEgressIPAssigned(ctx, 2*time.Minute); err != nil {
		return fmt.Errorf("EgressIP not assigned: %w", err)
	}

	// Pre-flight: verify SNAT is working before we start monitoring.
	if err := w.verifySNAT(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("pre-flight SNAT check failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "egressip disruption test: monitoring started (egressIP=%s, egressNode=%s, targetNodeIP=%s)\n",
		w.egressIP, w.egressNodeName, w.targetNodeIP)
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

	// Count disruption events from the intervals we produced.
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

	// Remove egress-assignable label from node.
	if w.egressNodeName != "" {
		log.Infof("removing egress-assignable label from node %s", w.egressNodeName)
		if err := w.removeNodeLabel(ctx, w.egressNodeName, egressAssignableLabel); err != nil {
			log.WithError(err).Error("failed to remove egress label")
		}
	}

	// Delete EgressIP CR.
	if w.dynamicClient != nil {
		log.Info("deleting EgressIP CR")
		err := w.dynamicClient.Resource(egressIPGVR).Delete(ctx, egressIPCRName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			log.WithError(err).Error("failed to delete EgressIP CR")
		}
	}

	// Delete namespace.
	if w.namespaceName != "" {
		log.Infof("deleting namespace %s", w.namespaceName)
		if err := w.kubeClient.CoreV1().Namespaces().Delete(ctx, w.namespaceName, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				log.WithError(err).Error("error during namespace deletion")
				return err
			}
		}

		startTime := time.Now()
		err := wait.PollUntilContextTimeout(ctx, 15*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := w.kubeClient.CoreV1().Namespaces().Get(ctx, w.namespaceName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			log.WithError(err).Errorf("timeout waiting for namespace deletion")
			return err
		}
		log.Infof("namespace deleted in %.0fs", time.Since(startTime).Seconds())
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pollerLogEntry represents a single JSON line from the poller pod's stdout.
type pollerLogEntry struct {
	Timestamp string `json:"ts"`
	OK        bool   `json:"ok"`
	Error     string `json:"err,omitempty"`
	GotIP     string `json:"got,omitempty"`
}

func (w *availability) collectPollerData(ctx context.Context) (monitorapi.Intervals, []*junitapi.JUnitTestCase, []error) {
	// Find poller pods by label.
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
	for _, pod := range pods.Items {
		intervals, errs := w.collectPodLogs(ctx, pod.Name)
		allIntervals = append(allIntervals, intervals...)
		allErrors = append(allErrors, errs...)
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

func (w *availability) collectPodLogs(ctx context.Context, podName string) (monitorapi.Intervals, []error) {
	req := w.kubeClient.CoreV1().Pods(w.namespaceName).GetLogs(podName, &corev1.PodLogOptions{})
	logStream, err := req.Stream(ctx)
	if err != nil {
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
			From:  ts,
			To:    ts.Add(1 * time.Second),
		})
	}

	return intervals, nil
}

func (w *availability) deployTarget(ctx context.Context, targetNode *corev1.Node) error {
	originalAgnhost := k8simage.GetOriginalImageConfigs()[k8simage.Agnhost]
	agnhostImage := image.LocationFor(originalAgnhost.GetE2EImage())

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetDepName,
			Namespace: w.namespaceName,
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
					HostNetwork: true,
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": targetNode.Name,
					},
					Containers: []corev1.Container{
						{
							Name:  "target",
							Image: agnhostImage,
							Args:  []string{"netexec", fmt.Sprintf("--http-port=%d", targetPort)},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
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

func (w *availability) deployPoller(ctx context.Context, pollerImage string) error {
	// The poller runs curl in a loop, checking that EgressIP SNAT is applied.
	// It outputs JSON lines to stdout: {"ts":"...", "ok":true/false, "err":"...", "got":"..."}
	// The poller runs until the pod is terminated during Cleanup.
	pollerScript := fmt.Sprintf(`#!/bin/sh
TARGET="http://%s:%d/clientip"
EXPECTED="%s"

while true; do
  ts=$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ 2>/dev/null || echo "unknown")
  resp=$(curl -s --connect-timeout 5 --max-time 10 "${TARGET}" 2>/dev/null) || true
  if [ -n "${resp}" ]; then
    got=$(echo "${resp}" | sed 's/:[0-9]*$//')
    if [ "${got}" = "${EXPECTED}" ]; then
      echo "{\"ts\":\"${ts}\",\"ok\":true}"
    else
      echo "{\"ts\":\"${ts}\",\"ok\":false,\"err\":\"snat-mismatch\",\"got\":\"${got}\"}"
    fi
  else
    echo "{\"ts\":\"${ts}\",\"ok\":false,\"err\":\"request-failed\"}"
  fi
  sleep 1
done
`, w.targetNodeIP, targetPort, w.egressIP)

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
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "EgressIP",
			"metadata": map[string]interface{}{
				"name": egressIPCRName,
			},
			"spec": map[string]interface{}{
				"egressIPs": []interface{}{w.egressIP},
				"namespaceSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
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
			if m, ok := item.(map[string]interface{}); ok {
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

// verifySNAT does a quick pre-flight check by exec-ing into a poller pod
// and verifying the source IP matches the expected EgressIP.
func (w *availability) verifySNAT(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := w.kubeClient.CoreV1().Pods(w.namespaceName).List(ctx, metav1.ListOptions{
			LabelSelector: "app=egressip-disruption-poller",
		})
		if err != nil || len(pods.Items) == 0 {
			return false, nil
		}
		// Read the latest log lines from the poller -- if we see an "ok":true entry, SNAT is working.
		req := w.kubeClient.CoreV1().Pods(w.namespaceName).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{
			TailLines: int64Ptr(5),
		})
		logStream, err := req.Stream(ctx)
		if err != nil {
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
		}
		return false, nil
	})
}

func (w *availability) waitForDeploymentReady(ctx context.Context, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		dep, err := w.kubeClient.AppsV1().Deployments(w.namespaceName).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return dep.Status.AvailableReplicas > 0, nil
	})
}

// patchNodeLabel adds a label to a node using a JSON merge patch (no GET+UPDATE race).
func (w *availability) patchNodeLabel(ctx context.Context, nodeName, key, value string) error {
	patch := fmt.Sprintf(`{"metadata":{"labels":{%q:%q}}}`, key, value)
	_, err := w.kubeClient.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

// removeNodeLabel removes a label from a node using a JSON merge patch.
func (w *availability) removeNodeLabel(ctx context.Context, nodeName, key string) error {
	patch := fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, key)
	_, err := w.kubeClient.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// allocateEgressIP finds a free IPv4 address from the egress node's subnet.
// Replicates logic from test/extended/networking/egressip_helpers.go.
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

// ipv4Subnet returns the IPv4 CIDR from the node's egress-ipconfig annotation, or "".
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

	// Reserve all node internal IPs.
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

	// Reserve IPs from existing EgressIP CRs.
	egressIPList, err := w.dynamicClient.Resource(egressIPGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range egressIPList.Items {
			spec, _, _ := unstructured.NestedMap(item.Object, "spec")
			if ips, found, _ := unstructured.NestedStringSlice(spec, "egressIPs"); found {
				reserved = append(reserved, ips...)
			}
		}
	}

	// Reserve IPs from CloudPrivateIPConfigs (cloud-level IP assignments).
	cpicList, err := w.dynamicClient.Resource(cloudPrivateIPConfigGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range cpicList.Items {
			// The CloudPrivateIPConfig name IS the IP address.
			reserved = append(reserved, item.GetName())
		}
	}

	return reserved, nil
}

// getFirstFreeIPs returns the first available IPs from the given CIDR, accounting for
// platform-specific reserved ranges. Replicates logic from egressip_helpers.go.
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

func nodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

func int64Ptr(v int64) *int64 {
	return &v
}
