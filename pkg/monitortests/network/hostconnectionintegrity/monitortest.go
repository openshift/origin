// Package hostconnectionintegrity implements a monitor test that detects established host-network TCP
// connections that are reset or silently black-holed by an ovnkube-node gateway reconcile during the
// teardown of a user defined network (OCPBUGS-128289).
//
// A hostNetwork poller runs on every node (see the poller subpackage). It keeps long-lived connections to
// every peer kubelet, to api-int and, on control-plane nodes, to the local kube-apiserver, and records every
// episode where such an established connection is reset or stops passing traffic while a new connection to
// the same target still works. After the run the monitor reads the ovnkube-controller logs of every
// ovnkube-node pod and correlates the failures with gateway reconciles on the same node.
package hostconnectionintegrity

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionpodnetwork"
	"github.com/openshift/origin/pkg/monitortests/network/hostconnectionintegrity/poller"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// MonitorName is the name the monitor test is registered under.
	MonitorName = "host-network-connection-integrity"

	// TestName is the junit test that fails when an established connection is black-holed by a gateway
	// reconcile.
	TestName = "[sig-network] established host-network connections should not be black-holed by gateway reconcile during UDN teardown"
	// LogCollectionTestName fails when the ovnkube-controller logs needed for the correlation can't be read.
	LogCollectionTestName = "[sig-network] can collect ovnkube-controller logs for host connection integrity"

	ovnKubernetesNamespace = "openshift-ovn-kubernetes"
	ovnkubeNodeSelector    = "app=ovnkube-node"
	ovnkubeControllerName  = "ovnkube-controller"
	pollerLabelKey         = "network.openshift.io/host-connection-integrity"
	stopConfigMapName      = "stop-collecting"
)

var (
	//go:embed manifests/*.yaml
	manifests embed.FS

	namespaceTemplate   *corev1.Namespace
	roleBindingTemplate *rbacv1.RoleBinding
	deploymentTemplate  *appsv1.Deployment
)

func manifestOrDie(name string) []byte {
	ret, err := manifests.ReadFile("manifests/" + name)
	if err != nil {
		panic(err)
	}
	return ret
}

func init() {
	namespaceTemplate = resourceread.ReadNamespaceV1OrDie(manifestOrDie("namespace.yaml"))
	roleBindingTemplate = resourceread.ReadRoleBindingV1OrDie(manifestOrDie("poller-rolebinding.yaml"))
	deploymentTemplate = resourceread.ReadDeploymentV1OrDie(manifestOrDie("poller-deployment.yaml"))
}

type hostConnectionIntegrity struct {
	payloadImagePullSpec string
	notSupportedReason   error

	kubeClient    kubernetes.Interface
	namespaceName string
	pollerImage   string
	peers         []string
	controlPlane  []string
	apiIntURL     string

	findings []Finding
}

// NewMonitorTest returns the monitor test.
func NewMonitorTest(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &hostConnectionIntegrity{payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec}
}

func (w *hostConnectionIntegrity) notSupported(format string, args ...interface{}) error {
	w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf(format, args...)}
	return w.notSupportedReason
}

func (w *hostConnectionIntegrity) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	configClient, err := configclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	w.kubeClient = kubeClient

	if isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient); err != nil {
		return err
	} else if isMicroShift {
		return w.notSupported("not supported on MicroShift")
	}
	if isHyperShift, err := exutil.IsHypershift(ctx, configClient); err != nil {
		return err
	} else if isHyperShift {
		return w.notSupported("not supported on HyperShift: the control plane is not on the cluster nodes")
	}
	if isManaged, _ := exutil.IsManagedServiceCluster(ctx, kubeClient); isManaged {
		return w.notSupported("hostNetwork pollers are unschedulable on managed service clusters (TRT-1869)")
	}

	network, err := configClient.ConfigV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if network.Status.NetworkType != "OVNKubernetes" {
		return w.notSupported("network type is %q, the test only applies to OVNKubernetes", network.Status.NetworkType)
	}

	infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		return w.notSupported("not supported on single replica control plane topology")
	}
	w.apiIntURL = infra.Status.APIServerInternalURL

	oc := exutil.NewCLIWithoutNamespace("openshift-tests")
	w.pollerImage, err = disruptionpodnetwork.GetOpenshiftTestsImagePullSpec(ctx, adminRESTConfig, w.payloadImagePullSpec, oc)
	if err != nil {
		return w.notSupported("unable to determine openshift-tests image: %v", err)
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	w.peers, w.controlPlane = peersFromNodes(nodes.Items)
	if len(w.peers) < 2 {
		return w.notSupported("need at least two nodes with an InternalIP, found %d", len(w.peers))
	}
	return nil
}

