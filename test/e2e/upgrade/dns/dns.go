package dns

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	kappsv1 "k8s.io/api/apps/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edaemonset "k8s.io/kubernetes/test/e2e/framework/daemonset"
	"k8s.io/kubernetes/test/e2e/upgrades"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

type UpgradeTest struct{}

var appName string

func (t *UpgradeTest) Name() string { return "check-for-dns-availability" }
func (UpgradeTest) DisplayName() string {
	return "[sig-network-edge] Verify DNS availability during and after upgrade success"
}

// Setup creates a DaemonSet to verify DNS availability during and after upgrade
func (t *UpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	ginkgo.By("Setting up upgrade DNS availability test")

	ginkgo.By("Getting DNS Service Cluster IP")
	dnsServiceIP := t.getServiceIP(f)

	ginkgo.By("Creating a DaemonSet to verify DNS availability")
	appName = fmt.Sprintf("dns-test-%s", string(uuid.NewUUID()))
	ds := t.createDNSTestDaemonSet(f, dnsServiceIP)
	framework.ExpectNotEqual(ds, nil, "DaemonSet should not be nil")

	ginkgo.By("Waiting for DaemonSet pods to become ready")
	err := wait.Poll(framework.Poll, framework.PodStartTimeout, func() (bool, error) {
		return e2edaemonset.CheckRunningOnAllNodes(ctx, f, ds)
	})
	framework.ExpectNoError(err)

}

// Test checks for logs from DNS availability test a minute after upgrade finishes
// to cover during and after upgrade phase, and verifies the results.
func (t *UpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, _ upgrades.UpgradeType) {
	ginkgo.By("Validating DNS results during upgrade")
	t.validateDNSResults(f)

	// Block until upgrade is done
	<-done

	ginkgo.By("Sleeping for a minute to give it time for verifying DNS after upgrade")
	time.Sleep(1 * time.Minute)

	ginkgo.By("Validating DNS results after upgrade")
	t.validateDNSResults(f)
}

// getServiceIP gets Cluster IP from DNS Service
func (t *UpgradeTest) getServiceIP(f *framework.Framework) string {
	dnsService, err := f.ClientSet.CoreV1().Services("openshift-dns").Get(context.Background(), "dns-default", metav1.GetOptions{})
	framework.ExpectNoError(err)
	return dnsService.Spec.ClusterIP
}

// createDNSTestDaemonSet creates a DaemonSet to test DNS availability
func (t *UpgradeTest) createDNSTestDaemonSet(f *framework.Framework, dnsServiceIP string) *kappsv1.DaemonSet {
	cmd := fmt.Sprintf("while true; do dig +short @%s google.com || echo $(date) fail && sleep 1; done", dnsServiceIP)
	ds, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.Background(), &kappsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec: kappsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			Template: kapiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": appName},
				},
				Spec: kapiv1.PodSpec{
					Containers: []kapiv1.Container{
						{
							Name:    "querier",
							Image:   imageutils.GetE2EImage(imageutils.JessieDnsutils),
							Command: []string{"sh", "-c", cmd},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return ds
}

// validateDNSResults retrieves the Pod logs and validates the results
func (t *UpgradeTest) validateDNSResults(f *framework.Framework) {
	ginkgo.By(fmt.Sprintf("Listing Pods with label app=%s", appName))
	podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
	selector, _ := labels.Parse(fmt.Sprintf("app=%s", appName))
	pods, err := podClient.List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	framework.ExpectNoError(err)

	waitingPods := sets.String{}
	ginkgo.By("Retrieving logs from all the Pods belonging to the DaemonSet and asserting no failure")
	for _, pod := range pods.Items {
		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Waiting != nil {
			// this container is waiting, so there will be no logs even if we try again later, we won't have logs of interest
			waitingPods.Insert(fmt.Sprintf("ns/%v pod/%v", pod.Namespace, pod.Name))
			continue
		}
		r, err := podClient.GetLogs(pod.Name, &kapiv1.PodLogOptions{Container: "querier"}).Stream(context.Background())
		if err != nil && strings.Contains(err.Error(), "waiting to start") {
			// this container is waiting, so there will be no logs even if we try again later, we won't have logs of interest
			// the best theory we have for this situation is that the list happened before a pod was restarted and so we don't
			// have logs for it.  this happens (currently) pretty infrequently.
			waitingPods.Insert(fmt.Sprintf("ns/%v pod/%v", pod.Namespace, pod.Name))
			continue
		}
		framework.ExpectNoError(err)

		failureCount := 0.0
		successCount := 0.0
		scan := bufio.NewScanner(r)
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, "fail") {
				failureCount++
			} else if ip := net.ParseIP(line); ip != nil {
				successCount++
			}
		}

		if successRate := (successCount / (successCount + failureCount)) * 100; successRate < 99 {
			err = fmt.Errorf("success rate is less than 99%% on the node %s: [%0.2f]", pod.Spec.NodeName, successRate)
		} else {
			err = nil
		}
		framework.ExpectNoError(err)
	}

	if len(waitingPods) > 2 {
		framework.ExpectNoError(fmt.Errorf("too many pods were waiting: %v", strings.Join(waitingPods.List(), ",")))
	}
}

// Teardown cleans up any objects that are created that
// aren't already cleaned up by the framework.
func (t *UpgradeTest) Teardown(_ context.Context, _ *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
