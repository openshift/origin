package rorfs

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:ReadOnlyRootFilesystem]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("rorfs", admissionapi.LevelPrivileged)

	g.It("Explicitly set readOnlyRootFilesystem to true [Timeout:30m][Skipped:Disconnected]", func() {
		// Skipped case on proxy cluster or disconnected env
		g.By("Check if it's a proxy cluster")
		httpProxy, _, _ := getGlobalProxy(oc)
		if strings.Contains(httpProxy, "http") {
			g.Skip("Skip for proxy platform or disconnected env")
		}

		var (
			randmStr = uuid.New()
			testNs1  = "test-rofs-cm-" + randmStr
			testNs2  = "test-rofs-rm-" + randmStr
		)
		namespaces := []string{
			"openshift-controller-manager",
			"openshift-controller-manager-operator",
			"openshift-route-controller-manager",
			//"hypershift",
			"openshift-cluster-version",
			"openshift-image-registry",
			"openshift-insights",
			//"openshift-dns",
			"openshift-etcd",
			"openshift-etcd-operator",
			"openshift-kube-controller-manager",
			"openshift-kube-controller-manager-operator",
			"openshift-kube-scheduler-operator",
			"openshift-kube-scheduler",
			//"openshift-kube-apiserver-operator",
			//"openshift-kube-apiserver",
			"openshift-cloud-credential-operator",
			"openshift-marketplace",
			//"openshift-console-operator",
			//"openshift-console",
		}

		oc.SetupProject()
		token, err := oc.Run("whoami").Args("-t").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, ns := range namespaces {
			podNames := getPodsListByLabel(oc.AsAdmin(), ns, "")
			for _, podName := range podNames {
				// Skip unwanted pods
				if !strings.Contains(podName, "installer") &&
					!strings.Contains(podName, "guard") &&
					!strings.Contains(podName, "revision") &&
					!strings.Contains(podName, "pruner") {

					if ns != "openshift-operator-lifecycle-manager" {
						assertPodToBeReady(oc, podName, ns)

						framework.Logf("Inspect the %s Pod's securityContext.", ns)
						securityContext := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", podName, "-n", ns, "-o", "jsonpath={.spec.containers[0].securityContext}")
						o.Expect(securityContext).To(o.ContainSubstring("readOnlyRootFilesystem"),
							"pod %s in namespace %s does not have readOnlyRootFilesystem", podName, ns)
					}

					framework.Logf("Negative Test : Attempting to Write to Root Filesystem on %s pod %s", ns, podName)

					// Test file creation in various restricted paths - all should be read-only
					testPaths := []string{
						"/usr/local/bin/testfile",
						"/etc/testfile",
						"/usr/bin/testfile",
					}

					for _, testPath := range testPaths {
						out, _ := oc.AsAdmin().WithoutNamespace().Run("exec").Args(podName, "-n", ns, "--", "touch", testPath).Output()
						readOnlyMsg := fmt.Sprintf("cannot touch '%s': Read-only file system", testPath)
						permissionMsg := fmt.Sprintf("cannot touch '%s': Permission denied", testPath)

						hasReadOnlyError := strings.Contains(out, readOnlyMsg)
						hasPermissionError := strings.Contains(out, permissionMsg)

						o.Expect(hasReadOnlyError || hasPermissionError).To(o.BeTrue(),
							"pod %s in namespace %s should not allow writing to %s (expect Read-only file system or Permission denied). Got output: %s", podName, ns, testPath, out)
					}
				}
			}
		}

		g.By("openshift-controller-manager: create Deployment, scale, verify")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", testNs1, "--ignore-not-found").Output()
		_, err = oc.AsAdmin().WithoutNamespace().Run("new-project").Args(testNs1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("create").Args("deployment", "nginx", "--image=quay.io/openshifttest/nginx-alpine@sha256:f78c5a93df8690a5a937a6803ef4554f5b6b1ef7af4f19a441383b8976304b4c", "-n", testNs1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("deployment/nginx", "--replicas=3", "-n", testNs1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for pods to be Ready")
		podName := getPodsList(oc.AsAdmin(), testNs1)
		assertPodToBeReady(oc, podName[0], testNs1)

		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", testNs1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("openshift-route-controller-manager: create Route, check")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", testNs2, "--ignore-not-found").Output()
		_, err = oc.AsAdmin().WithoutNamespace().Run("new-project").Args(testNs2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("new-app").Args("--name=my-app", "nginx-example", "--image=quay.io/openshifttest/nginx-alpine@sha256:f78c5a93df8690a5a937a6803ef4554f5b6b1ef7af4f19a441383b8976304b4c", "-n", testNs2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AsAdmin().WithoutNamespace().Run("expose").Args("service", "my-app", "-n", testNs2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for Route & test")
		routeHost := ""
		o.Eventually(func() string {
			routeHost, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("route", "my-app", "-o", "jsonpath={.spec.host}", "-n", testNs2).Output()
			return routeHost
		}, 2*time.Minute, 10*time.Second).ShouldNot(o.BeEmpty(), "Route was not created")

		g.By("Attempt curl to the route")
		url := fmt.Sprintf("http://%s", routeHost)
		output := clientCurl(token, url)
		o.Expect(output).To(o.ContainSubstring("Hello-OpenShift"), "Nginx welcome page not reachable at %s", url)
	})
})

// Helper functions that need to be implemented or imported from existing test utilities
func getGlobalProxy(oc *exutil.CLI) (string, string, string) {
	httpProxy, err := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	httpsProxy, err := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpsProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	noProxy, err := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.noProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	return httpProxy, httpsProxy, noProxy
}

// Get the pods List by label
func getPodsListByLabel(oc *exutil.CLI, namespace string, selectorLabel string) []string {
	podsOp := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", "-n", namespace, "-l", selectorLabel, "-o=jsonpath={.items[*].metadata.name}")
	o.Expect(podsOp).NotTo(o.BeEmpty())
	return strings.Split(podsOp, " ")
}

// Get something existing resource
func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	return doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
}

// Get something resource to be ready
func getResourceToBeReady(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	errPoll := wait.PollUntilContextTimeout(context.Background(), 6*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil || len(result) == 0 {
			framework.Logf("Unable to retrieve the expected resource, retrying...")
			return false, nil
		}
		return true, nil
	})
	o.Expect(errPoll).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to retrieve %v", parameters))
	framework.Logf("The resource returned:\n%v", result)
	return result
}

// the method is to do something with oc.
func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}

