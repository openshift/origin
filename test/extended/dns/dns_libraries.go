package dns

import (
	"bufio"
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"regexp"
	"strconv"
	"strings"

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
	g.By("creating the pod and waiting for success status")
	podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
	updated, err := podClient.Create(context.Background(), pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	w, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Watch(context.Background(), metav1.SingleObject(
		metav1.ObjectMeta{
			Name:            pod.Name,
			ResourceVersion: updated.ResourceVersion,
		},
	))
	o.Expect(err).NotTo(o.HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), e2e.PodStartTimeout)
	defer cancel()
	if _, err = watchtools.UntilWithoutRetry(ctx, w, PodSucceeded); err != nil {
		e2e.Failf("Failed: %v", err)
	}

	g.By("retrieving the pod logs")
	r, err := podClient.GetLogs(pod.Name, &kapiv1.PodLogOptions{Container: "querier"}).Stream(context.Background())
	o.Expect(err).NotTo(o.HaveOccurred())

	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "fail") {
			e2e.Failf("DNS resolution failed: %s", line)
		}
	}
}

var _ = g.Describe("[sig-network-edge] DNS lookup", func() {
	f := e2e.NewDefaultFramework("dns-libraries")
	oc := exutil.NewCLI("dns-libraries")

	// Create pods using the image built from the Dockerfile located in testdata/dns/go-dns-resolver.
	// These pods perform a DNS query to verify different versions of Go's DNS resolver function correctly
	// with OpenShift DNS.
	g.It("using Go's DNS resolver", func() {
		configClient, err := configclient.NewForConfig(f.ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType {
			g.Skip("Skip DNS lookup using Go's DNS resolver on BareMetal as there is no external connection to pull the base image.")
		}

		dnsService, err := f.ClientSet.CoreV1().Services("openshift-dns").Get(context.Background(), "dns-default", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Failed to get default DNS service: %v", err)
		}

		// Go Toolset UBI 9 Image: https://catalog.redhat.com/software/containers/ubi9/go-toolset/61e5c00b4ec9945c18787690
		goToolsetImage := "registry.redhat.io/ubi9/go-toolset"
		oldestGoMinorVersionToTest := 17

		// Create a pod with the latest Go toolset image to identify the Go version it is running.
		// This allows us to test older Go versions up to and including this latest version.
		g.By(fmt.Sprintf("identifying the Go version running in %s:latest", goToolsetImage))
		podExec, err := exutil.NewPodExecutor(oc, "go-latest-version-finder", goToolsetImage+":latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		goLatestVersion, err := podExec.Exec("echo -n ${GO_MAJOR_VERSION}.${GO_MINOR_VERSION}")
		o.Expect(err).NotTo(o.HaveOccurred())
		re := regexp.MustCompile(`^1\.(\d+)$`)
		if !re.MatchString(goLatestVersion) {
			e2e.Failf("unexpected output of go-latest-version-finder pod: %q", goLatestVersion)
		}
		e2e.Logf("%s:latest is running Go version: %s", goToolsetImage, goLatestVersion)
		matches := re.FindStringSubmatch(goLatestVersion)
		o.Expect(len(matches)).To(o.Equal(2))
		latestGoMinorVersion, err := strconv.Atoi(matches[1])
		o.Expect(err).NotTo(o.HaveOccurred())

		// Start at the oldest GO version and iterate to the latest.
		curGoMinorVersion := oldestGoMinorVersionToTest
		for curGoMinorVersion <= latestGoMinorVersion {
			goVersion := fmt.Sprintf("1.%d", curGoMinorVersion)
			configName := "dns-libraries-go-" + strings.Replace(goVersion, ".", "-", -1)
			g.By(fmt.Sprintf("creating build and deployment for %s testing Go Version %s", configName, goVersion))

			// Build an image from testdata/dns/go-dns-resolver/Dockerfile while specifying
			// a base image with a specific Go version to test.
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
								BuildArgs: []kapiv1.EnvVar{
									{
										Name:  "GO_VERSION",
										Value: goVersion,
									},
									{
										Name:  "GO_TOOLSET_IMAGE",
										Value: goToolsetImage,
									},
								},
							},
							Type: buildv1.DockerBuildStrategyType,
						},
					},
				},
			}
			imageStream := v1.ImageStream{
				ObjectMeta: commonObjectMeta,
			}

			bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Create(context.Background(), &buildConfig, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(bc.Name).To(o.Equal(configName))
			e2e.Logf("created BuildConfig, creationTimestamp: %v", bc.CreationTimestamp)

			is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Create(context.Background(), &imageStream, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(is.Name).To(o.Equal(configName))
			e2e.Logf("created ImageStream, creationTimestamp: %v", is.CreationTimestamp)

			br, err := exutil.StartBuildAndWait(oc, configName, fmt.Sprintf("--from-dir=%s", exutil.FixturePath("testdata", "dns", "go-dns-resolver")))
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()
			o.Expect(br.Build.Status.OutputDockerImageReference).NotTo(o.BeEmpty())

			// Create a pod from the new image provided from the build.
			podExec, err := exutil.NewPodExecutor(oc, configName, br.Build.Status.OutputDockerImageReference)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to have the right go version")
			out, err := podExec.Exec("echo -n ${GO_MAJOR_VERSION}.${GO_MINOR_VERSION}")
			o.Expect(err).NotTo(o.HaveOccurred())
			if goVersion != out {
				e2e.Failf("expected GO version to be %s got %s", goVersion, out)
			}

			g.By("expect the go application to successfully resolve DNS")
			out, err = podExec.Exec(fmt.Sprintf("go run /go/dns_libraries.go -cluster-ip=%q", dnsService.Spec.ClusterIP))
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(out, "Successfully") {
				e2e.Failf("DNS resolution failed: %s", out)
			}
			curGoMinorVersion++
		}
	})

	// Create a simple Pod that is using getent to perform DNS queries
	// to make sure glibc's DNS resolver works fine with OpenShift DNS.
	g.It("using glibc's DNS resolver", func() {
		// Using www.redhat.com just as a sample host for dns queries
		const host = "www.redhat.com"

		// Running getent command 10 times to see if it can resolve the dns steadily
		cmd := repeatCommand(
			10,
			fmt.Sprintf("getent -s dns ahosts %s || echo $(date) fail", host),
		)
		pod := createDNSPod(f.Namespace.Name, cmd)
		checkForPodLogFailures(f, pod)
	})
})