// peersFromNodes returns nodeName=InternalIP pairs and the control-plane node names.
func peersFromNodes(nodes []corev1.Node) ([]string, []string) {
	peers, controlPlane := []string{}, []string{}
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				peers = append(peers, fmt.Sprintf("%s=%s", n.Name, a.Address))
				break
			}
		}
		_, master := n.Labels["node-role.kubernetes.io/master"]
		_, cp := n.Labels["node-role.kubernetes.io/control-plane"]
		if master || cp {
			controlPlane = append(controlPlane, n.Name)
		}
	}
	sort.Strings(peers)
	sort.Strings(controlPlane)
	return peers, controlPlane
}

func (w *hostConnectionIntegrity) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}

	var ns *corev1.Namespace
	if err := utility.RetryWithExponentialBackoff(ctx, func() error {
		var createErr error
		ns, createErr = w.kubeClient.CoreV1().Namespaces().Create(ctx, namespaceTemplate, metav1.CreateOptions{})
		return createErr
	}); err != nil {
		return err
	}
	w.namespaceName = ns.Name

	if err := utility.RetryWithExponentialBackoff(ctx, func() error {
		_, createErr := w.kubeClient.RbacV1().RoleBindings(w.namespaceName).Create(ctx, roleBindingTemplate, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(createErr) {
			return nil
		}
		return createErr
	}); err != nil {
		return err
	}

	deployment := buildDeployment(w.pollerImage, w.peers, w.controlPlane, w.apiIntURL)
	klog.Infof("Starting deployment %s/%s with %d replicas", w.namespaceName, deployment.Name, *deployment.Spec.Replicas)
	return utility.RetryWithExponentialBackoff(ctx, func() error {
		_, createErr := w.kubeClient.AppsV1().Deployments(w.namespaceName).Create(ctx, deployment, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(createErr) {
			return nil
		}
		return createErr
	})
}

func buildDeployment(image string, peers, controlPlane []string, apiIntURL string) *appsv1.Deployment {
	d := deploymentTemplate.DeepCopy()
	replicas := int32(len(peers))
	d.Spec.Replicas = &replicas
	c := &d.Spec.Template.Spec.Containers[0]
	c.Image = image
	c.Command = append(c.Command,
		"--peers="+strings.Join(peers, ","),
		"--control-plane-nodes="+strings.Join(controlPlane, ","),
		"--api-int-url="+apiIntURL,
	)
	return d
}

func (w *hostConnectionIntegrity) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	var errs []error

	if _, err := w.kubeClient.CoreV1().ConfigMaps(w.namespaceName).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: stopConfigMapName},
	}, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		errs = append(errs, err)
	} else {
		// pollers check for the configmap every 5s and flush open episodes when they stop.
		select {
		case <-time.After(15 * time.Second):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}

	pollerIntervals, pollerJunits, pollerErrs := disruptionlibrary.CollectIntervalsForPods(ctx, w.kubeClient, "sig-network", w.namespaceName,
		labels.SelectorFromSet(labels.Set{pollerLabelKey: "poller"}))
	intervals = append(intervals, pollerIntervals...)
	junits = append(junits, pollerJunits...)
	errs = append(errs, pollerErrs...)

	ovnIntervals, logJunit := w.collectOVNKubeControllerEvents(ctx, beginning)
	intervals = append(intervals, ovnIntervals...)
	junits = append(junits, logJunit)

	return intervals, junits, errors.Join(errs...)
}