func clientCurl(tokenValue string, url string) string {
	timeoutDuration := 3 * time.Second
	var bodyString string

	proxyURL := getProxyURL()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		framework.Failf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokenValue)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeoutDuration,
	}

	errCurl := wait.PollImmediate(10*time.Second, 300*time.Second, func() (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			bodyString = string(bodyBytes)
			return true, nil
		}
		return false, nil
	})
	o.Expect(errCurl).NotTo(o.HaveOccurred(), fmt.Sprintf("error waiting for curl request output: %v", errCurl))
	return bodyString
}

const (
	asAdmin          = true
	withoutNamespace = true
)

func getProxyURL() *url.URL {
	// Prefer https_proxy, fallback to http_proxy
	proxyURLString := os.Getenv("https_proxy")
	if proxyURLString == "" {
		proxyURLString = os.Getenv("http_proxy")
	}
	if proxyURLString == "" {
		return nil
	}
	proxyURL, err := url.Parse(proxyURLString)
	if err != nil {
		framework.Failf("error parsing proxy URL: %v", err)
	}
	return proxyURL
}

// assertPodToBeReady poll pod status to determine it is ready
func assertPodToBeReady(oc *exutil.CLI, podName string, namespace string) {
	err := wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
		stdout, err := oc.AsAdmin().Run("get").Args("pod", podName, "-n", namespace, "-o", "jsonpath='{.status.conditions[?(@.type==\"Ready\")].status}'").Output()
		if err != nil {
			framework.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, "True") {
			framework.Logf("Pod %s is ready!", podName)
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Pod %s status is not ready!", podName))
}

// Get the pods List by label
func getPodsList(oc *exutil.CLI, namespace string) []string {
	podsOp := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}")
	podNames := strings.Split(strings.TrimSpace(podsOp), " ")
	framework.Logf("Namespace %s pods are: %s", namespace, string(podsOp))
	return podNames
}
