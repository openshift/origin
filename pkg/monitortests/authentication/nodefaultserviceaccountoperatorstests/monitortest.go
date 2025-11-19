package nodefaultserviceaccountoperatortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type noDefaultServiceAccountChecker struct {
	kubeClient kubernetes.Interface
	cfgClient  *configv1.ConfigV1Client
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &noDefaultServiceAccountChecker{}
}

// Cleanup implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) Cleanup(ctx context.Context) error {
	return nil
}

func exceptionWithJira(prefix, jiraURL string) func(corev1.Pod) (string, bool) {
	return func(pod corev1.Pod) (string, bool) {
		podNameNSCombo := pod.Namespace + "/" + pod.Name
		if strings.HasPrefix(podNameNSCombo, prefix) {
			return jiraURL, true
		}
		return "", false
	}
}

var exceptions = []func(pod corev1.Pod) (string, bool){
	exceptionWithJira("openshift-cluster-version/cluster-version-operator-", "https://issues.redhat.com/browse/OCPBUGS-65621"),
	exceptionWithJira("openshift-console/downloads-", "https://issues.redhat.com/browse/OCPBUGS-65622"),
	exceptionWithJira("openshift-etcd/etcd-guard-", "https://issues.redhat.com/browse/OCPBUGS-65626"),
	exceptionWithJira("openshift-ingress-canary/ingress-canary-", "https://issues.redhat.com/browse/OCPBUGS-65629"),
	exceptionWithJira("openshift-kube-apiserver/kube-apiserver-guard-", "https://issues.redhat.com/browse/OCPBUGS-65626"),
	exceptionWithJira("openshift-kube-controller-manager/kube-controller-manager-guard-", "https://issues.redhat.com/browse/OCPBUGS-65626"),
	exceptionWithJira("openshift-kube-scheduler/openshift-kube-scheduler-guard-", "https://issues.redhat.com/browse/OCPBUGS-65626"),
	exceptionWithJira("openshift-monitoring/monitoring-plugin-", "https://issues.redhat.com/browse/OCPBUGS-65630"),
	exceptionWithJira("openshift-multus/multus-", "https://issues.redhat.com/browse/OCPBUGS-65631"),
	exceptionWithJira("openshift-network-console/networking-console-plugin-", "https://issues.redhat.com/browse/OCPBUGS-65633"),
	exceptionWithJira("openshift-network-diagnostics/network-check-target-", "https://issues.redhat.com/browse/OCPBUGS-65633"),
	exceptionWithJira("default/verify-all-openshiftcommunityoperators-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("default/verify-all-openshiftredhatmarketplace-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("default/verify-all-openshiftcertifiedoperators-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("default/verify-all-openshiftredhatoperators-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("openshift-cluster-version/version-", "https://issues.redhat.com/browse/OCPBUGS-65621"),
	exceptionWithJira("openshift-must-gather/must-gather-", "https://issues.redhat.com/browse/OCPBUGS-65635"), // keep as default service account required for this component.
	exceptionWithJira("kube-system/konnectivity-agent-", "https://issues.redhat.com/browse/OCPBUGS-65636"),
	exceptionWithJira("openshift-multus/multus-additional-cni-plugins-", "https://issues.redhat.com/browse/OCPBUGS-65631"),
	exceptionWithJira("openshift-multus/cni-sysctl-allowlist-ds-", "https://issues.redhat.com/browse/OCPBUGS-65631"),
	exceptionWithJira("default/verify-metas-openshiftcertifiedoperators-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("default/verify-metas-openshiftcommunityoperators-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("default/verify-metas-openshiftredhatmarketplace-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("default/verify-metas-openshiftredhatoperators-", "https://issues.redhat.com/browse/OCPBUGS-65634"),
	exceptionWithJira("openshift-cluster-api/capv-controller-manager-", "https://issues.redhat.com/browse/OCPBUGS-65637"),

	// Handle the one outlier (Namespace only check) manually
	func(pod corev1.Pod) (string, bool) {
		if pod.Namespace == "openshift-marketplace" {
			return "NO-JIRA", true
		}
		return "", false
	},
}

// generateDefaultSAFailures generates a list of failures where the pod in a list of pods
// violated the default service account check.
func generateDefaultSAFailures(podList []corev1.Pod) []*junitapi.JUnitTestCase {
	junits := []*junitapi.JUnitTestCase{}
	for _, pod := range podList {
		podSA := pod.Spec.ServiceAccountName
		testName := fmt.Sprintf("[sig-auth] all pods in %s namespace must not use the default service account.", pod.Namespace)

		// if the service account name is not default, we can exit for that iteration
		if podSA != "default" {
			// test passes.
			junits = append(junits, &junitapi.JUnitTestCase{Name: testName})
			continue
		}

		exceptionMsg := ""
		for _, exception := range exceptions {
			if msg, ok := exception(pod); ok {
				exceptionMsg = msg
				break
			}
		}

		failure := fmt.Sprintf("pod %q is using the default service account", pod.Name)
		if exceptionMsg != "" {
			testName = "[EXCEPTIONS] " + testName // ensure exceptions are respected when needed, but separate list for when it is not.
			failure += fmt.Sprintf(" (exception: %s)", exceptionMsg)
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:          testName,
			SystemOut:     failure,
			FailureOutput: &junitapi.FailureOutput{Output: failure},
		})

		if exceptionMsg != "" {
			// introduce flake
			junits = append(junits, &junitapi.JUnitTestCase{
				Name: testName,
			})
		}
	}
	return junits
}

// CollectData implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if n.cfgClient == nil || n.kubeClient == nil {
		return nil, nil, nil
	}

	namespaces, err := n.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		if !strings.HasPrefix(ns.Name, "openshift") && !strings.HasPrefix(ns.Name, "kube-") && ns.Name != "default" {
			continue
		}
		// get list of all pods in the namespace
		pods, err := n.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, nil, err
		}
		// use helper method to generate default service account failures
		junits = append(junits, generateDefaultSAFailures(pods.Items)...)
	}
	return nil, junits, nil
}

// ConstructComputedIntervals implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning time.Time, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

// PrepareCollection implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// StartCollection implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	n.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	n.cfgClient, err = configv1.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

// WriteContentToStorage implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) WriteContentToStorage(ctx context.Context, storageDir string, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}
