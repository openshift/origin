package dns

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v12 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/image/v1"
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

		// 1.16 provides 404 error, though https://catalog.redhat.com/software/containers/rhel8/go-toolset/5b9c810add19c70b45cbd666?tag=1.16.12-10&push_date=1650995486000
		// clearly states 1.16 is a tag
		goVersions := []string{"1.16.12", "1.17", "1.18", "latest"}
		for _, version := range goVersions {
			configName := "dns-libraries-go-" + strings.Replace(version, ".", "-", -1)
			By(fmt.Sprintf("creating build and deployment config for %s", configName))

			imageReference := kapiv1.ObjectReference{
				Name: configName + ":latest",
				Kind: "ImageStreamTag",
			}
			commonObjectMeta := metav1.ObjectMeta{
				Name: configName,
				Labels: map[string]string{
					"build": configName,
				},
			}
			buildConfig := buildv1.BuildConfig{
				ObjectMeta: commonObjectMeta,
				Spec: buildv1.BuildConfigSpec{
					CommonSpec: buildv1.CommonSpec{
						Output: buildv1.BuildOutput{
							To: &imageReference,
						},
						Source: buildv1.BuildSource{
							Binary: nil,
							Type:   buildv1.BuildSourceBinary,
						},
						Strategy: buildv1.BuildStrategy{
							DockerStrategy: &buildv1.DockerBuildStrategy{
								BuildArgs: []kapiv1.EnvVar{{
									Name:  "GO_VERSION",
									Value: version,
								}},
							},
							Type: buildv1.DockerBuildStrategyType,
						},
					},
				},
			}
			imageStream := v1.ImageStream{
				ObjectMeta: commonObjectMeta,
			}
			deploymentConfigLabelSelectors := map[string]string{
				"app":              configName,
				"deploymentconfig": configName,
			}
			deploymentConfig := v12.DeploymentConfig{
				ObjectMeta: commonObjectMeta,
				Spec: v12.DeploymentConfigSpec{
					Replicas: 1,
					Selector: deploymentConfigLabelSelectors,
					Template: &kapiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: deploymentConfigLabelSelectors,
						},
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{{
								Image:                    configName,
								ImagePullPolicy:          kapiv1.PullAlways,
								Name:                     configName,
								TerminationMessagePolicy: kapiv1.TerminationMessageFallbackToLogsOnError,
							}},
						},
					},
					Triggers: v12.DeploymentTriggerPolicies{{
						Type: v12.DeploymentTriggerOnImageChange,
						ImageChangeParams: &v12.DeploymentTriggerImageChangeParams{
							Automatic:      true,
							ContainerNames: []string{configName},
							From:           imageReference,
						},
					}},
				},
			}

			bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Create(context.TODO(), &buildConfig, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(bc.Name).To(o.Equal(configName))
			e2e.Logf("created BuildConfig, creationTimestamp: %v", bc.CreationTimestamp)

			is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Create(context.TODO(), &imageStream, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(is.Name).To(o.Equal(configName))
			e2e.Logf("created ImageStream, creationTimestamp: %v", is.CreationTimestamp)

			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Create(context.TODO(), &deploymentConfig, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(configName))
			e2e.Logf("created DeploymentConfig, creationTimestamp: %v", dc.CreationTimestamp)

			_, err = exutil.StartBuildAndWait(oc, configName, fmt.Sprintf("--from-dir=%s", exutil.FixturePath("testdata", "dns", "go-library-test")))
			if err != nil {
				e2e.Failf("Failed to build the go library image: %v", err)
			}

			labels := exutil.ParseLabelsOrDie(fmt.Sprintf("app=%s", configName))

			By(fmt.Sprintf("expect the builds to complete successfully and deploy a %s pod", configName))
			pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), labels, exutil.CheckPodIsRunning, 1, 5*time.Minute)
			if err != nil {
				e2e.Failf("Failed to start the dns-libraries-go pod: %v", err)
			}
			if len(pods) != 1 {
				e2e.Failf("Got %d pods with labels %v, expected 1", len(pods), labels)
			}

			By("expect the go application to successfully resolve DNS")
			stdout, err := e2e.RunHostCmd(oc.Namespace(), pods[0], fmt.Sprintf("go run /go/dns_libraries.go -cluster-ip=%q", dnsService.Spec.ClusterIP))
			if err != nil {
				e2e.Failf("Failed to exec dns-libraries-go pod: %v", err)
			}
			if !strings.Contains(stdout, "Successfully resolved") {
				e2e.Failf("DNS resolution failed: %s", stdout)
			}
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
		pod := createDNSPod(f.Namespace.Name, cmd)
		checkForPodLogFailures(f, pod)
	})
})