func (w *hostConnectionIntegrity) collectOVNKubeControllerEvents(ctx context.Context, beginning time.Time) (monitorapi.Intervals, *junitapi.JUnitTestCase) {
	junit := &junitapi.JUnitTestCase{Name: LogCollectionTestName}

	pods, err := w.kubeClient.CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{LabelSelector: ovnkubeNodeSelector})
	if err != nil {
		junit.FailureOutput = &junitapi.FailureOutput{Output: fmt.Sprintf("unable to list ovnkube-node pods: %v", err)}
		return nil, junit
	}

	handler := newOVNKubeControllerLogHandler(beginning)
	failures := []string{}
	read := 0
	for _, pod := range pods.Items {
		streamer := podaccess.NewOneTimePodStreamer(w.kubeClient, pod.Namespace, pod.Name, ovnkubeControllerName, handler)
		if err := streamer.ReadLog(ctx); err != nil {
			failures = append(failures, fmt.Sprintf("pods/%s -c %s: %v", pod.Name, ovnkubeControllerName, err))
			continue
		}
		read++
	}
	junit.SystemOut = fmt.Sprintf("read %s logs from %d of %d ovnkube-node pods\n%s", ovnkubeControllerName, read, len(pods.Items), strings.Join(failures, "\n"))
	if read == 0 {
		junit.FailureOutput = &junitapi.FailureOutput{
			Output: fmt.Sprintf("unable to read %s logs from any of %d ovnkube-node pods:\n%s", ovnkubeControllerName, len(pods.Items), strings.Join(failures, "\n")),
		}
	}
	return handler.Intervals(), junit
}

func (w *hostConnectionIntegrity) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	w.findings = Correlate(startingIntervals)
	ret := monitorapi.Intervals{}
	for _, f := range w.findings {
		ret = append(ret, FindingToInterval(f))
	}
	return ret, nil
}

func (w *hostConnectionIntegrity) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	return Evaluate(finalIntervals), nil
}

// Evaluate returns the junit for the given final intervals. Only runs that tore down at least one UDN are
// evaluated; others pass with a note.
func Evaluate(finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	junit := &junitapi.JUnitTestCase{Name: TestName}
	if !UDNTeardownObserved(finalIntervals) {
		junit.SystemOut = "no UDN teardown observed in ovnkube-controller logs; not evaluated"
		return []*junitapi.JUnitTestCase{junit}
	}

	lines := []string{}
	for _, i := range finalIntervals {
		if i.Source == SourceBlackholedAfterGatewayReconcile {
			lines = append(lines, i.Message.HumanMessage)
		}
	}
	if len(lines) == 0 {
		return []*junitapi.JUnitTestCase{junit}
	}
	sort.Strings(lines)
	junit.FailureOutput = &junitapi.FailureOutput{
		Output: fmt.Sprintf("%d established host-network connection(s) were reset or black-holed right after an ovnkube-node gateway reconcile on the same node (OCPBUGS-128289):\n%s",
			len(lines), strings.Join(lines, "\n")),
	}
	junit.SystemOut = junit.FailureOutput.Output
	return []*junitapi.JUnitTestCase{junit}
}

// Stats is written to the storage directory for offline analysis.
type Stats struct {
	UDNTeardownObserved bool           `json:"udnTeardownObserved"`
	GatewayReconciles   int            `json:"gatewayReconciles"`
	Episodes            map[string]int `json:"episodes"`
	Findings            []Finding      `json:"findings"`
}

func (w *hostConnectionIntegrity) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}
	stats := Stats{UDNTeardownObserved: UDNTeardownObserved(finalIntervals), Episodes: map[string]int{}, Findings: w.findings}
	for _, i := range finalIntervals {
		switch i.Source {
		case SourceGatewayReconcile:
			stats.GatewayReconciles++
		case poller.IntervalSource:
			stats.Episodes[fmt.Sprintf("%s/%s", i.Locator.Keys[poller.LocatorBackendKey], i.Message.Reason)]++
		}
	}
	b, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(storageDir, fmt.Sprintf("host-connection-integrity%s.json", timeSuffix)), b, 0644)
}

func (w *hostConnectionIntegrity) Cleanup(ctx context.Context) error {
	if len(w.namespaceName) == 0 || w.kubeClient == nil {
		return nil
	}
	if err := w.kubeClient.CoreV1().Namespaces().Delete(ctx, w.namespaceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := w.kubeClient.CoreV1().Namespaces().Get(ctx, w.namespaceName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}
