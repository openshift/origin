package dns

import (
	"bufio"
	"context"
	"fmt"
	o "github.com/onsi/gomega"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// checkForPodLogFailures goes through the pod logs and determines if there was a failure
// by looking for "fail" keyword in the logs.
func checkForPodLogFailures(f *e2e.Framework, pod *kapiv1.Pod) {
	By("submitting the pod to kubernetes")
	podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
	updated, err := podClient.Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		e2e.Failf("Failed to create %s pod: %v", pod.Name, err)
	}

	w, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Watch(context.Background(), metav1.SingleObject(metav1.ObjectMeta{Name: pod.Name, ResourceVersion: updated.ResourceVersion}))
	if err != nil {
		e2e.Failf("Failed to watch pods: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), e2e.PodStartTimeout)
	defer cancel()
	if _, err = watchtools.UntilWithoutRetry(ctx, w, PodSucceeded); err != nil {
		e2e.Failf("Failed: %v", err)
	}

	By("retrieving the pod logs")
	r, err := podClient.GetLogs(pod.Name, &kapiv1.PodLogOptions{Container: "querier"}).Stream(context.Background())
	if err != nil {
		e2e.Failf("Failed to get pod logs %s: %v", pod.Name, err)
	}

	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "fail") {
			e2e.Failf("DNS resolution failed: %s", line)
		}
	}
}

var _ = Describe("[sig-network-edge] DNS lookup", func() {
	f := e2e.NewDefaultFramework("dns-libraries")
	oc := exutil.NewCLI("dns-libraries")
	buildFixture := exutil.FixturePath("testdata", "dns", "dns_libraries_go.yaml")
	dnsFixture := exutil.FixturePath("testdata", "dns")
	labels := exutil.ParseLabelsOrDie("app=dns-libraries-go")
	nodeSelector := make(map[string]string)

	BeforeEach(func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology != configv1.ExternalTopologyMode {
			nodeSelector["node-role.kubernetes.io/master"] = ""
		}
	})

	// creates a simple Pod that is using the image under /testdata/dns, which performs a DNS query to make sure Go's DNS resolver works fine with OpenShift DNS.
	It("using Go's DNS resolver", func() {
		configClient, err := configclient.NewForConfig(f.ClientConfig())
		if err != nil {
			e2e.Failf("Failed to get config client: %v", err)
		}
		infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Failed to get cluster config: %v", err)
		}
		if infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType {
			Skip("Skip DNS lookup using Go's DNS resolver on BareMetal as there is no external connection to pull the base image.")
		}

		dnsService, err := f.ClientSet.CoreV1().Services("openshift-dns").Get(context.Background(), "dns-default", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Failed to get default DNS service: %v", err)
		}
		By("creating build and deployment config for dns-libraries-go")
		err = oc.Run("create").Args("-f", buildFixture).Execute()
		if err != nil {
			e2e.Failf("Failed to create build configs and deployment config for dns-libraries-go: %v", err)
		}
		By("starting the builder image build with a directory")
		err = oc.Run("start-build").Args("dns-libraries-go", fmt.Sprintf("--from-dir=%s", dnsFixture)).Execute()
		if err != nil {
			e2e.Failf("Failed to start the builder image build: %v", err)
		}
		By("expect the builds to complete successfully and deploy a dns-libraries-go pod")
		pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), labels, exutil.CheckPodIsRunning, 1, 5*time.Minute)
		if err != nil {
			e2e.Failf("Failed to start the dns-libraries-go pod: %v", err)
		}
		if len(pods) != 1 {
			e2e.Failf("Got %d pods with labels %v, expected 1", len(pods), labels)
		}

		By("expect the go application to successfully resolve DNS")
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Failed to get dns-libraries-go pod: %v", err)
		}
		//execute "go run /go/dns_libraries.go -cluster-ip={CLUSTER_IP}" command inside the pod
		args := []string{pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "bash", "-c", fmt.Sprintf("go run /go/dns_libraries.go -cluster-ip=%q", dnsService.Spec.ClusterIP)}
		output, err := oc.Run("exec").Args(args...).Output()
		if err != nil {
			e2e.Failf("Failed to exec dns-libraries-go pod: %v", err)
		}
		if !strings.Contains(output, "Successfully") {
			e2e.Failf("DNS resolution failed: %s", output)
		}
	})

	// creates a simple Pod that is using getent to perform DNS queries, to make sure glibc's DNS resolver works fine with OpenShift DNS.
	It("using glibc's DNS resolver", func() {
		// using www.redhat.com just as a sample host for dns queries
		const host = "www.redhat.com"

		// running getent command 10 times to see if it can resolve the dns steadily
		cmd := repeatCommand(
			10,
			fmt.Sprintf("getent -s dns ahosts %s || echo $(date) fail", host),
		)
		pod := createDNSPod(f.Namespace.Name, cmd, nodeSelector)
		checkForPodLogFailures(f, pod)
	})
})
